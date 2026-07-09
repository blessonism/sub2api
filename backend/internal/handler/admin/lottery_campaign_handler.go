package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LotteryCampaignHandler struct {
	svc *service.LotteryCampaignService
}

func NewLotteryCampaignHandler(svc *service.LotteryCampaignService) *LotteryCampaignHandler {
	return &LotteryCampaignHandler{svc: svc}
}

type lotteryCampaignRequest struct {
	Name              string                          `json:"name" binding:"required"`
	Description       string                          `json:"description"`
	RulesText         string                          `json:"rules_text"`
	ParticipationMode string                          `json:"participation_mode" binding:"required"`
	DrawScheduleType  string                          `json:"draw_schedule_type" binding:"required"`
	PrizeMode         string                          `json:"prize_mode" binding:"required"`
	EntryMode         string                          `json:"entry_mode" binding:"required"`
	ThresholdTokens   int64                           `json:"threshold_tokens" binding:"required"`
	EntryStepTokens   int64                           `json:"entry_step_tokens"`
	MaxEntriesPerUser int                             `json:"max_entries_per_user"`
	StartAt           string                          `json:"start_at" binding:"required"`
	EndAt             string                          `json:"end_at" binding:"required"`
	DrawAt            *string                         `json:"draw_at"`
	DailyDrawTime     string                          `json:"daily_draw_time"`
	PrizeTiers        []service.LotteryPrizeTierInput `json:"prize_tiers" binding:"required"`
}

func (h *LotteryCampaignHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *LotteryCampaignHandler) Create(c *gin.Context) {
	var req lotteryCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, ok := req.toInput(c)
	if !ok {
		return
	}
	input.OperatorID = adminSubjectID(c)
	item, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Update(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	var req lotteryCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, ok := req.toInput(c)
	if !ok {
		return
	}
	input.OperatorID = adminSubjectID(c)
	item, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Get(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Publish(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	item, err := h.svc.Publish(c.Request.Context(), id, adminSubjectID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Cancel(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	item, err := h.svc.Cancel(c.Request.Context(), id, adminSubjectID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Feature(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	item, err := h.svc.Feature(c.Request.Context(), id, adminSubjectID(c), timezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *LotteryCampaignHandler) Delete(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *LotteryCampaignHandler) SyncEntries(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	drawDate, ok := parseLotteryDateQuery(c)
	if !ok {
		return
	}
	count, err := h.svc.SyncEntries(c.Request.Context(), id, drawDate, timezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"synced": count})
}

func (h *LotteryCampaignHandler) Draw(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	drawDate, ok := parseLotteryDateQuery(c)
	if !ok {
		return
	}
	batch, err := h.svc.Draw(c.Request.Context(), id, drawDate, "manual", adminSubjectID(c), timezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, batch)
}

func (h *LotteryCampaignHandler) ListBatches(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.ListBatches(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *LotteryCampaignHandler) ListWinners(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	var batchID *int64
	if raw := c.Param("batch_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid batch ID")
			return
		}
		batchID = &parsed
	}
	items, err := h.svc.ListWinners(c.Request.Context(), id, batchID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

type lotteryDesignationsRequest struct {
	Assignments []service.LotteryDesignationInput `json:"assignments"`
}

func (h *LotteryCampaignHandler) GetDesignations(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	drawDate, ok := parseLotteryDateQuery(c)
	if !ok {
		return
	}
	view, err := h.svc.GetDesignations(c.Request.Context(), id, drawDate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *LotteryCampaignHandler) ReplaceDesignations(c *gin.Context) {
	id, ok := parseLotteryID(c)
	if !ok {
		return
	}
	drawDate, ok := parseLotteryDateQuery(c)
	if !ok {
		return
	}
	var req lotteryDesignationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.svc.ReplaceDesignations(c.Request.Context(), id, drawDate, adminSubjectID(c), req.Assignments); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"saved": len(req.Assignments)})
}

func (req lotteryCampaignRequest) toInput(c *gin.Context) (service.LotteryCampaignInput, bool) {
	startAt, ok := parseRequestTime(c, req.StartAt)
	if !ok {
		return service.LotteryCampaignInput{}, false
	}
	endAt, ok := parseRequestTime(c, req.EndAt)
	if !ok {
		return service.LotteryCampaignInput{}, false
	}
	var drawAt *time.Time
	if req.DrawAt != nil && *req.DrawAt != "" {
		parsed, ok := parseRequestTime(c, *req.DrawAt)
		if !ok {
			return service.LotteryCampaignInput{}, false
		}
		drawAt = &parsed
	}
	return service.LotteryCampaignInput{
		Name:              req.Name,
		Description:       req.Description,
		RulesText:         req.RulesText,
		ParticipationMode: req.ParticipationMode,
		DrawScheduleType:  req.DrawScheduleType,
		PrizeMode:         req.PrizeMode,
		EntryMode:         req.EntryMode,
		ThresholdTokens:   req.ThresholdTokens,
		EntryStepTokens:   req.EntryStepTokens,
		MaxEntriesPerUser: req.MaxEntriesPerUser,
		StartAt:           startAt,
		EndAt:             endAt,
		DrawAt:            drawAt,
		DailyDrawTime:     req.DailyDrawTime,
		PrizeTiers:        req.PrizeTiers,
	}, true
}

func parseLotteryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid lottery campaign ID")
		return 0, false
	}
	return id, true
}

func parseLotteryDateQuery(c *gin.Context) (time.Time, bool) {
	raw := c.DefaultQuery("date", timezone.Now().Format("2006-01-02"))
	parsed, err := time.ParseInLocation("2006-01-02", raw, timezone.Location())
	if err != nil {
		response.BadRequest(c, "Invalid date")
		return time.Time{}, false
	}
	return parsed, true
}
