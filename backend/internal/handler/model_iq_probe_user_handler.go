package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelIQProbeUserHandler 智商探针用户端只读 handler：渠道状态页按模型汇总。
type ModelIQProbeUserHandler struct {
	svc *service.ModelIQProbeService
}

// NewModelIQProbeUserHandler 创建 handler。
func NewModelIQProbeUserHandler(svc *service.ModelIQProbeService) *ModelIQProbeUserHandler {
	return &ModelIQProbeUserHandler{svc: svc}
}

// GetSummary 返回按模型汇总的最近探针结果（禁用/无数据时 items 为空）。
func (h *ModelIQProbeUserHandler) GetSummary(c *gin.Context) {
	items, err := h.svc.UserSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
