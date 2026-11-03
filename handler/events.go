package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"incus-web/event"
	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// EventHandler 事件处理器
type EventHandler struct {
	broker *event.Broker
	client *incus.Client
}

// NewEventHandler 创建事件处理器
func NewEventHandler(broker *event.Broker, client *incus.Client) *EventHandler {
	return &EventHandler{broker: broker, client: client}
}

// sseHeartbeatInterval 心跳间隔：避免反向代理/浏览器把长时间无数据的长连接判定为空闲断开
const sseHeartbeatInterval = 25 * time.Second

// HandleGlobalEvents GET /api/events (SSE)
func (h *EventHandler) HandleGlobalEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := h.broker.Subscribe()
	defer h.broker.Unsubscribe(ch)

	clientGone := c.Request.Context().Done()
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	// 立即冲刷响应头，让浏览器尽早触发 onopen
	c.Writer.Flush()

	for {
		select {
		case <-clientGone:
			return
		case <-heartbeat.C:
			// SSE 注释行，客户端会忽略
			fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		case msg := <-ch:
			fmt.Fprint(c.Writer, msg)
			c.Writer.Flush()
		}
	}
}

// GetLogs GET /api/instances/:name/logs
func (h *EventHandler) GetLogs(c *gin.Context) {
	instanceName := c.Param("name")
	lines := 100

	logs, err := h.client.GetLogs(instanceName, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{"logs": logs}})
}

// StreamLogs GET /api/instances/:name/logs/stream (SSE)
//
// 实现方式：按固定间隔拉取日志尾部并只推送新增行。
// - 初始回填与每轮拉取都走 Incus 的 ?lines= 尾部窗口（初始 2000 行、增量 2000 行），
//   不再每 2 秒重下整个日志文件（长期运行容器的 lxc.log 可能是几十 MB）。
// - 增量去重：记住上次发送的最后一行，在新尾部中找它最后一次出现的位置，其后即为新增；
//   找不到（轮转/新增超过窗口）时保守推送整个窗口。
// - 之前用 exec + websocket 取流是错误的：非交互 exec 的 fd 0 是 stdin（输出在 fd 1），
//   且原始字节直接塞进 "data:" 行，日志一换行就会破坏 SSE 帧格式。
func (h *EventHandler) StreamLogs(c *gin.Context) {
	instanceName := c.Param("name")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	const tailLines = 2000

	full, err := h.client.GetLogs(instanceName, tailLines)
	if err != nil {
		writeSSEError(c, err.Error())
		return
	}
	lines := splitLogLines(full)
	for _, line := range lines {
		writeSSELine(c, line)
	}
	c.Writer.Flush()

	// 上次已推送的最后一行（空串表示还没有推送过任何行）
	lastSent := ""
	if len(lines) > 0 {
		lastSent = lines[len(lines)-1]
	}

	clientGone := c.Request.Context().Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			cur, err := h.client.GetLogs(instanceName, tailLines)
			if err != nil {
				writeSSEError(c, err.Error())
				return
			}
			newLines := splitLogLines(cur)
			start := 0
			if lastSent != "" {
				found := -1
				for i, line := range newLines {
					if line == lastSent {
						found = i
					}
				}
				if found >= 0 {
					start = found + 1
				}
				// 找不到：日志被轮转/截断，或窗口内新增超过 tailLines 行——保守重发整个窗口
			}
			if start < len(newLines) {
				for _, line := range newLines[start:] {
					writeSSELine(c, line)
				}
				lastSent = newLines[len(newLines)-1]
				c.Writer.Flush()
			}
		}
	}
}

// writeSSELine 写入单行 SSE 数据帧（保证一行一个 data:，不破坏帧边界）
func writeSSELine(c *gin.Context, line string) {
	line = strings.ReplaceAll(line, "\r", "")
	fmt.Fprintf(c.Writer, "data: %s\n\n", line)
}

// writeSSEError 以 SSE 帧返回错误（JSON 形式，便于前端统一解析）
func writeSSEError(c *gin.Context, msg string) {
	b, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		b = []byte(`{"error":"unknown error"}`)
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", b)
	c.Writer.Flush()
}

// splitLogLines 按行切分日志增量（去掉末尾换行，忽略空片段）
func splitLogLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
