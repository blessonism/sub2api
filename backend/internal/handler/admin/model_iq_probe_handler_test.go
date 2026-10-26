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

func TestModelIQProbeHandlerPersistsDisplayConfiguration(t *testing.T) {
	repo := &settingHandlerRepoStub{values: map[string]string{}}
	svc := service.NewModelIQProbeService(nil, service.NewSettingService(repo, nil), nil, nil)
	runner := service.NewModelIQProbeRunner(svc)
	defer runner.Stop()
	handler := NewModelIQProbeHandler(svc, runner)
	cfg := service.DefaultModelIQProbeConfig()
	cfg.Enabled = true
	cfg.Visibility = service.ModelIQProbeVisibilityAllUsers
	cfg.UserDisplayMode = service.ModelIQProbeDisplayAlwaysGreen
	cfg.ShowExclusion = true
	body, err := json.Marshal(cfg)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-iq-probe/config", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateConfig(c)
	require.Equal(t, http.StatusOK, w.Code)
	stored, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg, stored)

	cfg.Visibility = "invalid"
	body, err = json.Marshal(cfg)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-iq-probe/config", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateConfig(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	stored, err = svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, service.ModelIQProbeVisibilityAllUsers, stored.Visibility)
}
