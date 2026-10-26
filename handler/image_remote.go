package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// ImageRemote 镜像远程源
type ImageRemote struct {
	Name     string `json:"name"`
	Addr     string `json:"addr"`
	Protocol string `json:"protocol"`
	Static   bool   `json:"static"`
}

// RemoteImage 搜索到的远程镜像
type RemoteImage struct {
	Alias       string   `json:"alias"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
	OS          string   `json:"os"`
	Release     string   `json:"release"`
	Fingerprint string   `json:"fingerprint"`
	Size        int64    `json:"size"`
	Type        string   `json:"type"`
}

var remoteNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// findRemote 在已配置的远程源中按名称查找（用于保存失败时回滚）
func findRemote(name string) (ImageRemote, bool) {
	list, err := fetchRemotes()
	if err != nil {
		return ImageRemote{}, false
	}
	for _, r := range list {
		if r.Name == name {
			return r, true
		}
	}
	return ImageRemote{}, false
}

// remoteList 返回已配置的远程源。命中磁盘缓存时立即返回，否则回退到实时读取。
func (h *NetworkDiagHandler) remoteList(c *gin.Context) {
	if list, updatedAt, ok := h.imgCache.GetRemotes(); ok {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
			"remotes":    list,
			"cached":     true,
			"updated_at": updatedAt.Format(time.RFC3339),
		}})
		return
	}
	list, err := fetchRemotes()
	if err != nil {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: err.Error()})
		return
	}
	h.imgCache.SetRemotes(list)
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
		"remotes":    list,
		"cached":     false,
		"updated_at": time.Now().Format(time.RFC3339),
	}})
}

func (h *NetworkDiagHandler) remoteAdd(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.URL == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "name 与 url 为必填"})
		return
	}
	if !remoteNameRe.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "名称仅允许字母、数字、下划线与中划线"})
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "URL 需以 http(s):// 开头"})
		return
	}
	// 已存在则先移除再添加（等效「保存」）。
	// 若新地址添加失败，回滚到原配置 —— 否则一次手误就会把可用的远程源删掉。
	old, hadOld := findRemote(req.Name)
	runCmd(10*time.Second, "incus", "remote", "remove", req.Name)
	if out, code := runCmd(15*time.Second, "incus", "remote", "add", req.Name, req.URL, "--protocol=simplestreams"); code != 0 {
		restored := false
		if hadOld {
			proto := old.Protocol
			if proto == "" {
				proto = "simplestreams"
			}
			if _, rc := runCmd(15*time.Second, "incus", "remote", "add", old.Name, old.Addr, "--protocol="+proto); rc == 0 {
				restored = true
				h.imgCache.Invalidate(req.Name)
				if list, err := fetchRemotes(); err == nil {
					h.imgCache.SetRemotes(list)
				}
			}
		}
		msg := "添加远程源失败: " + strings.TrimSpace(out)
		if restored {
			msg += "（已恢复原有配置）"
		} else if hadOld {
			msg += "（原配置恢复失败，请手动重新添加）"
		}
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: msg})
		return
	}
	h.imgCache.Invalidate(req.Name)
	if list, err := fetchRemotes(); err == nil {
		h.imgCache.SetRemotes(list)
	}
	// 新增/重建的源立即在后台预热索引，用户打开搜索框时即可命中
	h.imgCache.refreshAsync(req.Name)
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{"message": "远程源已保存"}})
}

func (h *NetworkDiagHandler) remoteRemove(c *gin.Context) {
	name := c.Param("name")
	if !remoteNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "非法名称"})
		return
	}
	if out, code := runCmd(10*time.Second, "incus", "remote", "remove", name); code != 0 {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "移除失败: " + strings.TrimSpace(out)})
		return
	}
	h.imgCache.Invalidate(name)
	if list, err := fetchRemotes(); err == nil {
		h.imgCache.SetRemotes(list)
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{"message": "远程源已移除"}})
}

// remoteImages 搜索远程镜像：缓存优先。
//   - 命中缓存：立即返回，附带 updated_at / refreshing 元信息；软过期时顺带触发后台刷新
//   - 未命中：同步拉取一次（首次），随后落盘
//   - refresh=1：强制同步刷新后再返回
func (h *NetworkDiagHandler) remoteImages(c *gin.Context) {
	name := c.Param("name")
	if !remoteNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "非法远程源名称"})
		return
	}
	keyword := c.Query("keyword")
	force := c.Query("refresh") == "1"

	entry, hit := h.imgCache.GetImages(name)
	if force || !hit {
		if err := h.imgCache.Refresh(name); err != nil && !hit {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: err.Error()})
			return
		}
		entry, _ = h.imgCache.GetImages(name)
	} else if time.Since(entry.UpdatedAt) > softTTL {
		// 有缓存但已软过期：先返回旧数据，同时后台刷新，用户无感知
		h.imgCache.refreshAsync(name)
	}

	resp := gin.H{
		"images":     FilterRemoteImages(entry.Images, keyword),
		"total":      len(entry.Images),
		"cached":     hit && !force,
		"refreshing": h.imgCache.IsRefreshing(name),
		"updated_at": entry.UpdatedAt.Format(time.RFC3339),
	}
	if entry.Error != "" {
		resp["error"] = entry.Error
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: resp})
}

// remoteRefresh 强制刷新单个远程源（POST /image-remotes/:name/refresh）
func (h *NetworkDiagHandler) remoteRefresh(c *gin.Context) {
	name := c.Param("name")
	if !remoteNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "非法远程源名称"})
		return
	}
	if err := h.imgCache.Refresh(name); err != nil {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: err.Error()})
		return
	}
	entry, _ := h.imgCache.GetImages(name)
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
		"images":     FilterRemoteImages(entry.Images, c.Query("keyword")),
		"total":      len(entry.Images),
		"cached":     false,
		"refreshing": false,
		"updated_at": entry.UpdatedAt.Format(time.RFC3339),
	}})
}

// RegisterImageRemoteRoutes 注册远程源管理路由
// 注意：gin 的路由树不允许同一层级同时存在静态段与通配段，
// 因此不提供 /image-remotes/refresh 这类与 /image-remotes/:name 冲突的路径，
// 「刷新全部」由后台定时任务承担。
func (h *NetworkDiagHandler) RegisterImageRemoteRoutes(rg *gin.RouterGroup) {
	rg.GET("/image-remotes", h.remoteList)
	rg.POST("/image-remotes", h.remoteAdd)
	rg.DELETE("/image-remotes/:name", h.remoteRemove)
	rg.GET("/image-remotes/:name/images", h.remoteImages)
	rg.POST("/image-remotes/:name/refresh", h.remoteRefresh)
}
