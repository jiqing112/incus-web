package handler

import (
	"net/http"
	"strings"
	"sync"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// OperationHandler 操作跟踪处理器
type OperationHandler struct {
	client *incus.Client
	ops    sync.Map // 操作状态缓存
}

// NewOperationHandler 创建操作处理器
func NewOperationHandler(client *incus.Client) *OperationHandler {
	return &OperationHandler{client: client}
}

// BuildOperationID 从 Incus 操作 URL 中提取 ID 并构建 /api/operations/:id URL
func BuildOperationID(opURL string) (string, string) {
	// 提取最后一个路径段作为 ID
	id := opURL
	if idx := strings.LastIndex(opURL, "/"); idx >= 0 {
		id = opURL[idx+1:]
	}
	return id, "/api/operations/" + id
}

// TrackOperation 记录异步操作
func (h *OperationHandler) TrackOperation(id string, description string) {
	h.ops.Store(id, model.Operation{
		ID:          id,
		Status:      "pending",
		Description: description,
		StatusURL:   "/api/operations/" + id,
	})
}

// GetOperation GET /api/operations/:id
func (h *OperationHandler) GetOperation(c *gin.Context) {
	id := c.Param("id")

	// 始终以 Incus 的最新状态为准；否则进行中的操作会被首次查询缓存的状态永久覆盖，
	// 导致前端轮询直到超时也看不到 Success/Failure。
	op, err := h.client.GetOperation(id)
	if err == nil {
		h.ops.Store(id, model.Operation{
			ID:          op.ID,
			Status:      op.Status,
			Description: op.Description,
			StatusURL:   "/api/operations/" + op.ID,
		})
		c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: op})
		return
	}

	// Incus 会在操作完成后回收它，此时查询返回 404。若该操作此前被跟踪过，
	// 说明它已结束，但我们没赶上终态 —— 不能直接宣称成功（失败的操作同样会被回收），
	// 因此返回 Unknown，由前端提示「请刷新确认」，而不是误报成功。
	if cached, ok := h.ops.Load(id); ok && strings.Contains(err.Error(), "status 404") {
		cachedOp := cached.(model.Operation)
		if !isTerminalStatus(cachedOp.Status) {
			cachedOp.Status = "Unknown"
			cachedOp.Description = "操作状态已不可查询（Incus 已回收该操作），请刷新列表确认结果"
		}
		c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: cachedOp})
		return
	}

	c.JSON(http.StatusInternalServerError, model.ApiResponse{
		Code:  500,
		Error: err.Error(),
	})
}

// isTerminalStatus 判断操作是否已处于终态
func isTerminalStatus(status string) bool {
	switch strings.ToLower(status) {
	case "success", "failure", "cancelled":
		return true
	}
	return false
}
