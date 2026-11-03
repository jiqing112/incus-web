package service

import (
	"log"
	"strings"
	"time"

	"incus-web/event"
	"incus-web/incus"
	"incus-web/model"
)

// InstanceService 实例业务逻辑
type InstanceService struct {
	client *incus.Client
	broker *event.Broker
}

// NewInstanceService 创建实例服务
func NewInstanceService(client *incus.Client, broker *event.Broker) *InstanceService {
	return &InstanceService{client: client, broker: broker}
}

// ListInstances 列出所有实例
func (s *InstanceService) ListInstances(instanceType, status string) ([]model.InstanceInfo, error) {
	return s.client.ListInstances(instanceType, status)
}

// GetInstance 获取实例详情
func (s *InstanceService) GetInstance(name string) (*model.InstanceInfo, error) {
	return s.client.GetInstance(name)
}

// CreateInstance 创建实例（异步）
func (s *InstanceService) CreateInstance(req model.CreateInstanceRequest) (string, error) {
	opURL, err := s.client.CreateInstance(req)
	if err != nil {
		return "", err
	}
	return opURL, nil
}

// UpdateInstanceConfig 更新实例配置
func (s *InstanceService) UpdateInstanceConfig(name string, req model.UpdateInstanceConfigRequest) (string, error) {
	return s.client.UpdateInstanceConfig(name, req)
}

// DeleteInstance 删除实例（异步）
//
// Incus 的 DELETE /1.0/instances/{name}?force=true 对运行中实例仍然返回
// 400 "Instance is running"（与 CLI --force 行为不同），所以必须先 stop 再 delete。
// 已停止的实例直接删除一步到位；运行中的实例先发 stop 并把 stop 的 operation
// 返回给调用方轮询，等停稳后的删除放到后台执行——避免 HTTP 请求被阻塞最长一分钟。
func (s *InstanceService) DeleteInstance(name string, force bool) (string, error) {
	opURL, err := s.client.DeleteInstance(name, force)
	if err == nil {
		// 已停止实例一步删除成功，同样要广播事件，否则前端列表不会自动刷新
		s.broker.PublishInstance(name, "delete", "deleted")
		return opURL, nil
	}
	if !strings.Contains(err.Error(), "Instance is running") {
		return "", err
	}

	stopURL, serr := s.client.ChangeInstanceState(name, model.StateRequest{Action: "stop", Force: true, Timeout: 60})
	if serr != nil {
		return "", serr
	}
	go s.stopThenDelete(name, force)
	return stopURL, nil
}

// stopThenDelete 等待实例真正停止后执行删除（在后台 goroutine 中运行）。
// 完成或失败都广播实例事件，驱动前端列表刷新。
func (s *InstanceService) stopThenDelete(name string, force bool) {
	// 最多等 90 秒；期间实例被外部删掉（查询报错）也直接进入删除尝试
	for i := 0; i < 90; i++ {
		time.Sleep(time.Second)
		info, err := s.client.GetInstance(name)
		if err != nil {
			break
		}
		if info.Status == "Stopped" {
			break
		}
	}
	if _, err := s.client.DeleteInstance(name, force); err != nil {
		log.Printf("delete instance %s failed after stop: %v", name, err)
		s.broker.PublishInstance(name, "delete", "error")
		return
	}
	s.broker.PublishInstance(name, "delete", "deleted")
}

// ChangeInstanceState 变更实例状态
func (s *InstanceService) ChangeInstanceState(name string, req model.StateRequest) (string, error) {
	opURL, err := s.client.ChangeInstanceState(name, req)
	if err != nil {
		return "", err
	}
	// 事件状态应与动作对应（此前无论 start/stop/restart 都发布 "started"）
	status := "started"
	switch req.Action {
	case "stop":
		status = "stopped"
	case "restart":
		status = "restarted"
	}
	s.broker.PublishInstance(name, req.Action, status)
	return opURL, nil
}
