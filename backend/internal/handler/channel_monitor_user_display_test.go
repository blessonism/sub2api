//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorUserDisplayNullableContractAndQuota(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	group, rate, cache, tps := int64(7), 0.25, 0.0, 15.2
	for _, populated := range []bool{false, true} {
		view := &service.UserMonitorView{ID: 1, PrimaryStatus: "operational", LatestQuota: &domain.MonitorQuotaSnapshot{Success: true}}
		if populated {
			view.GroupID = &group
			view.PrimaryCheckedAt = &now
			view.VisibleRateMultiplier = &rate
			view.CacheRate24h = &cache
			view.OutputTPS5m = &tps
		}
		body, err := json.Marshal(userMonitorViewToItem(view, false))
		require.NoError(t, err)
		var data map[string]any
		require.NoError(t, json.Unmarshal(body, &data))
		require.NotContains(t, data, "latest_quota")
		for _, field := range []string{"group_id", "primary_checked_at", "visible_rate_multiplier", "cache_rate_24h", "output_tps_5m"} {
			require.Contains(t, data, field)
			if populated {
				require.NotNil(t, data[field])
			} else {
				require.Nil(t, data[field])
			}
		}
		if populated {
			require.Equal(t, 0.0, data["cache_rate_24h"])
			require.Equal(t, 0.25, data["visible_rate_multiplier"])
			require.Equal(t, 15.2, data["output_tps_5m"])
		}
		require.NotNil(t, userMonitorViewToItem(view, true).LatestQuota)
	}
}

type userDisplayMonitorRepo struct {
	service.ChannelMonitorRepository
}

func (userDisplayMonitorRepo) ListEnabled(context.Context) ([]*service.ChannelMonitor, error) {
	return []*service.ChannelMonitor{{ID: 1, Name: "primary", PrimaryModel: "gpt-test", Enabled: true}}, nil
}
func (userDisplayMonitorRepo) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*service.ChannelMonitorLatest, error) {
	return map[int64][]*service.ChannelMonitorLatest{1: {{Model: "gpt-test", EffectiveStatus: service.MonitorStatusOperational, CheckedAt: time.Now()}}}, nil
}
func (userDisplayMonitorRepo) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*service.ChannelMonitorAvailability, error) {
	return nil, nil
}
func (userDisplayMonitorRepo) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*service.ChannelMonitorHistoryEntry, error) {
	return nil, nil
}

type userDisplayEnricherStub struct{ userIDs []int64 }

func (s *userDisplayEnricherStub) Enrich(_ context.Context, userID int64, views []*service.UserMonitorView) {
	s.userIDs = append(s.userIDs, userID)
	rate, tps := .25, 15.2
	views[0].VisibleRateMultiplier = &rate
	views[0].OutputTPS5m = &tps
}
func TestChannelMonitorUserListUsesAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, userID := range []int64{0, 7} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel-monitors?user_id=999", nil)
		if userID > 0 {
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		}
		enrich := &userDisplayEnricherStub{}
		h := &ChannelMonitorUserHandler{monitorService: service.NewChannelMonitorService(userDisplayMonitorRepo{}, nil), displayService: enrich}
		h.List(ctx)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), `"primary_status":"operational"`)
		if userID > 0 {
			require.Equal(t, []int64{7}, enrich.userIDs)
			require.Contains(t, recorder.Body.String(), `"visible_rate_multiplier":0.25`)
			require.Contains(t, recorder.Body.String(), `"output_tps_5m":15.2`, "TPS does not depend on quota or scale visibility flags")
		} else {
			require.Empty(t, enrich.userIDs)
			require.Contains(t, recorder.Body.String(), `"visible_rate_multiplier":null`)
		}
	}
}
