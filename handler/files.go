package handler

import (
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// FileHandler 文件管理处理器
type FileHandler struct {
	client *incus.Client
}

// NewFileHandler 创建文件处理器
func NewFileHandler(client *incus.Client) *FileHandler {
	return &FileHandler{client: client}
}

// ListFiles GET /api/instances/:name/files
func (h *FileHandler) ListFiles(c *gin.Context) {
	instanceName := c.Param("name")
	dirPath := c.Query("path")
	if dirPath == "" {
		dirPath = "/"
	}

	files, err := incus.ExecLS(instanceName, dirPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: files})
}

// UploadFile POST /api/instances/:name/files/upload
func (h *FileHandler) UploadFile(c *gin.Context) {
	instanceName := c.Param("name")

	remotePath := c.PostForm("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "path is required",
		})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "file is required: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// 目录路径（以 / 结尾）拼接上传文件名；否则视为完整目标路径
	if strings.HasSuffix(remotePath, "/") {
		remotePath = remotePath + header.Filename
	}
	// 规范化路径：目标是 Linux 容器内的路径，必须用 POSIX 语义的 path.Clean。
	// 此前用 filepath.Clean，在 Windows 宿主上会把 / 变成 \，
	// 「/root/a.txt」会退化成字面量文件名「root\a.txt」写入容器根目录。
	remotePath = path.Clean(remotePath)
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	if err := incus.FilePush(instanceName, remotePath, file); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// DownloadFile GET /api/instances/:name/files/download
func (h *FileHandler) DownloadFile(c *gin.Context) {
	instanceName := c.Param("name")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "path is required",
		})
		return
	}

	resp, err := incus.FilePull(instanceName, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 远端是 POSIX 路径，统一用 path 包取文件名（Windows 的 filepath 会混淆分隔符）
	fileName := path.Base(filePath)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("file download interrupted (%s:%s): %v", instanceName, filePath, err)
	}
}

// DeleteFile DELETE /api/instances/:name/files
func (h *FileHandler) DeleteFile(c *gin.Context) {
	instanceName := c.Param("name")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "path is required",
		})
		return
	}

	if err := incus.FileDelete(instanceName, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}
