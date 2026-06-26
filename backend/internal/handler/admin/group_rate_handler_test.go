package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type captureGroupRateAdminService struct {
	stubAdminService

	groupID int64
	entries []service.GroupRateMultiplierInput
}

func (s *captureGroupRateAdminService) BatchSetGroupRateMultipliers(_ context.Context, groupID int64, entries []service.GroupRateMultiplierInput) error {
	s.groupID = groupID
	s.entries = entries
	return nil
}

func TestGroupHandlerBatchSetGroupRateMultipliersDecodesTriStateFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := &captureGroupRateAdminService{}
	handler := NewGroupHandler(adminSvc, nil, nil)
	router := gin.New()
	router.PUT("/groups/:id/rate-multipliers", handler.BatchSetGroupRateMultipliers)

	body := bytes.NewBufferString(`{"entries":[{"user_id":1,"rate_multiplier":1.25},{"user_id":2,"visible_rate_multiplier":null},{"user_id":3,"visible_rate_multiplier":0.75}]}`)
	req := httptest.NewRequest(http.MethodPut, "/groups/10/rate-multipliers", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(10), adminSvc.groupID)
	require.Len(t, adminSvc.entries, 3)
	require.True(t, adminSvc.entries[0].RateMultiplierSet)
	require.NotNil(t, adminSvc.entries[0].RateMultiplier)
	require.False(t, adminSvc.entries[0].VisibleRateMultiplierSet)
	require.False(t, adminSvc.entries[1].RateMultiplierSet)
	require.True(t, adminSvc.entries[1].VisibleRateMultiplierSet)
	require.Nil(t, adminSvc.entries[1].VisibleRateMultiplier)
	require.False(t, adminSvc.entries[2].RateMultiplierSet)
	require.True(t, adminSvc.entries[2].VisibleRateMultiplierSet)
	require.NotNil(t, adminSvc.entries[2].VisibleRateMultiplier)
}
