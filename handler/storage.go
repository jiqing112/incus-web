package handler

import (
	"net/http"

	"incus-web/model"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// StorageHandler 存储处理器
type StorageHandler struct {
	svc *service.StorageService
}

// NewStorageHandler 创建存储处理器
func NewStorageHandler(svc *service.StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

// ListStoragePools GET /api/storage-pools
func (h *StorageHandler) ListStoragePools(c *gin.Context) {
	pools, err := h.svc.ListStoragePools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: pools})
}

// GetStoragePool GET /api/storage-pools/:name
func (h *StorageHandler) GetStoragePool(c *gin.Context) {
	name := c.Param("name")
	pool, err := h.svc.GetStoragePool(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: pool})
}

// CreateStoragePool POST /api/storage-pools
func (h *StorageHandler) CreateStoragePool(c *gin.Context) {
	var req model.CreateStoragePoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.CreateStoragePool(req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// DeleteStoragePool DELETE /api/storage-pools/:name
func (h *StorageHandler) DeleteStoragePool(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteStoragePool(name); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// ListStorageVolumes GET /api/storage-pools/:name/volumes
func (h *StorageHandler) ListStorageVolumes(c *gin.Context) {
	poolName := c.Param("name")
	volumes, err := h.svc.ListStorageVolumes(poolName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: volumes})
}

// CreateStorageVolume POST /api/storage-pools/:name/volumes
func (h *StorageHandler) CreateStorageVolume(c *gin.Context) {
	poolName := c.Param("name")
	var req model.CreateStorageVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.CreateStorageVolume(poolName, req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}
