package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UpstreamCostCalibrationHandler struct {
	svc *service.UpstreamCostCalibrationService
}

func NewUpstreamCostCalibrationHandler(svc *service.UpstreamCostCalibrationService) *UpstreamCostCalibrationHandler {
	return &UpstreamCostCalibrationHandler{svc: svc}
}

func (h *UpstreamCostCalibrationHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters := service.UpstreamCostCalibrationTaskListFilters{}
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
	items, pageResult, err := h.svc.ListTasks(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *UpstreamCostCalibrationHandler) Create(c *gin.Context) {
	var req service.UpstreamCostCalibrationTaskInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	task, err := h.svc.CreateTask(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *UpstreamCostCalibrationHandler) Get(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	task, err := h.svc.GetTask(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *UpstreamCostCalibrationHandler) Update(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	var req service.UpstreamCostCalibrationTaskInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	task, err := h.svc.UpdateTask(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *UpstreamCostCalibrationHandler) Delete(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTask(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *UpstreamCostCalibrationHandler) Run(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	run, err := h.svc.RunTask(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func (h *UpstreamCostCalibrationHandler) ListRuns(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
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

func (h *UpstreamCostCalibrationHandler) GetRun(c *gin.Context) {
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	runID, ok := parseCalibrationRunID(c)
	if !ok {
		return
	}
	run, err := h.svc.GetRun(c.Request.Context(), id, runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func (h *UpstreamCostCalibrationHandler) ApplyRun(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCalibrationTaskID(c)
	if !ok {
		return
	}
	runID, ok := parseCalibrationRunID(c)
	if !ok {
		return
	}
	run, err := h.svc.ApplyRunSuggestions(c.Request.Context(), id, runID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}

func parseCalibrationTaskID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid calibration task id")
		return 0, false
	}
	return id, true
}

func parseCalibrationRunID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid calibration run id")
		return 0, false
	}
	return id, true
}
