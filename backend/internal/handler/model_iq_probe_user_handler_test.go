package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type iqProbeHandlerSettings struct {
	service.SettingRepository
	raw string
}

func (s *iqProbeHandlerSettings) GetValue(context.Context, string) (string, error) { return s.raw, nil }

type iqProbeHandlerResults struct {
	service.ModelIQProbeRepository
	calls int
}

func (r *iqProbeHandlerResults) ListUserHistory(_ context.Context, model string, ids []int64, _ time.Time, status string, page, size int) ([]*service.ModelIQProbeResult, service.ModelIQProbeHistoryCounts, int64, error) {
	r.calls++
	if model != "gpt-5.4" || len(ids) != 1 || ids[0] != 1 || page != 1 || size != 20 {
		return nil, service.ModelIQProbeHistoryCounts{}, 0, nil
	}
	if status == "pass" {
		return nil, service.ModelIQProbeHistoryCounts{WrongAnswer: 1, Error: 1}, 0, nil
	}
	return []*service.ModelIQProbeResult{{ID: 1, Model: model, Status: "wrong_answer", ActualAnswer: "20", AccountName: "private@example.com"}, {ID: 2, Status: "error", ErrorMessage: "secret upstream error"}}, service.ModelIQProbeHistoryCounts{WrongAnswer: 1, Error: 1}, 2, nil
}

func (r *iqProbeHandlerResults) ListSummaryModels(context.Context, []int64) ([]string, error) {
	return []string{"gpt-5.4"}, nil
}

func TestModelIQProbeHistoryVisibilityAndProjection(t *testing.T) {
	for _, tc := range []struct {
		name, role, visibility, display, filter string
		wantItems                               int
		wantStatus                              string
		wantCalls                               int
	}{
		{"private history", "user", "admin_only", "real", "", 0, "", 0},
		{"real history", "user", "all_users", "real", "", 2, "wrong_answer", 1},
		{"green history", "user", "all_users", "always_green", "", 2, "pass", 1},
		{"green hides filtered wrong", "user", "all_users", "always_green", "wrong_answer", 0, "", 1},
		{"admin sees real history", "admin", "admin_only", "always_green", "", 2, "wrong_answer", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := service.DefaultModelIQProbeConfig()
			cfg.Enabled = true
			cfg.Models = []string{"gpt-5.4"}
			cfg.Visibility = tc.visibility
			cfg.UserDisplayMode = tc.display
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			repo := &iqProbeHandlerResults{}
			svc := service.NewModelIQProbeService(repo, service.NewSettingService(&iqProbeHandlerSettings{raw: string(raw)}, nil), &iqProbeHandlerAccounts{}, nil)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/?model=gpt-5.4&status="+tc.filter, nil)
			c.Set(string(middleware.ContextKeyUserRole), tc.role)
			NewModelIQProbeUserHandler(svc).GetHistory(c)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tc.wantCalls, repo.calls)
			var envelope struct {
				Data service.ModelIQProbeUserHistory `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			require.Len(t, envelope.Data.Items, tc.wantItems)
			require.NotContains(t, rec.Body.String(), "private@example.com")
			require.NotContains(t, rec.Body.String(), "secret upstream error")
			if tc.wantItems > 0 {
				require.Equal(t, tc.wantStatus, envelope.Data.Items[0].Status)
			}
			if tc.display == "always_green" && tc.role == "user" {
				require.EqualValues(t, 2, envelope.Data.Counts.Pass)
				require.Zero(t, envelope.Data.Counts.WrongAnswer)
				require.NotContains(t, rec.Body.String(), `"answer"`)
			}
		})
	}
}

func TestModelIQProbeHistoryRejectsInvalidFilters(t *testing.T) {
	for _, query := range []string{"", "model=x&hours=1", "model=x&page=-1", "model=x&status=other"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
		NewModelIQProbeUserHandler(nil).GetHistory(c)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestModelIQProbeHistoryExclusionMarker(t *testing.T) {
	for _, tc := range []struct {
		name    string
		show    bool
		display string
		want    bool
	}{
		{"enabled", true, "real", true},
		{"disabled", false, "real", false},
		{"projected like summary", true, "always_green", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := service.DefaultModelIQProbeConfig()
			cfg.Enabled, cfg.ShowExclusion = true, tc.show
			cfg.Models, cfg.Visibility, cfg.UserDisplayMode = []string{"gpt-5.4"}, "all_users", tc.display
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			svc := service.NewModelIQProbeService(&iqProbeHandlerResults{}, service.NewSettingService(&iqProbeHandlerSettings{raw: string(raw)}, nil), &iqProbeHandlerAccounts{}, nil)
			result, err := svc.UserHistory(context.Background(), false, "gpt-5.4", 168, 1, "")
			require.NoError(t, err)
			require.Len(t, result.Items, 2)
			require.Equal(t, tc.want, result.Items[0].MarkedForExclusion)
			require.False(t, result.Items[1].MarkedForExclusion)
		})
	}
}
func (r *iqProbeHandlerResults) ListForSummary(context.Context, []service.ModelIQProbeTargetRef, int) ([]*service.ModelIQProbeResult, error) {
	r.calls++
	return []*service.ModelIQProbeResult{{Model: "gpt-5.4", Status: service.ModelIQProbeStatusWrongAnswer, ActualAnswer: "20"}}, nil
}

type iqProbeHandlerAccounts struct{ service.AccountRepository }

func (r *iqProbeHandlerAccounts) ListSchedulable(context.Context) ([]service.Account, error) {
	return []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true}}, nil
}
func TestModelIQProbeUserHandlerUsesAuthenticatedRole(t *testing.T) {
	for _, tc := range []struct {
		name, role, visibility, display string
		wantItems                       int
		wantStatus                      string
	}{
		{"user cannot bypass visibility with query", "user", "admin_only", "real", 0, ""},
		{"missing role is private", "", "admin_only", "real", 0, ""},
		{"administrator sees real results", "admin", "admin_only", "always_green", 1, "wrong_answer"},
		{"public user gets projected results", "user", "all_users", "always_green", 1, "pass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := service.DefaultModelIQProbeConfig()
			cfg.Enabled = true
			cfg.Visibility = tc.visibility
			cfg.UserDisplayMode = tc.display
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			repo := &iqProbeHandlerResults{}
			svc := service.NewModelIQProbeService(repo, service.NewSettingService(&iqProbeHandlerSettings{raw: string(raw)}, nil), &iqProbeHandlerAccounts{}, nil)
			handler := NewModelIQProbeUserHandler(svc)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel-monitors/iq-probe?is_admin=true", nil)
			if tc.role != "" {
				c.Set(string(middleware.ContextKeyUserRole), tc.role)
			}
			handler.GetSummary(c)
			require.Equal(t, 200, rec.Code)
			var envelope struct {
				Data struct {
					Items []service.ModelIQProbeUserSummary `json:"items"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			require.Len(t, envelope.Data.Items, tc.wantItems)
			if tc.wantItems == 0 {
				require.Zero(t, repo.calls)
			} else {
				require.Equal(t, tc.wantStatus, envelope.Data.Items[0].Latest.Status)
			}
		})
	}
}
