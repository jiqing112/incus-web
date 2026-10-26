package service

import (
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
// 这里先直接尝试删除（已停止的实例一步到位），命中 "Instance is running" 时
// 发 stop 并轮询重试删除，避免把「运行中无法删除」暴露给用户。
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

	if _, serr := s.client.ChangeInstanceState(name, model.StateRequest{Action: "stop", Force: true, Timeout: 60}); serr != nil {
		return "", serr
	}
	// stop 是异步操作，需要等实例真正停下后再删
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		if opURL, err = s.client.DeleteInstance(name, force); err == nil {
			s.broker.PublishInstance(name, "delete", "deleted")
			return opURL, nil
		}
		if !strings.Contains(err.Error(), "Instance is running") {
			return "", err
		}
	}
	return "", err
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
