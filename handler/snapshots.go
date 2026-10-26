package handler

import (
	"net/http"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// SnapshotHandler 快照处理器
type SnapshotHandler struct {
	client *incus.Client
}

// NewSnapshotHandler 创建快照处理器
func NewSnapshotHandler(client *incus.Client) *SnapshotHandler {
	return &SnapshotHandler{client: client}
}

// ListSnapshots GET /api/instances/:name/snapshots
func (h *SnapshotHandler) ListSnapshots(c *gin.Context) {
	instanceName := c.Param("name")
	snapshots, err := h.client.ListSnapshots(instanceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: snapshots})
}

// CreateSnapshot POST /api/instances/:name/snapshots
func (h *SnapshotHandler) CreateSnapshot(c *gin.Context) {
	instanceName := c.Param("name")
	var req model.CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	opURL, err := h.client.CreateSnapshot(instanceName, req)
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

// RestoreSnapshot POST /api/instances/:name/snapshots/:snap/restore
func (h *SnapshotHandler) RestoreSnapshot(c *gin.Context) {
	instanceName := c.Param("name")
	snapshotName := c.Param("snap")

	// body 可为空；仅当显式传 stateful=true 时才连同运行时状态恢复
	var req model.RestoreSnapshotRequest
	_ = c.ShouldBindJSON(&req)

	opURL, err := h.client.RestoreSnapshot(instanceName, snapshotName, req.Stateful)
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

// RenameSnapshot POST /api/instances/:name/snapshots/:snap/rename
func (h *SnapshotHandler) RenameSnapshot(c *gin.Context) {
	instanceName := c.Param("name")
	snapshotName := c.Param("snap")

	var req model.RenameSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: "新快照名称不能为空",
		})
		return
	}

	opURL, err := h.client.RenameSnapshot(instanceName, snapshotName, req.Name)
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

// DeleteSnapshot DELETE /api/instances/:name/snapshots/:snap
func (h *SnapshotHandler) DeleteSnapshot(c *gin.Context) {
	instanceName := c.Param("name")
	snapshotName := c.Param("snap")

	opURL, err := h.client.DeleteSnapshot(instanceName, snapshotName)
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
