package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	svc *service.ConversationCaptureService
}

func NewConversationHandler(svc *service.ConversationCaptureService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

func (h *ConversationHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ConversationHandler) UpdateConfig(c *gin.Context) {
	var req service.ConversationCaptureConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	cfg, err := h.svc.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ConversationHandler) ListSessions(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters, ok := parseConversationFilters(c)
	if !ok {
		return
	}
	items, total, err := h.svc.ListSessions(c.Request.Context(), filters, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *ConversationHandler) GetSession(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ConversationHandler) ListSessionTurns(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	session, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.ListSessionTurns(c.Request.Context(), session.SessionID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *ConversationHandler) GetTurn(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetTurn(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *ConversationHandler) SetSessionExportable(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Exportable bool `json:"exportable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.SetSessionExportable(c.Request.Context(), id, req.Exportable); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"exportable": req.Exportable})
}

func (h *ConversationHandler) SetTurnExportable(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Exportable bool `json:"exportable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.SetTurnExportable(c.Request.Context(), id, req.Exportable); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"exportable": req.Exportable})
}

func (h *ConversationHandler) SetSessionQuality(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req service.ConversationQualityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.SetSessionQuality(c.Request.Context(), id, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"quality_status": req.QualityStatus})
}

func (h *ConversationHandler) SetTurnQuality(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req service.ConversationQualityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.SetTurnQuality(c.Request.Context(), id, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"quality_status": req.QualityStatus})
}

func (h *ConversationHandler) BulkSetQuality(c *gin.Context) {
	var req service.ConversationBulkQualityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.BulkSetQuality(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

func (h *ConversationHandler) MergeSessions(c *gin.Context) {
	var req service.ConversationMergeSessionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.MergeSessions(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"merged": true})
}

func (h *ConversationHandler) SplitSession(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req service.ConversationSplitSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	session, err := h.svc.SplitSession(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

func (h *ConversationHandler) MoveTurn(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	var req service.ConversationMoveTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.svc.MoveTurn(c.Request.Context(), id, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"moved": true})
}

func (h *ConversationHandler) ExportMessagesJSONL(c *gin.Context) {
	var req service.ConversationExportMessagesJSONLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if req.Limit <= 0 {
		req.Limit = 200
	}
	data, err := h.svc.ExportMessagesJSONL(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	filename := "conversation_messages_" + time.Now().Format("20060102_150405") + ".jsonl"
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/x-ndjson; charset=utf-8", data)
}

func (h *ConversationHandler) CreateExportJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return
	}
	var req service.ConversationCreateExportJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	job, err := h.svc.CreateExportJob(c.Request.Context(), req, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *ConversationHandler) ListExportJobs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.ListExportJobs(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *ConversationHandler) GetExportJob(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	job, err := h.svc.GetExportJob(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *ConversationHandler) CreateExportDownloadTicket(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	ticket, err := h.svc.CreateExportDownloadTicket(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *ConversationHandler) DeleteExportJob(c *gin.Context) {
	id, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteExportJob(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func parseConversationFilters(c *gin.Context) (service.ConversationSessionFilters, bool) {
	var filters service.ConversationSessionFilters
	if v := strings.TrimSpace(c.Query("user_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid user_id")
			return filters, false
		}
		filters.UserID = id
	}
	if v := strings.TrimSpace(c.Query("api_key_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid api_key_id")
			return filters, false
		}
		filters.APIKeyID = id
	}
	filters.Model = strings.TrimSpace(c.Query("model"))
	filters.RequestID = strings.TrimSpace(c.Query("request_id"))
	filters.QualityStatus = strings.TrimSpace(c.Query("quality_status"))
	if v := strings.TrimSpace(c.Query("exportable")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			response.BadRequest(c, "invalid exportable")
			return filters, false
		}
		filters.Exportable = &b
	}
	if v := strings.TrimSpace(c.Query("started_at_from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.BadRequest(c, "invalid started_at_from")
			return filters, false
		}
		filters.StartedAtFrom = &t
	}
	if v := strings.TrimSpace(c.Query("started_at_to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.BadRequest(c, "invalid started_at_to")
			return filters, false
		}
		filters.StartedAtTo = &t
	}
	return filters, true
}

func parsePositiveIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}
