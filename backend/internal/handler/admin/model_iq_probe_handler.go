package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelIQProbeHandler 智商探针管理端 handler：配置、结果查询、手动触发、人工改判。
type ModelIQProbeHandler struct {
	svc    *service.ModelIQProbeService
	runner *service.ModelIQProbeRunner
}

// NewModelIQProbeHandler 创建 handler。
func NewModelIQProbeHandler(svc *service.ModelIQProbeService, runner *service.ModelIQProbeRunner) *ModelIQProbeHandler {
	return &ModelIQProbeHandler{svc: svc, runner: runner}
}

// GetConfig 返回探针配置。
func (h *ModelIQProbeHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

type modelIQProbeConfigRequest struct {
	Enabled         bool     `json:"enabled"`
	Visibility      string   `json:"visibility" binding:"omitempty,oneof=admin_only all_users"`
	UserDisplayMode string   `json:"user_display_mode" binding:"omitempty,oneof=real always_green"`
	ShowExclusion   bool     `json:"show_exclusion"`
	Question        string   `json:"question"`
	ExpectedAnswer  string   `json:"expected_answer" binding:"required,max=200"`
	IntervalMinutes int      `json:"interval_minutes" binding:"required,min=5,max=10080"`
	TargetMode      string   `json:"target_mode" binding:"required,oneof=account_model model_only"`
	Models          []string `json:"models"`
	AccountIDs      []int64  `json:"account_ids"`
}

// UpdateConfig 保存探针配置并触发调度重排。
func (h *ModelIQProbeHandler) UpdateConfig(c *gin.Context) {
	var req modelIQProbeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrModelIQProbeInvalidConfig.WithCause(err))
		return
	}
	cfg := &service.ModelIQProbeConfig{
		Enabled:         req.Enabled,
		Visibility:      req.Visibility,
		UserDisplayMode: req.UserDisplayMode,
		ShowExclusion:   req.ShowExclusion,
		Question:        req.Question,
		ExpectedAnswer:  req.ExpectedAnswer,
		IntervalMinutes: req.IntervalMinutes,
		TargetMode:      req.TargetMode,
		Models:          req.Models,
		AccountIDs:      req.AccountIDs,
	}
	if err := h.svc.SaveConfig(c.Request.Context(), cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.runner.Reload()
	response.Success(c, cfg)
}

// ListResults 管理端分页查询探测结果。
func (h *ModelIQProbeHandler) ListResults(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters := service.ModelIQProbeListFilters{
		Model:  c.Query("model"),
		Status: c.Query("status"),
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if accountID, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil && accountID > 0 {
			filters.AccountID = accountID
		}
	}
	items, total, err := h.svc.ListResultsAdmin(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// RunProbe 手动触发一轮探测。
func (h *ModelIQProbeHandler) RunProbe(c *gin.Context) {
	state, err := h.runner.StartRun(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

// GetRun 返回共享的后台探测进度，页面刷新或切换实例后仍可读取。
func (h *ModelIQProbeHandler) GetRun(c *gin.Context) {
	state, err := h.runner.GetRunState(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

type modelIQProbeOverrideRequest struct {
	// Status 传 pass/wrong_answer/error 覆盖自动判定；null/缺省清除改判。
	Status *string `json:"status"`
}

// OverrideResult 人工改判单条结果。
func (h *ModelIQProbeHandler) OverrideResult(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid result id")
		return
	}
	var req modelIQProbeOverrideRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, service.ErrModelIQProbeInvalidConfig.WithCause(err))
			return
		}
	}
	if err := h.svc.OverrideStatus(c.Request.Context(), id, req.Status); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}
