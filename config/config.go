package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	IncusSocketPath string
	IncusBaseURL    string
	APIToken        string
	// APITokenGenerated 为 true 表示未通过环境变量提供 token，启动时随机生成（需从启动日志获取）
	APITokenGenerated bool
	// BindAddr HTTP 监听地址（默认 0.0.0.0；只想本机访问可设为 127.0.0.1）
	BindAddr string
	// DataDir 面板持久化数据目录（远程镜像索引缓存等）。
	DataDir string
	// RemoteImageRefreshInterval 后台刷新远程镜像索引的间隔（分钟）。
	RemoteImageRefreshInterval int

	// socketMu 保护 IncusSocketPath 的延迟重解析
	socketMu sync.RWMutex
)

// defaultTokenPlaceholder 旧版本内置的公开默认令牌，仅用于启动时给出告警
const defaultTokenPlaceholder = "incus-web-token-2024"

func Init() {
	IncusSocketPath = resolveIncusSocket()
	IncusBaseURL = "http://unix/1.0"

	// 不再使用公开的固定默认令牌：未配置时随机生成，避免「开箱即用」等于「开箱即裸奔」
	if v := os.Getenv("API_TOKEN"); v != "" {
		APIToken = v
	} else {
		APIToken = randomToken(24)
		APITokenGenerated = true
	}

	BindAddr = getEnv("BIND_ADDR", "0.0.0.0")

	DataDir = getEnv("DATA_DIR", defaultDataDir())
	if err := os.MkdirAll(DataDir, 0o755); err != nil {
		// 目录不可写时退化为临时目录，保证面板仍可启动（仅缓存不持久化）
		DataDir = filepath.Join(os.TempDir(), "incus-web")
		_ = os.MkdirAll(DataDir, 0o755)
	}

	RemoteImageRefreshInterval = getEnvInt("REMOTE_IMAGE_REFRESH_MINUTES", 30)

	// 明确记录实际使用的 socket：RHEL 系在 /run/incus，与 Debian 不同，
	// 出问题时第一现场就在启动日志里。
	if os.Getenv("INCUS_SOCKET") != "" {
		log.Printf("Incus socket: %s（来自 INCUS_SOCKET）", IncusSocketPath)
	} else {
		log.Printf("Incus socket: %s（自动探测）", IncusSocketPath)
	}
}

// socketCandidates Incus unix socket 的常见位置（按发行版差异排序）
//   - Debian/Ubuntu/Arch/源码编译：/var/lib/incus/unix.socket
//   - Fedora/RHEL/Rocky/Alma 官方包：/run/incus/unix.socket
var socketCandidates = []string{
	"/var/lib/incus/unix.socket",
	"/run/incus/unix.socket",
}

// resolveIncusSocket 决定要连接的 socket 路径。
// 显式设置 INCUS_SOCKET 时以其为准（即使文件暂不存在，也尊重用户配置）；
// 未设置时按候选列表探测，避免 RHEL 系把 socket 放在 /run/incus 下面板全盘 500。
func resolveIncusSocket() string {
	if v := os.Getenv("INCUS_SOCKET"); v != "" {
		return v
	}
	for _, p := range socketCandidates {
		if st, err := os.Stat(p); err == nil && st.Mode()&os.ModeSocket != 0 {
			return p
		}
	}
	// 兜底：在两个父目录下找任意 *.socket（某些发行版命名不同）
	for _, dir := range []string{"/var/lib/incus", "/run/incus"} {
		if matches, err := filepath.Glob(filepath.Join(dir, "*.socket")); err == nil && len(matches) > 0 {
			return matches[0]
		}
	}
	return socketCandidates[0]
}

// IncusSocket 返回当前可用的 socket 路径。
// 启动时守护进程可能还没起来（面板先于 incus 启动），因此每次连接前做一次轻量探测，
// 路径失效时重新解析，避免面板必须重启才能连上后启动的 incusd。
func IncusSocket() string {
	socketMu.RLock()
	p := IncusSocketPath
	socketMu.RUnlock()
	if _, err := os.Stat(p); err == nil {
		return p
	}

	socketMu.Lock()
	defer socketMu.Unlock()
	if _, err := os.Stat(IncusSocketPath); err == nil {
		return IncusSocketPath
	}
	next := resolveIncusSocket()
	if next != IncusSocketPath {
		log.Printf("Incus socket 路径变化：%s -> %s", IncusSocketPath, next)
		IncusSocketPath = next
	}
	return IncusSocketPath
}

// randomToken 生成 n 字节的随机十六进制令牌
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退回时间无关的固定长度占位，仍比公开默认值安全
		return hex.EncodeToString([]byte("incus-web-fallback-token"))
	}
	return hex.EncodeToString(b)
}

// UsesLegacyDefaultToken 是否仍在使用旧版公开默认令牌（用于启动告警）
func UsesLegacyDefaultToken() bool {
	return APIToken == defaultTokenPlaceholder
}

// defaultDataDir 优先使用可执行文件同级目录，便于单文件部署时数据随身。
func defaultDataDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "data")
	}
	return "data"
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		n := 0
		for _, ch := range val {
			if ch < '0' || ch > '9' {
				return defaultVal
			}
			n = n*10 + int(ch-'0')
		}
		if n > 0 {
			return n
		}
	}
	return defaultVal
}
