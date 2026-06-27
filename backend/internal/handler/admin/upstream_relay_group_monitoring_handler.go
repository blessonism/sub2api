package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UpstreamRelayGroupMonitoringHandler struct {
	svc *service.UpstreamRelayGroupMonitoringService
}

func NewUpstreamRelayGroupMonitoringHandler(svc *service.UpstreamRelayGroupMonitoringService) *UpstreamRelayGroupMonitoringHandler {
	return &UpstreamRelayGroupMonitoringHandler{svc: svc}
}

func (h *UpstreamRelayGroupMonitoringHandler) ListConnectors(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, pageResult, err := h.svc.ListConnectors(c.Request.Context(), page, pageSize, service.UpstreamRelayConnectorListFilters{
		Status: c.Query("status"),
		Search: c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *UpstreamRelayGroupMonitoringHandler) CreateConnector(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.UpstreamRelayConnectorInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	connector, err := h.svc.CreateConnector(c.Request.Context(), req, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, connector)
}

func (h *UpstreamRelayGroupMonitoringHandler) UpdateConnector(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	var req service.UpstreamRelayConnectorInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	connector, err := h.svc.UpdateConnector(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, connector)
}

func (h *UpstreamRelayGroupMonitoringHandler) DeleteConnector(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteConnector(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *UpstreamRelayGroupMonitoringHandler) SyncConnector(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	snapshots, err := h.svc.SyncConnector(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshots)
}

func (h *UpstreamRelayGroupMonitoringHandler) ListSnapshots(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	snapshots, err := h.svc.ListSnapshots(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshots)
}

func (h *UpstreamRelayGroupMonitoringHandler) ListCandidates(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters := service.UpstreamRelayCandidateListFilters{}
	if connectorID := c.Query("connector_id"); connectorID != "" {
		v, err := strconv.ParseInt(connectorID, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid connector_id")
			return
		}
		filters.ConnectorID = v
	}
	if enabled := c.Query("enabled"); enabled != "" {
		v, err := strconv.ParseBool(enabled)
		if err != nil {
			response.BadRequest(c, "invalid enabled")
			return
		}
		filters.Enabled = &v
	}
	items, pageResult, err := h.svc.ListCandidates(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *UpstreamRelayGroupMonitoringHandler) CreateCandidate(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.UpstreamRelayCandidateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	candidate, err := h.svc.CreateCandidate(c.Request.Context(), req, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, candidate)
}

func (h *UpstreamRelayGroupMonitoringHandler) UpdateCandidate(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	var req service.UpstreamRelayCandidateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	candidate, err := h.svc.UpdateCandidate(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, candidate)
}

func (h *UpstreamRelayGroupMonitoringHandler) DeleteCandidate(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteCandidate(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *UpstreamRelayGroupMonitoringHandler) ProbeCandidate(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.ProbeCandidate(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamRelayGroupMonitoringHandler) GenerateRecommendations(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	run, err := h.svc.GenerateRecommendations(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func (h *UpstreamRelayGroupMonitoringHandler) ListRecommendationRuns(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	runs, pageResult, err := h.svc.ListRecommendationRuns(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, runs, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *UpstreamRelayGroupMonitoringHandler) GetRecommendationRun(c *gin.Context) {
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	run, err := h.svc.GetRecommendationRun(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func (h *UpstreamRelayGroupMonitoringHandler) ApplyRecommendationRun(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseRelayID(c, "id")
	if !ok {
		return
	}
	run, err := h.svc.ApplyRecommendationRun(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func parseRelayID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return id, true
}
