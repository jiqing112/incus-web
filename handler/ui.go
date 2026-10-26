package handler

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// ServeUI 服务 SPA：优先返回静态资源文件，未命中回退到 index.html（前端为 hash 路由）。
func ServeUI(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// 带扩展名的请求按静态资源处理
	if idx := strings.LastIndexByte(path, '.'); idx > strings.LastIndexByte(path, '/') {
		data, err := distFS.ReadFile("dist/" + path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, contentTypeOf(path), data)
		return
	}

	data, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "ui assets missing: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// ServeStatic 供 /assets/ 前缀等静态资源路由复用
func ServeStatic(c *gin.Context) {
	ServeUI(c)
}

func contentTypeOf(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".webmanifest"):
		return "application/manifest+json"
	case strings.HasSuffix(path, ".map"):
		return "application/json"
	}
	if _, err := fs.Stat(distFS, "dist/"+path); err != nil {
		return "application/octet-stream"
	}
	return "application/octet-stream"
}
