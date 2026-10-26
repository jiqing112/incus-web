package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"incus-web/config"
)

// 远程镜像索引缓存。
//
// 设计目标（对应需求）：远程 simplestreams 索引拉取一次需要十几秒，
// 因此把「远程源列表（名称+地址）」与「每个远程源的镜像索引（别名/描述/大小）」
// 落到磁盘，面板重启后仍在；用户第二次打开搜索框时立即命中缓存，
// 同时由后台 goroutine 定期刷新，保证数据不会长期陈旧。
type RemoteImageCache struct {
	mu         sync.RWMutex
	path       string
	remotes    []ImageRemote
	remotesAt  time.Time
	images     map[string]cachedRemoteEntry
	refreshing map[string]bool
}

// cachedRemoteEntry 单个远程源的镜像索引缓存
type cachedRemoteEntry struct {
	Images    []RemoteImage `json:"images"`
	UpdatedAt time.Time     `json:"updated_at"`
	Error     string        `json:"error,omitempty"`
}

// remoteCacheFile 落盘结构
type remoteCacheFile struct {
	Version   int                          `json:"version"`
	Remotes   []ImageRemote                `json:"remotes"`
	RemotesAt time.Time                    `json:"remotes_at"`
	Images    map[string]cachedRemoteEntry `json:"images"`
}

// softTTL 读缓存时的「软过期」阈值：超过则在返回缓存的同时触发一次后台刷新。
const softTTL = 10 * time.Minute

// NewRemoteImageCache 创建缓存实例（此时不读盘，需显式 Load）。
func NewRemoteImageCache() *RemoteImageCache {
	return &RemoteImageCache{
		path:       filepath.Join(config.DataDir, "remote-image-cache.json"),
		images:     map[string]cachedRemoteEntry{},
		refreshing: map[string]bool{},
	}
}

// Load 从磁盘加载缓存，文件不存在或损坏时静默忽略（视为空缓存）。
func (m *RemoteImageCache) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	var f remoteCacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remotes = f.Remotes
	m.remotesAt = f.RemotesAt
	if f.Images != nil {
		m.images = f.Images
	}
	return nil
}

// persistLocked 写盘（调用方需持有写锁）。使用临时文件 + rename 保证原子性。
func (m *RemoteImageCache) persistLocked() {
	f := remoteCacheFile{
		Version:   1,
		Remotes:   m.remotes,
		RemotesAt: m.remotesAt,
		Images:    m.images,
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, m.path)
}

/* ------------------------------ 读 ------------------------------ */

// GetImages 读取某远程源的镜像缓存
func (m *RemoteImageCache) GetImages(name string) (cachedRemoteEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.images[name]
	return e, ok
}

// GetRemotes 读取远程源列表缓存
func (m *RemoteImageCache) GetRemotes() ([]ImageRemote, time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.remotes) == 0 {
		return nil, time.Time{}, false
	}
	out := make([]ImageRemote, len(m.remotes))
	copy(out, m.remotes)
	return out, m.remotesAt, true
}

// IsRefreshing 该远程源是否正在后台刷新
func (m *RemoteImageCache) IsRefreshing(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshing[name]
}

/* ------------------------------ 写 ------------------------------ */

// SetRemotes 更新远程源列表缓存并落盘
func (m *RemoteImageCache) SetRemotes(list []ImageRemote) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remotes = list
	m.remotesAt = time.Now()
	m.persistLocked()
}

// setImages 写入某远程源的镜像缓存并落盘
func (m *RemoteImageCache) setImages(name string, entry cachedRemoteEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.images[name] = entry
	m.persistLocked()
}

// Invalidate 删除某远程源的镜像缓存（远程源被移除/重建时调用）
func (m *RemoteImageCache) Invalidate(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.images, name)
	m.persistLocked()
}

/* ------------------------------ 抓取 ------------------------------ */

