package event

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"incus-web/model"
)

// Broker SSE 事件分发器
type Broker struct {
	clients    map[chan string]struct{}
	mu         sync.RWMutex
	register   chan chan string
	unregister chan chan string
	stop       chan struct{}
}

// NewBroker 创建事件分发器
func NewBroker() *Broker {
	b := &Broker{
		clients:    make(map[chan string]struct{}),
		register:   make(chan chan string),
		unregister: make(chan chan string),
		stop:       make(chan struct{}),
	}
	go b.run()
	return b
}

func (b *Broker) run() {
	for {
		select {
		case ch := <-b.register:
			b.mu.Lock()
			b.clients[ch] = struct{}{}
			b.mu.Unlock()
		case ch := <-b.unregister:
			b.mu.Lock()
			delete(b.clients, ch)
			close(ch)
			b.mu.Unlock()
		case <-b.stop:
			return
		}
	}
}

// Subscribe 订阅事件，返回消息通道
func (b *Broker) Subscribe() chan string {
	ch := make(chan string, 64)
	b.register <- ch
	return ch
}

// Unsubscribe 取消订阅
func (b *Broker) Unsubscribe(ch chan string) {
	b.unregister <- ch
}

// Publish 发布事件到所有订阅者
func (b *Broker) Publish(event model.EventMessage) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	eventStr := string(data)
	eventStr = "data: " + eventStr + "\n\n"

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- eventStr:
		default:
			// 跳过阻塞的客户端
		}
	}
}

// PublishOperation 发布操作事件
func (b *Broker) PublishOperation(id, status, description string) {
	b.Publish(model.EventMessage{
		Type:      "operation",
		Status:    status,
		Payload:   map[string]string{"operation_id": id, "description": description},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishInstance 发布实例事件
func (b *Broker) PublishInstance(instanceName, action, status string) {
	b.Publish(model.EventMessage{
		Type:      "instance",
		Status:    status,
		Payload:   map[string]string{"instance": instanceName, "action": action},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Close 关闭分发器
func (b *Broker) Close() {
	close(b.stop)
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		delete(b.clients, ch)
		close(ch)
	}
}

// GlobalBroker 全局事件分发器
var GlobalBroker *Broker

func Init() {
	GlobalBroker = NewBroker()
	fmt.Println("Event broker initialized")
}
