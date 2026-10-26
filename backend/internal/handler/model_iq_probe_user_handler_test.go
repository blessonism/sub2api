package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func (r *iqProbeHandlerResults) ListSummaryModels(context.Context, []int64) ([]string, error) {
	return []string{"gpt-5.4"}, nil
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
