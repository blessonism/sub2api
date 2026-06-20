package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TokenUsagePolicyHandler struct {
	svc *service.TokenUsageAutoPolicyService
}

func NewTokenUsagePolicyHandler(svc *service.TokenUsageAutoPolicyService) *TokenUsagePolicyHandler {
	return &TokenUsagePolicyHandler{svc: svc}
}

func (h *TokenUsagePolicyHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters := service.TokenUsageAutoPolicyListFilters{}
	if enabled := c.Query("enabled"); enabled != "" {
		v, err := strconv.ParseBool(enabled)
		if err != nil {
			response.BadRequest(c, "invalid enabled")
			return
		}
		filters.Enabled = &v
	}
	if targetGroup := c.Query("target_group_id"); targetGroup != "" {
		v, err := strconv.ParseInt(targetGroup, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid target_group_id")
			return
		}
		filters.TargetGroupID = v
	}
	items, pageResult, err := h.svc.ListPolicies(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *TokenUsagePolicyHandler) Create(c *gin.Context) {
	var req service.TokenUsageAutoPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	policy, err := h.svc.CreatePolicy(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *TokenUsagePolicyHandler) Get(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	policy, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *TokenUsagePolicyHandler) Update(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	var req service.TokenUsageAutoPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	policy, err := h.svc.UpdatePolicy(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *TokenUsagePolicyHandler) Delete(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	if err := h.svc.DeletePolicy(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *TokenUsagePolicyHandler) Preview(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	preview, err := h.svc.PreviewPolicy(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

func (h *TokenUsagePolicyHandler) Run(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	run, err := h.svc.RunPolicy(c.Request.Context(), id, service.TokenUsagePolicyRunTypeManual)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func (h *TokenUsagePolicyHandler) ListRuns(c *gin.Context) {
	id, ok := parseTokenUsagePolicyID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	runs, pageResult, err := h.svc.ListRuns(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, runs, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func parseTokenUsagePolicyID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid policy id")
		return 0, false
	}
	return id, true
}
