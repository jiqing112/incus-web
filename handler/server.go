package handler

import (
	"net/http"
	"strings"
	"time"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// ServerHandler 服务器信息处理器
type ServerHandler struct {
	client *incus.Client
}

// NewServerHandler 创建服务器信息处理器
func NewServerHandler(client *incus.Client) *ServerHandler {
	return &ServerHandler{client: client}
}

// GetServerInfo GET /api/server/info
func (h *ServerHandler) GetServerInfo(c *gin.Context) {
	info, err := h.client.GetServerInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: info})
}

// GetServerResources GET /api/server/resources
func (h *ServerHandler) GetServerResources(c *gin.Context) {
	resources, err := h.client.GetServerResources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: resources})
}

// ServerFeatures 宿主机可选能力探测
type ServerFeatures struct {
	// CRIU 容器检查点/恢复工具是否可用（容器 stateful 快照与在线迁移依赖它）
	CRIU        bool   `json:"criu"`
	CRIUVersion string `json:"criu_version,omitempty"`
	// ContainerStateful 容器能否创建 stateful 快照（= CRIU 可用）
	ContainerStateful bool `json:"container_stateful"`
	// VMStateful 虚拟机 stateful 快照由 QEMU 原生支持，无需 CRIU
	VMStateful bool `json:"vm_stateful"`
}

// GetServerFeatures GET /api/server/features
//
// 用途：容器创建 stateful（运行时）快照依赖宿主机 CRIU；缺失时 Incus 不会快速报错，
// 而是长时间挂起，因此前端需要提前探测并把该选项禁用/标注。
func (h *ServerHandler) GetServerFeatures(c *gin.Context) {
	out, code := runCmd(5*time.Second, "criu", "--version")
	feat := ServerFeatures{VMStateful: true}
	if code == 0 {
		feat.CRIU = true
		feat.CRIUVersion = firstLine(out)
		feat.ContainerStateful = true
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: feat})
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
