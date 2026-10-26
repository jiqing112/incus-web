package service

import (
	"incus-web/incus"
	"incus-web/model"
)

// StorageService 存储业务逻辑
type StorageService struct {
	client *incus.Client
}

// NewStorageService 创建存储服务
func NewStorageService(client *incus.Client) *StorageService {
	return &StorageService{client: client}
}

// ListStoragePools 列出所有存储池
func (s *StorageService) ListStoragePools() ([]model.StoragePoolInfo, error) {
	return s.client.ListStoragePools()
}

// GetStoragePool 获取存储池详情
func (s *StorageService) GetStoragePool(name string) (*model.StoragePoolInfo, error) {
	return s.client.GetStoragePool(name)
}

// CreateStoragePool 创建存储池
func (s *StorageService) CreateStoragePool(req model.CreateStoragePoolRequest) error {
	return s.client.CreateStoragePool(req)
}

// DeleteStoragePool 删除存储池
func (s *StorageService) DeleteStoragePool(name string) error {
	return s.client.DeleteStoragePool(name)
}

// ListStorageVolumes 列出存储卷
func (s *StorageService) ListStorageVolumes(poolName string) ([]model.StorageVolumeInfo, error) {
	return s.client.ListStorageVolumes(poolName)
}

// CreateStorageVolume 创建存储卷
func (s *StorageService) CreateStorageVolume(poolName string, req model.CreateStorageVolumeRequest) error {
	return s.client.CreateStorageVolume(poolName, req)
}

// DeleteStorageVolume 删除存储卷
func (s *StorageService) DeleteStorageVolume(poolName, volType, volName string) error {
	return s.client.DeleteStorageVolume(poolName, volType, volName)
}
