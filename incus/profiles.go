package incus

import (
	"incus-web/model"
)

// IncusProfileResponse 配置文件列表响应
type IncusProfileResponse struct {
	Type       string               `json:"type"`
	Status     string               `json:"status"`
	StatusCode int                  `json:"status_code"`
	Metadata   []IncusProfileRecord `json:"metadata"`
}

// IncusProfileRecord 配置文件记录
type IncusProfileRecord struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Config      map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"devices"`
	UsedBy      []string                     `json:"used_by"`
}

// ListProfiles 列出所有 profile
func (c *Client) ListProfiles() ([]model.ProfileInfo, error) {
	var resp IncusProfileResponse
	if err := c.Get("/profiles?recursion=1", &resp); err != nil {
		return nil, err
	}

	profiles := []model.ProfileInfo{}
	for _, rec := range resp.Metadata {
		profiles = append(profiles, model.ProfileInfo{
			Name:        rec.Name,
			Description: rec.Description,
			Config:      rec.Config,
			Devices:     rec.Devices,
			UsedBy:      rec.UsedBy,
		})
	}
	return profiles, nil
}

// GetProfile 获取 profile 详情
func (c *Client) GetProfile(name string) (*model.ProfileInfo, error) {
	type wrapper struct {
		Type       string             `json:"type"`
		Status     string             `json:"status"`
		StatusCode int                `json:"status_code"`
		Metadata   IncusProfileRecord `json:"metadata"`
	}
	var w wrapper
	if err := c.Get("/profiles/"+seg(name), &w); err != nil {
		return nil, err
	}
	rec := w.Metadata

	return &model.ProfileInfo{
		Name:        rec.Name,
		Description: rec.Description,
		Config:      rec.Config,
		Devices:     rec.Devices,
		UsedBy:      rec.UsedBy,
	}, nil
}

// CreateProfile 创建 profile
func (c *Client) CreateProfile(req model.CreateProfileRequest) error {
	incusReq := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"config":      req.Config,
		"devices":     req.Devices,
	}
	if incusReq["config"] == nil {
		incusReq["config"] = map[string]string{}
	}
	if incusReq["devices"] == nil {
		incusReq["devices"] = map[string]map[string]string{}
	}
	return c.Post("/profiles", incusReq, nil)
}

// UpdateProfile 更新 profile
func (c *Client) UpdateProfile(name string, req model.UpdateProfileRequest) error {
	incusReq := map[string]interface{}{
		"description": req.Description,
		"config":      req.Config,
		"devices":     req.Devices,
	}
	return c.Put("/profiles/"+seg(name), incusReq, nil)
}

// DeleteProfile 删除 profile
func (c *Client) DeleteProfile(name string) error {
	return c.Delete("/profiles/"+seg(name), nil)
}
