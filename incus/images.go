package incus

import (
	"incus-web/model"
	"io"
	"net/http"
	"net/url"
)

// IncusImageResponse 镜像列表响应
type IncusImageResponse struct {
	Type       string             `json:"type"`
	Status     string             `json:"status"`
	StatusCode int                `json:"status_code"`
	Metadata   []IncusImageRecord `json:"metadata"`
}

// IncusImageRecord Incus 镜像记录
type IncusImageRecord struct {
	Fingerprint  string `json:"fingerprint"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Architecture string `json:"architecture"`
	Type         string `json:"type"`
	CreatedAt    string `json:"created_at"`
	UploadedAt   string `json:"uploaded_at"`
	ExpiresAt    string `json:"expires_at"`
	Aliases      []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"aliases"`
	Properties map[string]string `json:"properties"`
	Public     bool              `json:"public"`
	Cached     bool              `json:"cached"`
	AutoUpdate bool              `json:"auto_update"`
}

// IncusRemoteImageResponse 远程镜像响应
type IncusRemoteImageResponse struct {
	Type       string                   `json:"type"`
	Status     string                   `json:"status"`
	StatusCode int                      `json:"status_code"`
	Metadata   []IncusRemoteImageRecord `json:"metadata"`
}

// IncusRemoteImageRecord 远程镜像记录
type IncusRemoteImageRecord struct {
	Alias        string            `json:"alias"`
	Fingerprint  string            `json:"fingerprint"`
	Architecture string            `json:"architecture"`
	Description  string            `json:"description"`
	Release      string            `json:"release"`
	Variant      string            `json:"variant"`
	Size         int64             `json:"size"`
	Properties   map[string]string `json:"properties"`
}

// ListLocalImages 列出本地镜像
func (c *Client) ListLocalImages() ([]model.ImageInfo, error) {
	var resp IncusImageResponse
	if err := c.Get("/images?recursion=1", &resp); err != nil {
		return nil, err
	}

	images := []model.ImageInfo{}
	for _, rec := range resp.Metadata {
		aliases := make([]model.ImageAlias, len(rec.Aliases))
		for i, a := range rec.Aliases {
			aliases[i] = model.ImageAlias{Name: a.Name, Description: a.Description}
		}
		images = append(images, model.ImageInfo{
			Fingerprint:  rec.Fingerprint,
			Filename:     rec.Filename,
			Size:         rec.Size,
			Architecture: rec.Architecture,
			Type:         rec.Type,
			CreatedAt:    rec.CreatedAt,
			UploadedAt:   rec.UploadedAt,
			ExpiresAt:    rec.ExpiresAt,
			Aliases:      aliases,
			Properties:   rec.Properties,
			Public:       rec.Public,
			Cached:       rec.Cached,
			AutoUpdate:   rec.AutoUpdate,
		})
	}
	return images, nil
}

// ListRemoteImages 列出远程镜像服务器的镜像
func (c *Client) ListRemoteImages(serverURL string) ([]model.RemoteImageInfo, error) {
	// 通过 Incus REST API 查询远程服务器镜像
	path := "/images?recursion=1&remote=" + url.QueryEscape(serverURL)

	var resp IncusRemoteImageResponse
	if err := c.Get(path, &resp); err != nil {
		return nil, err
	}

	images := []model.RemoteImageInfo{}
	for _, rec := range resp.Metadata {
		images = append(images, model.RemoteImageInfo{
			Alias:        rec.Alias,
			Fingerprint:  rec.Fingerprint,
			Architecture: rec.Architecture,
			Description:  rec.Description,
			Release:      rec.Release,
			Variant:      rec.Variant,
			Size:         rec.Size,
		})
	}
	return images, nil
}

// RefreshRemoteImages 刷新远程镜像列表
func (c *Client) RefreshRemoteImages(server string) error {
	// 对 images.linuxcontainers.org 使用 simplestreams 协议
	path := "/images/refresh"
	body := map[string]interface{}{
		"server": server,
	}
	return c.Post(path, body, nil)
}

// ImportImage 从远程拉取镜像到本地（异步）
// Incus API 要求镜像来源为嵌套的 source 对象。
func (c *Client) ImportImage(req model.ImportImageRequest) (string, error) {
	protocol := "simplestreams"
	body := map[string]interface{}{
		"source": map[string]interface{}{
			"type":     "image",
			"alias":    req.Alias,
			"server":   req.Server,
			"protocol": protocol,
		},
	}

	var opResp IncusOperationResponse
	if err := c.Post("/images", body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// ImportImageUpload 上传本地镜像文件到 Incus（异步），返回操作 URL。
// body 为合并格式的镜像 tarball 流。
func (c *Client) ImportImageUpload(r io.Reader) (string, error) {
	headers := map[string]string{
		"Content-Type":   "application/octet-stream",
		"X-Incus-Public": "false",
	}
	var opResp IncusOperationResponse
	if err := c.PostWithHeaders("/images", r, headers, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// DeleteImage 删除本地镜像
func (c *Client) DeleteImage(fingerprint string) error {
	return c.Delete("/images/"+url.PathEscape(fingerprint), nil)
}

// ExportImage 导出镜像为 tar 文件（返回流式响应）
func (c *Client) ExportImage(fingerprint, format string) (*http.Response, error) {
	path := "/images/" + url.PathEscape(fingerprint) + "/export"
	if format != "" {
		path += "?format=" + url.QueryEscape(format)
	}
	return c.GetRaw(path)
}
