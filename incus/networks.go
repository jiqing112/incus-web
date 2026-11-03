package incus

import (
	"incus-web/model"
)

// IncusNetworkResponse 网络列表响应
type IncusNetworkResponse struct {
	Type       string               `json:"type"`
	Status     string               `json:"status"`
	StatusCode int                  `json:"status_code"`
	Metadata   []IncusNetworkRecord `json:"metadata"`
}

// IncusNetworkRecord 网络记录
type IncusNetworkRecord struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Managed     bool              `json:"managed"`
	Status      string            `json:"status"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
	Locations   []string          `json:"locations"`
}

// IncusNetworkLeaseResponse DHCP 租约响应
type IncusNetworkLeaseResponse struct {
	Type       string                    `json:"type"`
	Status     string                    `json:"status"`
	StatusCode int                       `json:"status_code"`
	Metadata   []IncusNetworkLeaseRecord `json:"metadata"`
}

// IncusNetworkLeaseRecord 租约记录
type IncusNetworkLeaseRecord struct {
	Hostname string `json:"hostname"`
	Hwaddr   string `json:"hwaddr"`
	Address  string `json:"address"`
	Type     string `json:"type"`
}

// ListNetworks 列出所有网络
func (c *Client) ListNetworks() ([]model.NetworkInfo, error) {
	var resp IncusNetworkResponse
	if err := c.Get("/networks?recursion=1", &resp); err != nil {
		return nil, err
	}

	networks := []model.NetworkInfo{}
	for _, rec := range resp.Metadata {
		networks = append(networks, model.NetworkInfo{
			Name:        rec.Name,
			Type:        rec.Type,
			Description: rec.Description,
			Managed:     rec.Managed,
			Status:      rec.Status,
			Config:      rec.Config,
			UsedBy:      rec.UsedBy,
			Locations:   rec.Locations,
		})
	}
	return networks, nil
}

// GetNetwork 获取网络详情（包含 DHCP 租约）
func (c *Client) GetNetwork(name string) (*model.NetworkInfo, error) {
	type wrapper struct {
		Type       string             `json:"type"`
		Status     string             `json:"status"`
		StatusCode int                `json:"status_code"`
		Metadata   IncusNetworkRecord `json:"metadata"`
	}
	var w wrapper
	if err := c.Get("/networks/"+seg(name), &w); err != nil {
		return nil, err
	}
	rec := w.Metadata

	network := &model.NetworkInfo{
		Name:        rec.Name,
		Type:        rec.Type,
		Description: rec.Description,
		Managed:     rec.Managed,
		Status:      rec.Status,
		Config:      rec.Config,
		UsedBy:      rec.UsedBy,
		Locations:   rec.Locations,
	}

	// 获取 DHCP 租约
	leases, err := c.GetNetworkLeases(name)
	if err == nil {
		network.Leases = leases
	}

	return network, nil
}

// GetNetworkLeases 获取网络 DHCP 租约
func (c *Client) GetNetworkLeases(name string) ([]model.NetworkLease, error) {
	var resp IncusNetworkLeaseResponse
	if err := c.Get("/networks/"+seg(name)+"/leases", &resp); err != nil {
		return nil, err
	}

	leases := []model.NetworkLease{}
	for _, rec := range resp.Metadata {
		leases = append(leases, model.NetworkLease{
			Hostname: rec.Hostname,
			Hwaddr:   rec.Hwaddr,
			Address:  rec.Address,
			Type:     rec.Type,
		})
	}
	return leases, nil
}

// CreateNetwork 创建网络
func (c *Client) CreateNetwork(req model.CreateNetworkRequest) error {
	incusReq := map[string]interface{}{
		"name":   req.Name,
		"type":   req.Type,
		"config": req.Config,
	}
	if incusReq["config"] == nil {
		incusReq["config"] = map[string]string{}
	}
	return c.Post("/networks", incusReq, nil)
}

// UpdateNetwork 更新网络配置
func (c *Client) UpdateNetwork(name string, req model.UpdateNetworkRequest) error {
	incusReq := map[string]interface{}{
		"config":      req.Config,
		"description": req.Description,
	}
	return c.Put("/networks/"+seg(name), incusReq, nil)
}

// DeleteNetwork 删除网络
func (c *Client) DeleteNetwork(name string) error {
	return c.Delete("/networks/"+seg(name), nil)
}
