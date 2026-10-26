package incus

import (
	"encoding/json"
	"log"
	"time"

	"incus-web/event"
	"incus-web/model"

	"github.com/gorilla/websocket"
)

// EventClient 连接到 Incus 事件的 WebSocket 客户端
type EventClient struct {
	conn   *websocket.Conn
	broker *event.Broker
	done   chan struct{}
}

// StartEventClient 启动 Incus 事件监听
func StartEventClient(broker *event.Broker) {
	dialer := websocket.Dialer{
		NetDialContext:   dialIncus,
		HandshakeTimeout: 10 * time.Second,
	}

	client := &EventClient{
		broker: broker,
		done:   make(chan struct{}),
	}

	go client.connectLoop(dialer)
}

func (c *EventClient) connectLoop(dialer websocket.Dialer) {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn, _, err := dialer.Dial("ws://unix/1.0/events", nil)
		if err != nil {
			log.Printf("Failed to connect to Incus events: %v, retrying in 5s...", err)
			select {
			case <-c.done:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		c.conn = conn
		log.Println("Connected to Incus event stream")
		c.readLoop()
	}
}

func (c *EventClient) readLoop() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Incus event read error: %v", err)
			c.conn.Close()
			return
		}

		var incusEvent map[string]interface{}
		if json.Unmarshal(message, &incusEvent) != nil {
			continue
		}

		eventType := "incus_event"
		if t, ok := incusEvent["type"].(string); ok {
			eventType = t
		}

		c.broker.Publish(model.EventMessage{
			Type:      eventType,
			Status:    "updated",
			Payload:   incusEvent,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// Close 关闭事件客户端
func (c *EventClient) Close() {
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
}