// fetchRemotes 通过 incus CLI 读取已配置的远程源
func fetchRemotes() ([]ImageRemote, error) {
	out, code := runCmd(15*time.Second, "incus", "remote", "list", "--format", "json")
	if code != 0 {
		return nil, errString("读取远程源失败: " + strings.TrimSpace(out))
	}
	var raw map[string]struct {
		Addr     string `json:"Addr"`
		Protocol string `json:"Protocol"`
		Static   bool   `json:"Static"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, errString("解析远程源失败: " + err.Error())
	}
	list := []ImageRemote{}
	for name, r := range raw {
		if r.Protocol == "incus" && r.Addr == "unix://" {
			continue // 本机 socket 不是镜像源
		}
		list = append(list, ImageRemote{Name: name, Addr: r.Addr, Protocol: r.Protocol, Static: r.Static})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// fetchRemoteImages 拉取某远程源的完整镜像索引（未过滤，含全部别名与架构变体）
func fetchRemoteImages(name string) ([]RemoteImage, error) {
	out, code := runCmd(60*time.Second, "incus", "image", "list", name+":", "--format", "json")
	if code != 0 {
		return nil, errString("获取远程镜像列表失败: " + strings.TrimSpace(out))
	}
	var raw []struct {
		Fingerprint string `json:"fingerprint"`
		Size        int64  `json:"size"`
		Type        string `json:"type"`
		Properties  struct {
			Description string `json:"description"`
			OS          string `json:"os"`
			Release     string `json:"release"`
		} `json:"properties"`
		Aliases []struct {
			Name string `json:"name"`
		} `json:"aliases"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, errString("解析远程镜像失败: " + err.Error())
	}
	list := make([]RemoteImage, 0, len(raw))
	for _, r := range raw {
		aliases := []string{}
		for _, a := range r.Aliases {
			if a.Name != "" {
				aliases = append(aliases, a.Name)
			}
		}
		alias := ""
		if len(r.Fingerprint) >= 12 {
			alias = r.Fingerprint[:12]
		} else {
			alias = r.Fingerprint
		}
		if len(aliases) > 0 {
			alias = aliases[0]
		}
		list = append(list, RemoteImage{
			Alias:       alias,
			Aliases:     aliases,
			Description: r.Properties.Description,
			OS:          r.Properties.OS,
			Release:     r.Properties.Release,
			Fingerprint: r.Fingerprint,
			Size:        r.Size,
			Type:        r.Type,
		})
	}
	return list, nil
}

/* ------------------------------ 刷新 ------------------------------ */

// Refresh 同步刷新单个远程源的镜像索引
func (m *RemoteImageCache) Refresh(name string) error {
	m.mu.Lock()
	m.refreshing[name] = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.refreshing, name)
		m.mu.Unlock()
	}()

	list, err := fetchRemoteImages(name)
	entry := cachedRemoteEntry{Images: list, UpdatedAt: time.Now()}
	if err != nil {
		// 保留旧数据，仅记录错误，避免一次网络抖动清空可用缓存
		if old, ok := m.GetImages(name); ok {
			entry.Images = old.Images
		}
		entry.Error = err.Error()
	}
	m.setImages(name, entry)
	return err
}

// refreshAsync 后台异步刷新（带去重，避免并发重复拉取）
func (m *RemoteImageCache) refreshAsync(name string) {
	m.mu.Lock()
	if m.refreshing[name] {
		m.mu.Unlock()
		return
	}
	m.refreshing[name] = true
	m.mu.Unlock()
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.refreshing, name)
			m.mu.Unlock()
		}()
		_ = m.Refresh(name)
	}()
}

// RefreshAll 刷新远程源列表与全部远程源的镜像索引
func (m *RemoteImageCache) RefreshAll() {
	if remotes, err := fetchRemotes(); err == nil {
		m.SetRemotes(remotes)
	}
	remotes, _, ok := m.GetRemotes()
	if !ok {
		return
	}
	for _, r := range remotes {
		_ = m.Refresh(r.Name)
	}
}

// StartBackground 启动后台定期刷新，返回停止函数。
func (m *RemoteImageCache) StartBackground(interval time.Duration) func() {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		// 启动后稍等再跑首轮，避开面板启动瞬间的资源竞争
		t := time.NewTimer(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				m.RefreshAll()
				t.Reset(interval)
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }
}

/* ------------------------------ 过滤 ------------------------------ */

// 只保留 x86_64 容器镜像，挑选每个镜像的标准别名（跳过 arm64/cloud 等变体），按别名去重
var remoteVariantSubs = []string{
	"/arm64", "/armhf", "/riscv64", "/loong64", "/i386", "/ppc64le", "/s390x",
	"/cloud", "/tinycloud", "/desktop",
}

func containsVariant(s string) bool {
	for _, sub := range remoteVariantSubs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// FilterRemoteImages 对缓存的原始索引做展示层过滤：架构/变体归一 + 关键字匹配 + 数量上限
func FilterRemoteImages(all []RemoteImage, keyword string) []RemoteImage {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	filtered := []RemoteImage{}
	seen := map[string]bool{}
	for _, img := range all {
		if img.Type != "container" {
			continue
		}
		best := ""
		for _, a := range img.Aliases {
			if containsVariant(a) {
				continue
			}
			if best == "" || len(a) < len(best) {
				best = a
			}
		}
		if best == "" || seen[best] {
			continue
		}
		seen[best] = true
		if keyword != "" {
			hay := strings.ToLower(img.Alias + " " + best + " " + img.Description + " " +
				img.OS + " " + img.Release + " " + strings.Join(img.Aliases, " "))
			if !strings.Contains(hay, keyword) {
				continue
			}
		}
		img.Alias = best
		filtered = append(filtered, img)
		if len(filtered) >= 200 {
			break
		}
	}
	return filtered
}

// errString 简易 error 类型（避免额外依赖）
type errString string

func (e errString) Error() string { return string(e) }
