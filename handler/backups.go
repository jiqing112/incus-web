package handler

import (
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// BackupHandler 备份处理器
type BackupHandler struct {
	client *incus.Client
}

// NewBackupHandler 创建备份处理器
func NewBackupHandler(client *incus.Client) *BackupHandler {
	return &BackupHandler{client: client}
}

// ListBackups GET /api/instances/:name/backups
func (h *BackupHandler) ListBackups(c *gin.Context) {
	instanceName := c.Param("name")
	backups, err := h.client.ListBackups(instanceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: backups})
}

// CreateBackup POST /api/instances/:name/backup
func (h *BackupHandler) CreateBackup(c *gin.Context) {
	instanceName := c.Param("name")
	var req model.CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.client.CreateBackup(instanceName, req)
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

// DownloadBackup GET /api/instances/:name/backups/:file/download
func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	instanceName := c.Param("name")
	backupFile := c.Param("file")

	resp, err := h.client.DownloadBackup(instanceName, backupFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": backupFile}))
	// 带上长度，浏览器才能发现下载被截断（否则会当成正常完成）
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		// 响应头已发出，无法再改状态码；记录日志便于排查中断的下载
		log.Printf("backup download interrupted (%s/%s): %v", instanceName, backupFile, err)
	}
}

// DeleteBackup DELETE /api/instances/:name/backups/:file
func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	instanceName := c.Param("name")
	backupFile := c.Param("file")

	if err := h.client.DeleteBackup(instanceName, backupFile); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// ExportInstance POST /api/instances/:name/export
func (h *BackupHandler) ExportInstance(c *gin.Context) {
	instanceName := c.Param("name")
	var req model.ExportInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.client.ExportInstance(instanceName, req.Name)
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
