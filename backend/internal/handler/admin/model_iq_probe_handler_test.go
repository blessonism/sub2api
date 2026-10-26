package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

type modelIQProbeRepoStub struct {
	service.ModelIQProbeRepository
	results        map[int64]*service.ModelIQProbeResult
	passAnswer     string
	passOK         bool
	overrideID     int64
	overrideStatus *string
	overrideAnswer *string
}

func (s *modelIQProbeRepoStub) GetByID(_ context.Context, id int64) (*service.ModelIQProbeResult, error) {
	row, ok := s.results[id]
	if !ok {
		return nil, service.ErrModelIQProbeResultNotFound
	}
	return row, nil
}

func (s *modelIQProbeRepoStub) RandomPassAnswer(_ context.Context, _ string, _ int64) (string, bool, error) {
	return s.passAnswer, s.passOK, nil
}

func (s *modelIQProbeRepoStub) OverrideStatus(_ context.Context, id int64, status *string, actualAnswer *string) error {
	s.overrideID = id
	s.overrideStatus = status
	s.overrideAnswer = actualAnswer
	return nil
}

func TestModelIQProbeOverrideAnswerRewriteAndPassSample(t *testing.T) {
	repo := &modelIQProbeRepoStub{
		results:    map[int64]*service.ModelIQProbeResult{7: {ID: 7, Model: "gpt-5.2", Status: service.ModelIQProbeStatusWrongAnswer}},
		passAnswer: "所以最少取出 21 个糖果。",
		passOK:     true,
	}
	svc := service.NewModelIQProbeService(repo, service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{}}, nil), nil, nil)
	runner := service.NewModelIQProbeRunner(svc)
	defer runner.Stop()
	handler := NewModelIQProbeHandler(svc, runner)
	router := gin.New()
	router.POST("/api/v1/admin/model-iq-probe/results/:id/override", handler.OverrideResult)
	router.GET("/api/v1/admin/model-iq-probe/results/:id/pass-answer", handler.GetPassAnswerSample)

	// 改判同时改写回复：trim 后落参。
	body := `{"status":"pass","actual_answer":"  最少 21 个（星形 12 + 圆形 9）。  "}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-iq-probe/results/7/override", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 7, repo.overrideID)
	require.NotNil(t, repo.overrideStatus)
	require.Equal(t, "pass", *repo.overrideStatus)
	require.NotNil(t, repo.overrideAnswer)
	require.Equal(t, "最少 21 个（星形 12 + 圆形 9）。", *repo.overrideAnswer)

	// 空白回复视为不改写；status 缺省 = 清除改判。
	repo.overrideStatus, repo.overrideAnswer = nil, nil
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-iq-probe/results/7/override", strings.NewReader(`{"actual_answer":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, repo.overrideStatus)
	require.Nil(t, repo.overrideAnswer)

	// 通过回复样本端点。
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-iq-probe/results/7/pass-answer", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Data struct {
			Answer string `json:"answer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "所以最少取出 21 个糖果。", payload.Data.Answer)

	// 无可用样本 → 404。
	repo.passOK = false
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-iq-probe/results/7/pass-answer", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}
