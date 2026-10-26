package service

import (
	"incus-web/incus"
	"incus-web/model"
)

// NetworkService 网络业务逻辑
type NetworkService struct {
	client *incus.Client
}

// NewNetworkService 创建网络服务
func NewNetworkService(client *incus.Client) *NetworkService {
	return &NetworkService{client: client}
}

// ListNetworks 列出所有网络
func (s *NetworkService) ListNetworks() ([]model.NetworkInfo, error) {
	return s.client.ListNetworks()
}

// GetNetwork 获取网络详情
func (s *NetworkService) GetNetwork(name string) (*model.NetworkInfo, error) {
	return s.client.GetNetwork(name)
}

// CreateNetwork 创建网络
func (s *NetworkService) CreateNetwork(req model.CreateNetworkRequest) error {
	return s.client.CreateNetwork(req)
}

// UpdateNetwork 更新网络配置
func (s *NetworkService) UpdateNetwork(name string, req model.UpdateNetworkRequest) error {
	return s.client.UpdateNetwork(name, req)
}

// DeleteNetwork 删除网络
func (s *NetworkService) DeleteNetwork(name string) error {
	return s.client.DeleteNetwork(name)
}
