package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"incus-web/incus"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	// 只允许同源页面建立终端连接：token 走查询参数，
	// 若放行任意 Origin，被访问的第三方页面即可复用泄露的 token 打开 shell。
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // 非浏览器客户端（CLI / 测试脚本）
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(u.Host, r.Host)
	},
}

// TerminalHandler 终端 WebSocket 处理器
type TerminalHandler struct{}

// NewTerminalHandler 创建终端处理器
func NewTerminalHandler() *TerminalHandler {
	return &TerminalHandler{}
}

// HandleTerminal GET /api/instances/:name/terminal (WebSocket)
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	instanceName := c.Param("name")

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	// 终端输入以按键为主，限制单帧大小避免被大报文拖垮
	conn.SetReadLimit(1 << 20)

	session := incus.NewTerminalSession(instanceName)
	cfg := incus.TerminalConfig{Width: 80, Height: 24}

	// 读取第一条消息获取初始窗口大小
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Read initial message failed: %v", err)
		return
	}

	var initMsg struct {
		Type   string `json:"type"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if json.Unmarshal(msg, &initMsg) == nil && initMsg.Type == "resize" {
		if initMsg.Width > 0 {
			cfg.Width = initMsg.Width
		}
		if initMsg.Height > 0 {
			cfg.Height = initMsg.Height
		}
	}

	if err := session.Connect(conn, cfg); err != nil {
		log.Printf("Terminal session error: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
	}
}
