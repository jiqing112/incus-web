package incus

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// TerminalExecResponse exec API 响应
type TerminalExecResponse struct {
	Type       string `json:"type"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Operation  string `json:"operation"`
	Metadata   struct {
		ID         string `json:"id"`
		Class      string `json:"class"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
		Resources  struct {
			Container map[string]string `json:"container"`
		} `json:"resources"`
		Metadata  map[string]interface{} `json:"metadata"`
		MayCancel bool                   `json:"may_cancel"`
		Err       string                 `json:"err"`
	} `json:"metadata"`
}

// TerminalConfig 终端配置
type TerminalConfig struct {
	Width  int
	Height int
}

// TerminalSession 终端会话
type TerminalSession struct {
	instanceName string
	wsConn       *websocket.Conn
	controlWS    *websocket.Conn
	done         chan struct{}
}

// NewTerminalSession 创建终端会话
func NewTerminalSession(instanceName string) *TerminalSession {
	return &TerminalSession{
		instanceName: instanceName,
		done:         make(chan struct{}, 1),
	}
}

// Connect 连接到实例终端
func (t *TerminalSession) Connect(wsConn *websocket.Conn, cfg TerminalConfig) error {
	t.wsConn = wsConn

	// 1. 创建 exec 会话，同时获取 fds
	opID, fd0Secret, controlSecret, err := createExecSession(t.instanceName, cfg)
	if err != nil {
		return fmt.Errorf("create exec session failed: %w", err)
	}

	// 2. 构建 WebSocket URL
	wsBase := "ws://unix/1.0/operations/" + opID + "/websocket?secret="
	fd0URL := wsBase + fd0Secret
	controlURL := wsBase + controlSecret

	// 3. 连接控制通道
	controlWS, err := connectIncusWebSocket(controlURL)
	if err != nil {
		return fmt.Errorf("connect control websocket failed: %w", err)
	}
	t.controlWS = controlWS
	// 会话结束时必须关闭控制通道：之前只在连接失败时关闭，
	// 正常结束会一直挂着，反复打开终端会持续泄漏连接。
	defer controlWS.Close()

	// 4. 连接交互式 WebSocket (fd 0)
	incusWS, err := connectIncusWebSocket(fd0URL)
	if err != nil {
		controlWS.Close()
		return fmt.Errorf("connect Incus WebSocket failed: %w", err)
	}
	defer incusWS.Close()

	// 5. 双向代理
	go t.proxyFromIncusToClient(incusWS)
	t.proxyFromClientToIncus(incusWS)

	return nil
}

func createExecSession(instanceName string, cfg TerminalConfig) (opID, fd0Secret, controlSecret string, err error) {
	client := &http.Client{Transport: newUnixTransport()}

	execBody := map[string]interface{}{
		"command": []string{"/bin/sh"},
		"environment": map[string]string{
			"TERM":  "xterm-256color",
			"HOME":  "/root",
			"SHELL": "/bin/sh",
		},
		"interactive":        true,
		"width":              cfg.Width,
		"height":             cfg.Height,
		"wait-for-websocket": true,
		"record-output":      false,
	}

	bodyBytes, _ := json.Marshal(execBody)
	req, _ := http.NewRequest("POST", "http://unix/1.0/instances/"+seg(instanceName)+"/exec", bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("exec API error (status %d): %s", resp.StatusCode, string(body))
	}

	var opResp TerminalExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&opResp); err != nil {
		return "", "", "", err
	}

	// 从 operation URL 提取 ID
	opID = opResp.Operation
	if idx := strings.LastIndex(opID, "/"); idx >= 0 {
		opID = opID[idx+1:]
	}

	// 从 metadata.metadata.fds 获取 WebSocket secrets
	fds, ok := opResp.Metadata.Metadata["fds"].(map[string]interface{})
	if !ok {
		return "", "", "", fmt.Errorf("no fds in exec response")
	}

	fd0, _ := fds["0"].(string)
	ctrl, _ := fds["control"].(string)
	if fd0 == "" || ctrl == "" {
		return "", "", "", fmt.Errorf("missing fds secrets")
	}

	return opID, fd0, ctrl, nil
}

func connectIncusWebSocket(url string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		NetDialContext: dialIncus,
	}
	conn, _, err := dialer.Dial(url, nil)
	return conn, err
}

func (t *TerminalSession) proxyFromIncusToClient(incusWS *websocket.Conn) {
	defer func() {
		// 非阻塞发送，避免 Connect 返回后无接收者导致 goroutine 泄漏
		select {
		case t.done <- struct{}{}:
		default:
		}
	}()

	for {
		msgType, data, err := incusWS.ReadMessage()
		if err != nil {
			log.Printf("Incus WS read error: %v", err)
			return
		}

		select {
		case <-t.done:
			return
		default:
		}

		if err := t.wsConn.WriteMessage(msgType, data); err != nil {
			log.Printf("Client WS write error: %v", err)
			return
		}
	}
}

func (t *TerminalSession) proxyFromClientToIncus(incusWS *websocket.Conn) {
	for {
		_, data, err := t.wsConn.ReadMessage()
		if err != nil {
			log.Printf("terminal[%s] client read error: %v", t.instanceName, err)
			return
		}

		// 检查是否为终端大小调整消息
		var resizeMsg struct {
			Type   string `json:"type"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		}
		if json.Unmarshal(data, &resizeMsg) == nil && resizeMsg.Type == "resize" {
			// 通过控制通道发送大小调整指令
			if t.controlWS != nil {
				ctrlMsg := map[string]interface{}{
					"command": "window-resize",
					"args": map[string]interface{}{
						"width":  fmt.Sprintf("%d", resizeMsg.Width),
						"height": fmt.Sprintf("%d", resizeMsg.Height),
					},
				}
				ctrlData, _ := json.Marshal(ctrlMsg)
				if err := t.controlWS.WriteMessage(websocket.TextMessage, ctrlData); err != nil {
					log.Printf("terminal[%s] control write error: %v", t.instanceName, err)
				} else {
					log.Printf("terminal[%s] resize forwarded %dx%d", t.instanceName, resizeMsg.Width, resizeMsg.Height)
				}
			}
			continue
		}

		// Incus exec websocket 只消费 binary 帧（text 帧会被服务端静默丢弃）
		if err := incusWS.WriteMessage(websocket.BinaryMessage, data); err != nil {
			log.Printf("terminal[%s] incus write error: %v", t.instanceName, err)
			return
		}
	}
}

// bytesReader 将 []byte 包装成 io.Reader
func bytesReader(data []byte) io.Reader {
	return &byteReader{data: data, pos: 0}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
