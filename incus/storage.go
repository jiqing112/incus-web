package incus

import (
	"incus-web/model"
)

// IncusStoragePoolResponse 存储池列表响应
type IncusStoragePoolResponse struct {
	Type       string                   `json:"type"`
	Status     string                   `json:"status"`
	StatusCode int                      `json:"status_code"`
	Metadata   []IncusStoragePoolRecord `json:"metadata"`
}

// IncusStoragePoolRecord 存储池记录
type IncusStoragePoolRecord struct {
	Name      string            `json:"name"`
	Driver    string            `json:"driver"`
	Config    map[string]string `json:"config"`
	Status    string            `json:"status"`
	UsedBy    []string          `json:"used_by"`
	Locations []string          `json:"locations"`
}

// IncusStorageVolumeResponse 存储卷列表响应
type IncusStorageVolumeResponse struct {
	Type       string                     `json:"type"`
	Status     string                     `json:"status"`
	StatusCode int                        `json:"status_code"`
	Metadata   []IncusStorageVolumeRecord `json:"metadata"`
}

// IncusStorageVolumeRecord 存储卷记录
type IncusStorageVolumeRecord struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Config   map[string]string `json:"config"`
	UsedBy   []string          `json:"used_by"`
	Location string            `json:"location"`
}

// ListStoragePools 列出所有存储池
func (c *Client) ListStoragePools() ([]model.StoragePoolInfo, error) {
	var resp IncusStoragePoolResponse
	if err := c.Get("/storage-pools?recursion=1", &resp); err != nil {
		return nil, err
	}

	pools := []model.StoragePoolInfo{}
	for _, rec := range resp.Metadata {
		pools = append(pools, model.StoragePoolInfo{
			Name:      rec.Name,
			Driver:    rec.Driver,
			Config:    rec.Config,
			Status:    rec.Status,
			UsedBy:    rec.UsedBy,
			Locations: rec.Locations,
		})
	}
	return pools, nil
}

// GetStoragePool 获取存储池详情
func (c *Client) GetStoragePool(name string) (*model.StoragePoolInfo, error) {
	type wrapper struct {
		Type       string                 `json:"type"`
		Status     string                 `json:"status"`
		StatusCode int                    `json:"status_code"`
		Metadata   IncusStoragePoolRecord `json:"metadata"`
	}
	var w wrapper
	if err := c.Get("/storage-pools/"+seg(name), &w); err != nil {
		return nil, err
	}
	rec := w.Metadata

	return &model.StoragePoolInfo{
		Name:      rec.Name,
		Driver:    rec.Driver,
		Config:    rec.Config,
		Status:    rec.Status,
		UsedBy:    rec.UsedBy,
		Locations: rec.Locations,
	}, nil
}

// CreateStoragePool 创建存储池
func (c *Client) CreateStoragePool(req model.CreateStoragePoolRequest) error {
	return c.Post("/storage-pools", req, nil)
}

// DeleteStoragePool 删除存储池
func (c *Client) DeleteStoragePool(name string) error {
	return c.Delete("/storage-pools/"+seg(name), nil)
}

// ListStorageVolumes 列出存储池中的所有卷
func (c *Client) ListStorageVolumes(poolName string) ([]model.StorageVolumeInfo, error) {
	var resp IncusStorageVolumeResponse
	if err := c.Get("/storage-pools/"+seg(poolName)+"/volumes?recursion=1", &resp); err != nil {
		return nil, err
	}

	volumes := []model.StorageVolumeInfo{}
	for _, rec := range resp.Metadata {
		volumes = append(volumes, model.StorageVolumeInfo{
			Name:     rec.Name,
			Type:     rec.Type,
			Config:   rec.Config,
			UsedBy:   rec.UsedBy,
			Location: rec.Location,
		})
	}
	return volumes, nil
}

// CreateStorageVolume 创建存储卷
func (c *Client) CreateStorageVolume(poolName string, req model.CreateStorageVolumeRequest) error {
	return c.Post("/storage-pools/"+seg(poolName)+"/volumes", req, nil)
}

// DeleteStorageVolume 删除存储卷
func (c *Client) DeleteStorageVolume(poolName, volType, volName string) error {
	return c.Delete("/storage-pools/"+seg(poolName)+"/volumes/"+seg(volType)+"/"+seg(volName), nil)
}
