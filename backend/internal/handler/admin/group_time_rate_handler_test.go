package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type captureGroupTimeRateAdminService struct {
	stubAdminService
	input *service.UpdateGroupInput
}

func (s *captureGroupTimeRateAdminService) UpdateGroup(_ context.Context, _ int64, input *service.UpdateGroupInput) (*service.Group, error) {
	s.input = input
	return &service.Group{ID: 10, TimeRatePriority: *input.TimeRatePriority, TimeRatePeriods: *input.TimeRatePeriods}, nil
}

func TestGroupHandlerUpdateDecodesTimeRateStrategyWithoutClearingLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := &captureGroupTimeRateAdminService{}
	handler := NewGroupHandler(adminSvc, nil, nil)
	router := gin.New()
	router.PUT("/groups/:id", handler.Update)

	body := bytes.NewBufferString(`{"time_rate_priority":"user_first","time_rate_periods":[{"start_time":"22:00","end_time":"24:00","rate_multiplier":0.02,"visible_rate_multiplier":0.2,"enabled":true}]}`)
	req := httptest.NewRequest(http.MethodPut, "/groups/10", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.input)
	require.Equal(t, service.TimeRatePriorityUserFirst, *adminSvc.input.TimeRatePriority)
	require.Len(t, *adminSvc.input.TimeRatePeriods, 1)
	require.False(t, adminSvc.input.DailyLimitUSDSet)
	require.False(t, adminSvc.input.WeeklyLimitUSDSet)
	require.False(t, adminSvc.input.MonthlyLimitUSDSet)

	var payload struct {
		Data struct {
			TimeRatePriority string                        `json:"time_rate_priority"`
			TimeRatePeriods  []service.GroupTimeRatePeriod `json:"time_rate_periods"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, service.TimeRatePriorityUserFirst, payload.Data.TimeRatePriority)
	require.Len(t, payload.Data.TimeRatePeriods, 1)
}
