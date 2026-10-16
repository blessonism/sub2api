//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type displayOverridesHandlerRepo struct {
	service.ChannelMonitorRepository
	stored  *service.ChannelMonitor
	creates int
}

func (r *displayOverridesHandlerRepo) Create(_ context.Context, m *service.ChannelMonitor) error {
	r.creates++
	m.ID = 42
	copy := *m
	r.stored = &copy
	return nil
}
func (r *displayOverridesHandlerRepo) GetByID(context.Context, int64) (*service.ChannelMonitor, error) {
	copy := *r.stored
	return &copy, nil
}
func (r *displayOverridesHandlerRepo) Update(_ context.Context, m *service.ChannelMonitor) error {
	copy := *m
	r.stored = &copy
	return nil
}

func TestChannelMonitorHandlerDisplayOverridesLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &displayOverridesHandlerRepo{}
	h := NewChannelMonitorHandler(service.NewChannelMonitorService(repo, duplicateChannelMonitorHandlerEncryptor{}))
	router := gin.New()
	router.POST("/monitors", h.Create)
	router.PUT("/monitors/:id", h.Update)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	base := `"name":"Monitor","provider":"openai","api_mode":"responses","endpoint":"https://93.184.216.34","api_key":"secret","primary_model":"gpt-test","interval_seconds":60`
	bad := request(http.MethodPost, "/monitors", `{`+base+`,"display_overrides":{"cache_rate_24h":1.01}}`)
	require.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())
	require.Zero(t, repo.creates)
	good := request(http.MethodPost, "/monitors", `{`+base+`,"display_overrides":{"visible_rate_multiplier":0,"cache_rate_24h":0.625,"output_tps_5m":25,"primary_latency_ms":850,"availability_7d":90,"availability_15d":95,"availability_30d":99}}`)
	require.Equal(t, http.StatusCreated, good.Code, good.Body.String())
	require.NotContains(t, good.Body.String(), `"api_key":"secret"`)
	require.Contains(t, good.Body.String(), `"visible_rate_multiplier":0`)
	var payload struct {
		Data struct {
			DisplayOverrides domain.ChannelMonitorDisplayOverrides `json:"display_overrides"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(good.Body.Bytes(), &payload))
	original := payload.Data.DisplayOverrides
	require.Equal(t, repo.stored.DisplayOverrides, original)
	for _, body := range []string{`{}`, `{"display_overrides":null}`} {
		updated := request(http.MethodPut, "/monitors/42", body)
		require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
		require.Equal(t, original, repo.stored.DisplayOverrides)
	}
	invalid := request(http.MethodPut, "/monitors/42", `{"display_overrides":{"availability_15d":101}}`)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	require.Equal(t, original, repo.stored.DisplayOverrides)
	cleared := request(http.MethodPut, "/monitors/42", `{"display_overrides":{}}`)
	require.Equal(t, http.StatusOK, cleared.Code, cleared.Body.String())
	require.Equal(t, domain.ChannelMonitorDisplayOverrides{}, repo.stored.DisplayOverrides)
}
