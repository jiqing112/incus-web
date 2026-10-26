package service

import (
	"incus-web/incus"
	"incus-web/model"
)

// ProfileService 配置文件业务逻辑
type ProfileService struct {
	client *incus.Client
}

// NewProfileService 创建配置文件服务
func NewProfileService(client *incus.Client) *ProfileService {
	return &ProfileService{client: client}
}

// ListProfiles 列出所有 profile
func (s *ProfileService) ListProfiles() ([]model.ProfileInfo, error) {
	return s.client.ListProfiles()
}

// GetProfile 获取 profile 详情
func (s *ProfileService) GetProfile(name string) (*model.ProfileInfo, error) {
	return s.client.GetProfile(name)
}

// CreateProfile 创建 profile
func (s *ProfileService) CreateProfile(req model.CreateProfileRequest) error {
	return s.client.CreateProfile(req)
}

// UpdateProfile 更新 profile
func (s *ProfileService) UpdateProfile(name string, req model.UpdateProfileRequest) error {
	return s.client.UpdateProfile(name, req)
}

// DeleteProfile 删除 profile
func (s *ProfileService) DeleteProfile(name string) error {
	return s.client.DeleteProfile(name)
}
