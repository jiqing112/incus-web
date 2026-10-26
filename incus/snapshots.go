package incus

import (
	"incus-web/model"
)

// IncusSnapshotResponse 快照列表响应
type IncusSnapshotResponse struct {
	Type       string                `json:"type"`
	Status     string                `json:"status"`
	StatusCode int                   `json:"status_code"`
	Metadata   []IncusSnapshotRecord `json:"metadata"`
}

// IncusSnapshotRecord 快照记录
type IncusSnapshotRecord struct {
	Name      string `json:"name"`
	Stateful  bool   `json:"stateful"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

// ListSnapshots 列出实例所有快照
func (c *Client) ListSnapshots(instanceName string) ([]model.SnapshotInfo, error) {
	var resp IncusSnapshotResponse
	if err := c.Get("/instances/"+instanceName+"/snapshots?recursion=1", &resp); err != nil {
		return nil, err
	}

	snapshots := []model.SnapshotInfo{}
	for _, rec := range resp.Metadata {
		snapshots = append(snapshots, model.SnapshotInfo{
			Name:      rec.Name,
			Stateful:  rec.Stateful,
			CreatedAt: rec.CreatedAt,
			ExpiresAt: rec.ExpiresAt,
		})
	}
	return snapshots, nil
}

// CreateSnapshot 创建快照（异步）
func (c *Client) CreateSnapshot(instanceName string, req model.CreateSnapshotRequest) (string, error) {
	body := map[string]interface{}{
		"name":     req.Name,
		"stateful": req.Stateful,
	}
	if req.ExpiresAt != "" {
		body["expires_at"] = req.ExpiresAt
	}
	var opResp IncusOperationResponse
	if err := c.Post("/instances/"+instanceName+"/snapshots", body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// RenameSnapshot 重命名快照（异步）。
// Incus 通过 POST /instances/{name}/snapshots/{snap} 携带 {"name": "新名"} 实现改名。
func (c *Client) RenameSnapshot(instanceName, snapshotName, newName string) (string, error) {
	body := map[string]interface{}{
		"name": newName,
	}
	var opResp IncusOperationResponse
	if err := c.Post("/instances/"+instanceName+"/snapshots/"+snapshotName, body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// RestoreSnapshot 恢复快照（异步）。
// stateful=true 且目标快照为 stateful 时，会连同运行时内存状态一起恢复
// （PUT /1.0/instances/{name} 的 body 需带 {"restore": "<snap>", "stateful": true}）。
func (c *Client) RestoreSnapshot(instanceName, snapshotName string, stateful bool) (string, error) {
	body := map[string]interface{}{
		"restore": snapshotName,
	}
	if stateful {
		body["stateful"] = true
	}
	var opResp IncusOperationResponse
	if err := c.Put("/instances/"+instanceName, body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// DeleteSnapshot 删除快照（异步）
func (c *Client) DeleteSnapshot(instanceName, snapshotName string) (string, error) {
	var opResp IncusOperationResponse
	if err := c.Delete("/instances/"+instanceName+"/snapshots/"+snapshotName, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}
