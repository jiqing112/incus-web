package handler

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"

	"incus-web/model"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// ImageHandler 镜像处理器
type ImageHandler struct {
	svc *service.ImageService
}

// NewImageHandler 创建镜像处理器
func NewImageHandler(svc *service.ImageService) *ImageHandler {
	return &ImageHandler{svc: svc}
}

// ListImages GET /api/images
func (h *ImageHandler) ListImages(c *gin.Context) {
	remote := c.Query("remote")

	if remote == "1" {
		serverURL := c.Query("server")
		if serverURL == "" {
			serverURL = "https://images.linuxcontainers.org"
		}

		images, err := h.svc.ListRemoteImages(serverURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.ApiResponse{
				Code:  500,
				Error: err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: images})
		return
	}

	images, err := h.svc.ListLocalImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: images})
}

// RefreshImages POST /api/images/refresh
func (h *ImageHandler) RefreshImages(c *gin.Context) {
	var req model.RefreshImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if req.Server == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "server is required",
		})
		return
	}

	if err := h.svc.RefreshRemoteImages(req.Server); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// ImportImage POST /api/images/import
func (h *ImageHandler) ImportImage(c *gin.Context) {
	var req model.ImportImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.svc.ImportImage(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}

	opID, statusURL := BuildOperationID(opURL)

	c.JSON(http.StatusOK, model.ApiResponse{
		Code: 200,
		Data: gin.H{
			"operation_id": opID,
			"status_url":   statusURL,
		},
	})
}

// UploadImage POST /api/images/upload
// 接收浏览器上传的镜像文件（multipart，字段名 file），流式转发给 Incus 导入。
func (h *ImageHandler) UploadImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "file is required: " + err.Error()})
		return
	}
	defer file.Close()

	if header.Size == 0 {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "文件为空"})
		return
	}

	opURL, err := h.svc.UploadImage(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: err.Error()})
		return
	}

	opID, statusURL := BuildOperationID(opURL)
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
		"operation_id": opID,
		"status_url":   statusURL,
		"filename":     header.Filename,
		"size":         header.Size,
	}})
}

// DeleteImage DELETE /api/images/:fingerprint
func (h *ImageHandler) DeleteImage(c *gin.Context) {
	fingerprint := c.Param("fingerprint")
	if err := h.svc.DeleteImage(fingerprint); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// imageExportFormats 导出格式 → 文件扩展名。
// 允许列表同时避免了把任意 query 拼接进 Incus 请求。
var imageExportFormats = map[string]string{
	"":         "tar",
	"tar":      "tar",
	"tar.gz":   "tar.gz",
	"tar.xz":   "tar.xz",
	"tar.bz2":  "tar.bz2",
	"tar.lz4":  "tar.lz4",
	"tar.zst":  "tar.zst",
	"squashfs": "squashfs",
}

// ExportImage POST /api/images/export
func (h *ImageHandler) ExportImage(c *gin.Context) {
	var req model.ExportImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}
	h.streamImageExport(c, req.Fingerprint, req.Format)
}

// ExportImageStream GET /api/images/:fingerprint/export?format=tar.gz
// 浏览器原生下载无法携带 Authorization 头，token 由认证中间件从 query 读取。
func (h *ImageHandler) ExportImageStream(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		format = "tar.gz"
	}
	h.streamImageExport(c, c.Param("fingerprint"), format)
}

func (h *ImageHandler) streamImageExport(c *gin.Context, fingerprint, format string) {
	ext, ok := imageExportFormats[format]
	if !ok {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "不支持的导出格式: " + format,
		})
		return
	}
	if fingerprint == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "fingerprint is required",
		})
		return
	}

	resp, err := h.svc.ExportImage(fingerprint, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	name := fingerprint
	if len(name) > 12 {
		name = name[:12]
	}
	filename := fmt.Sprintf("%s.%s", name, ext)

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("image export interrupted (%s): %v", fingerprint, err)
	}
}
