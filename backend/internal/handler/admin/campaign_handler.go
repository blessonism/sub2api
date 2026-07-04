package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type CampaignHandler struct {
	svc *service.CampaignService
}

func NewCampaignHandler(svc *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{svc: svc}
}

type campaignCreateRequest struct {
	Name                     string   `json:"name" binding:"required"`
	Description              string   `json:"description"`
	CoverURL                 string   `json:"cover_url"`
	RulesText                string   `json:"rules_text"`
	WarmupStartAt            *string  `json:"warmup_start_at"`
	StartAt                  string   `json:"start_at" binding:"required"`
	EndAt                    string   `json:"end_at" binding:"required"`
	AuditStartAt             *string  `json:"audit_start_at"`
	AuditEndAt               *string  `json:"audit_end_at"`
	PublicityStartAt         *string  `json:"publicity_start_at"`
	PublicityEndAt           *string  `json:"publicity_end_at"`
	PayoutDueAt              *string  `json:"payout_due_at"`
	InitialBonusCents        int64    `json:"initial_bonus_cents"`
	RechargeThresholdCents   int64    `json:"recharge_threshold_cents"`
	AllowAccumulatedRecharge *bool    `json:"allow_accumulated_recharge"`
	PoolInjectionRate        *float64 `json:"pool_injection_rate"`
	RankPoolRatio            *float64 `json:"rank_pool_ratio"`
	ContributionPoolRatio    *float64 `json:"contribution_pool_ratio"`
	RankRewardCount          int      `json:"rank_reward_count"`
	RankWeights              []int64  `json:"rank_weights"`
	MinPayoutAmountCents     int64    `json:"min_payout_amount_cents"`
}

type campaignConfigVersionRequest struct {
	VersionScope             string   `json:"version_scope" binding:"required"`
	EffectiveAt              string   `json:"effective_at" binding:"required"`
	RechargeThresholdCents   *int64   `json:"recharge_threshold_cents"`
	PoolInjectionRate        *float64 `json:"pool_injection_rate"`
	AllowAccumulatedRecharge *bool    `json:"allow_accumulated_recharge"`
	ChangeReason             string   `json:"change_reason"`
}

type campaignPoolAdjustmentRequest struct {
	AdjustmentType string `json:"adjustment_type" binding:"required"`
	AmountCents    int64  `json:"amount_cents" binding:"required"`
	Reason         string `json:"reason"`
}

func (h *CampaignHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.ListCampaigns(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *CampaignHandler) Create(c *gin.Context) {
	var req campaignCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, ok := req.toServiceInput(c)
	if !ok {
		return
	}
	campaign, version, err := h.svc.CreateCampaign(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"campaign": campaign, "config_version": version})
}

