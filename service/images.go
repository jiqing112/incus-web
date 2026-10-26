package service

import (
	"incus-web/incus"
	"incus-web/model"
	"io"
	"net/http"
)

// ImageService 镜像业务逻辑
type ImageService struct {
	client *incus.Client
}

// NewImageService 创建镜像服务
func NewImageService(client *incus.Client) *ImageService {
	return &ImageService{client: client}
}

// ListLocalImages 列出本地镜像
func (s *ImageService) ListLocalImages() ([]model.ImageInfo, error) {
	return s.client.ListLocalImages()
}

// ListRemoteImages 列出远程镜像
func (s *ImageService) ListRemoteImages(serverURL string) ([]model.RemoteImageInfo, error) {
	return s.client.ListRemoteImages(serverURL)
}

// RefreshRemoteImages 刷新远程镜像列表
func (s *ImageService) RefreshRemoteImages(server string) error {
	return s.client.RefreshRemoteImages(server)
}

// ImportImage 从远程拉取镜像（异步）
func (s *ImageService) ImportImage(req model.ImportImageRequest) (string, error) {
	return s.client.ImportImage(req)
}

// DeleteImage 删除本地镜像
// UploadImage 上传本地镜像文件（流式转发给 Incus）
func (s *ImageService) UploadImage(r io.Reader) (string, error) {
	return s.client.ImportImageUpload(r)
}

func (s *ImageService) DeleteImage(fingerprint string) error {
	return s.client.DeleteImage(fingerprint)
}

// ExportImage 导出镜像（返回原始响应用于流式转发）
func (s *ImageService) ExportImage(fingerprint, format string) (*http.Response, error) {
	return s.client.ExportImage(fingerprint, format)
}
