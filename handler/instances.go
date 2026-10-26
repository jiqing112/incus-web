package handler

import (
	"net/http"

	"incus-web/model"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// InstanceHandler 实例处理器
type InstanceHandler struct {
	svc *service.InstanceService
}

// NewInstanceHandler 创建实例处理器
func NewInstanceHandler(svc *service.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

// ListInstances GET /api/instances
func (h *InstanceHandler) ListInstances(c *gin.Context) {
	instanceType := c.Query("type")
	status := c.Query("status")

	instances, err := h.svc.ListInstances(instanceType, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: instances})
}

// GetInstance GET /api/instances/:name
func (h *InstanceHandler) GetInstance(c *gin.Context) {
	name := c.Param("name")
	instance, err := h.svc.GetInstance(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: instance})
}

// CreateInstance POST /api/instances
func (h *InstanceHandler) CreateInstance(c *gin.Context) {
	var req model.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.svc.CreateInstance(req)
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

// UpdateInstanceConfig PATCH /api/instances/:name/config
func (h *InstanceHandler) UpdateInstanceConfig(c *gin.Context) {
	name := c.Param("name")

	var req model.UpdateInstanceConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.svc.UpdateInstanceConfig(name, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}

	// 仅改 config 时为同步操作，opURL 为空，前端 pollOperation 会直接判为成功
	opID, statusURL := BuildOperationID(opURL)

	c.JSON(http.StatusOK, model.ApiResponse{
		Code: 200,
		Data: gin.H{
			"operation_id": opID,
			"status_url":   statusURL,
		},
	})
}

// DeleteInstance DELETE /api/instances/:name
func (h *InstanceHandler) DeleteInstance(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	opURL, err := h.svc.DeleteInstance(name, force)
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

// ChangeInstanceState PUT /api/instances/:name/state
func (h *InstanceHandler) ChangeInstanceState(c *gin.Context) {
	name := c.Param("name")

	var req model.StateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[req.Action] {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "invalid action, must be start, stop or restart",
		})
		return
	}

	opURL, err := h.svc.ChangeInstanceState(name, req)
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
