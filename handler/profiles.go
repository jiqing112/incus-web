package handler

import (
	"net/http"

	"incus-web/model"
	"incus-web/service"

	"github.com/gin-gonic/gin"
)

// ProfileHandler 配置文件处理器
type ProfileHandler struct {
	svc *service.ProfileService
}

// NewProfileHandler 创建配置文件处理器
func NewProfileHandler(svc *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// ListProfiles GET /api/profiles
func (h *ProfileHandler) ListProfiles(c *gin.Context) {
	profiles, err := h.svc.ListProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: profiles})
}

// GetProfile GET /api/profiles/:name
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	name := c.Param("name")
	profile, err := h.svc.GetProfile(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: profile})
}

// CreateProfile POST /api/profiles
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	var req model.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.CreateProfile(req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// UpdateProfile PUT /api/profiles/:name
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	name := c.Param("name")
	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{
			Code:  400,
			Error: err.Error(),
		})
		return
	}

	if err := h.svc.UpdateProfile(name, req); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}

// DeleteProfile DELETE /api/profiles/:name
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteProfile(name); err != nil {
		c.JSON(http.StatusInternalServerError, model.ApiResponse{
			Code:  500,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: "ok"})
}
