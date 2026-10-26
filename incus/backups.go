package incus

import (
	"incus-web/model"
	"net/http"
)

// IncusBackupResponse 备份列表响应
type IncusBackupResponse struct {
	Type       string              `json:"type"`
	Status     string              `json:"status"`
	StatusCode int                 `json:"status_code"`
	Metadata   []IncusBackupRecord `json:"metadata"`
}

// IncusBackupRecord 备份记录
type IncusBackupRecord struct {
	Name             string `json:"name"`
	CreatedAt        string `json:"created_at"`
	ExpiresAt        string `json:"expires_at"`
	InstanceOnly     bool   `json:"instance_only"`
	OptimizedStorage bool   `json:"optimized_storage"`
	ContainerOnly    bool   `json:"container_only"`
}

// ListBackups 列出实例备份
func (c *Client) ListBackups(instanceName string) ([]model.BackupInfo, error) {
	var resp IncusBackupResponse
	if err := c.Get("/instances/"+instanceName+"/backups?recursion=1", &resp); err != nil {
		return nil, err
	}

	backups := []model.BackupInfo{}
	for _, rec := range resp.Metadata {
		backups = append(backups, model.BackupInfo{
			Name:             rec.Name,
			CreatedAt:        rec.CreatedAt,
			ExpiresAt:        rec.ExpiresAt,
			InstanceOnly:     rec.InstanceOnly,
			OptimizedStorage: rec.OptimizedStorage,
			ContainerOnly:    rec.ContainerOnly,
		})
	}
	return backups, nil
}

// CreateBackup 创建备份（异步）
func (c *Client) CreateBackup(instanceName string, req model.CreateBackupRequest) (string, error) {
	body := map[string]interface{}{
		"name":              req.Name,
		"instance_only":     req.InstanceOnly,
		"optimized_storage": req.OptimizedStorage,
	}
	if req.ExpiresAt != "" {
		body["expires_at"] = req.ExpiresAt
	}
	var opResp IncusOperationResponse
	if err := c.Post("/instances/"+instanceName+"/backups", body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// DownloadBackup 下载备份文件（返回原始响应体）
func (c *Client) DownloadBackup(instanceName, backupFile string) (*http.Response, error) {
	path := "/instances/" + instanceName + "/backups/" + backupFile + "/export"
	return c.GetRaw(path)
}

// DeleteBackup 删除备份
func (c *Client) DeleteBackup(instanceName, backupFile string) error {
	return c.Delete("/instances/"+instanceName+"/backups/"+backupFile, nil)
}

// ExportInstance 将实例发布为本地镜像（异步）。
// 注意：Incus 没有 POST /1.0/instances/{name}/export 端点，发布镜像走 POST /1.0/images，
// 通过 source.type=instance + source.name 指定来源实例（对应 CLI 的 incus publish）。
func (c *Client) ExportInstance(instanceName, imageName string) (string, error) {
	body := map[string]interface{}{
		"source": map[string]interface{}{
			"type": "instance",
			"name": instanceName,
		},
	}
	if imageName != "" {
		body["aliases"] = []map[string]string{{"name": imageName}}
	}
	var opResp IncusOperationResponse
	if err := c.Post("/images", body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}
