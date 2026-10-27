package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

// ModelIQProbeUserHandler 智商探针用户端只读 handler：渠道状态页按模型汇总。
type ModelIQProbeUserHandler struct {
	svc *service.ModelIQProbeService
}

func (h *ModelIQProbeUserHandler) GetHistory(c *gin.Context) {
	model := strings.TrimSpace(c.Query("model"))
	hours, err := strconv.Atoi(c.DefaultQuery("hours", "168"))
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	status := c.Query("status")
	if model == "" || len(model) > 255 || err != nil || (hours != 24 && hours != 168 && hours != 720) || pageErr != nil || page < 1 || page > 10000 || (status != "" && status != service.ModelIQProbeStatusPass && status != service.ModelIQProbeStatusWrongAnswer && status != service.ModelIQProbeStatusError) {
		response.BadRequest(c, "invalid history filters")
		return
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	result, err := h.svc.UserHistory(c.Request.Context(), role == service.RoleAdmin, model, hours, page, status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// NewModelIQProbeUserHandler 创建 handler。
func NewModelIQProbeUserHandler(svc *service.ModelIQProbeService) *ModelIQProbeUserHandler {
	return &ModelIQProbeUserHandler{svc: svc}
}

// GetSummary 返回按模型汇总的最近探针结果（禁用/无数据时 items 为空）。
func (h *ModelIQProbeUserHandler) GetSummary(c *gin.Context) {
	role, _ := middleware.GetUserRoleFromContext(c)
	items, err := h.svc.UserSummary(c.Request.Context(), role == service.RoleAdmin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