func (h *CampaignHandler) Get(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	campaign, err := h.svc.GetCampaign(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *CampaignHandler) Delete(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	result, err := h.svc.DeleteCampaign(c.Request.Context(), id, adminSubjectID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *CampaignHandler) Copy(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	campaign, version, err := h.svc.CopyCampaign(c.Request.Context(), id, adminSubjectID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"campaign": campaign, "config_version": version})
}

func (h *CampaignHandler) Publish(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	operatorID := adminSubjectID(c)
	campaign, err := h.svc.PublishCampaign(c.Request.Context(), id, operatorID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *CampaignHandler) CreateConfigVersion(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	var req campaignConfigVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	effectiveAt, ok := parseRequestTime(c, req.EffectiveAt)
	if !ok {
		return
	}
	input := service.CampaignConfigVersionInput{
		VersionScope:             strings.TrimSpace(req.VersionScope),
		EffectiveAt:              effectiveAt,
		RechargeThresholdCents:   req.RechargeThresholdCents,
		AllowAccumulatedRecharge: req.AllowAccumulatedRecharge,
		ChangeReason:             req.ChangeReason,
		OperatorID:               adminSubjectID(c),
	}
	if req.PoolInjectionRate != nil {
		rate := decimal.NewFromFloat(*req.PoolInjectionRate)
		input.PoolInjectionRate = &rate
	}
	version, err := h.svc.CreateConfigVersion(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, version)
}

func (h *CampaignHandler) AddPoolAdjustment(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	var req campaignPoolAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	err := h.svc.AddPoolAdjustment(c.Request.Context(), id, service.CampaignPoolAdjustmentInput{
		AdjustmentType: strings.TrimSpace(req.AdjustmentType),
		AmountCents:    req.AmountCents,
		Reason:         req.Reason,
		OperatorID:     adminSubjectID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *CampaignHandler) Pool(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	pool, err := h.svc.PoolSummary(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

func (h *CampaignHandler) Leaderboard(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	rows, err := h.svc.Leaderboard(c.Request.Context(), id, 200)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": rows})
}

func (h *CampaignHandler) Freeze(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	if err := h.svc.FreezeLeaderboard(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *CampaignHandler) Recalculate(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	status := c.DefaultQuery("status", service.CampaignCalculationPreview)
	summary, err := h.svc.RecalculateRewards(c.Request.Context(), id, status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *CampaignHandler) FinalRewardResults(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	summary, err := h.svc.GetFinalRewardResults(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *CampaignHandler) Payout(c *gin.Context) {
	id, ok := parseAdminCampaignID(c)
	if !ok {
		return
	}
	batch, err := h.svc.Payout(c.Request.Context(), id, adminSubjectID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, batch)
}

func (req campaignCreateRequest) toServiceInput(c *gin.Context) (service.CampaignCreateInput, bool) {
	startAt, ok := parseRequestTime(c, req.StartAt)
	if !ok {
		return service.CampaignCreateInput{}, false
	}
	endAt, ok := parseRequestTime(c, req.EndAt)
	if !ok {
		return service.CampaignCreateInput{}, false
	}
	allowAccumulated := true
	if req.AllowAccumulatedRecharge != nil {
		allowAccumulated = *req.AllowAccumulatedRecharge
	}
	input := service.CampaignCreateInput{
		Name:                     req.Name,
		Description:              req.Description,
		CoverURL:                 req.CoverURL,
		RulesText:                req.RulesText,
		StartAt:                  startAt,
		EndAt:                    endAt,
		InitialBonusCents:        req.InitialBonusCents,
		RechargeThresholdCents:   req.RechargeThresholdCents,
		AllowAccumulatedRecharge: allowAccumulated,
		RankRewardCount:          req.RankRewardCount,
		RankWeights:              req.RankWeights,
		MinPayoutAmountCents:     req.MinPayoutAmountCents,
		OperatorID:               adminSubjectID(c),
	}
	input.WarmupStartAt = parseOptionalRequestTime(c, req.WarmupStartAt)
	input.AuditStartAt = parseOptionalRequestTime(c, req.AuditStartAt)
	input.AuditEndAt = parseOptionalRequestTime(c, req.AuditEndAt)
	input.PublicityStartAt = parseOptionalRequestTime(c, req.PublicityStartAt)
	input.PublicityEndAt = parseOptionalRequestTime(c, req.PublicityEndAt)
	input.PayoutDueAt = parseOptionalRequestTime(c, req.PayoutDueAt)
	if req.PoolInjectionRate != nil {
		input.PoolInjectionRate = decimal.NewFromFloat(*req.PoolInjectionRate)
	}
	if req.RankPoolRatio != nil {
		input.RankPoolRatio = decimal.NewFromFloat(*req.RankPoolRatio)
	}
	if req.ContributionPoolRatio != nil {
		input.ContributionPoolRatio = decimal.NewFromFloat(*req.ContributionPoolRatio)
	}
	return input, true
}

func parseAdminCampaignID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid campaign id")
		return 0, false
	}
	return id, true
}

func parseRequestTime(c *gin.Context, raw string) (time.Time, bool) {
	v, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		response.BadRequest(c, "Invalid time, expected RFC3339")
		return time.Time{}, false
	}
	return v, true
}

func parseOptionalRequestTime(c *gin.Context, raw *string) *time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	v, ok := parseRequestTime(c, *raw)
	if !ok {
		return nil
	}
	return &v
}

func adminSubjectID(c *gin.Context) *int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return nil
	}
	return &subject.UserID
}
