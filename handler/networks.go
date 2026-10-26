package handler

import (
	"net/http"

	"incus-web/model"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// NetworkHandler 网络处理器
type NetworkHandler struct {
	svc *service.NetworkService
}

// NewNetworkHandler 创建网络处理器
func NewNetworkHandler(svc *service.NetworkService) *NetworkHandler {
	return &NetworkHandler{svc: svc}
}

// ListNetworks GET /api/networks
func (h *NetworkHandler) ListNetworks(c *gin.Context) {
	networks, err := h.svc.ListNetworks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: networks})
}

// GetNetwork GET /api/networks/:name
func (h *NetworkHandler) GetNetwork(c *gin.Context) {
	name := c.Param("name")
	network, err := h.svc.GetNetwork(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: network})
}

// CreateNetwork POST /api/networks
func (h *NetworkHandler) CreateNetwork(c *gin.Context) {
	var req model.CreateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.CreateNetwork(req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// UpdateNetwork PUT /api/networks/:name
func (h *NetworkHandler) UpdateNetwork(c *gin.Context) {
	name := c.Param("name")
	var req model.UpdateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.UpdateNetwork(name, req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// DeleteNetwork DELETE /api/networks/:name
func (h *NetworkHandler) DeleteNetwork(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteNetwork(name); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}
