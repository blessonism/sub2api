package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type iqProbeSettingsStub struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func (s *iqProbeSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (s *iqProbeSettingsStub) Set(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

type iqProbeAccountsStub struct {
	AccountRepository
	accounts []Account
}

func (r *iqProbeAccountsStub) ListSchedulable(context.Context) ([]Account, error) {
	return r.accounts, nil
}
func (r *iqProbeAccountsStub) UpdateExtra(context.Context, int64, map[string]any) error { return nil }

type iqProbeResultsStub struct {
	ModelIQProbeRepository
	mu           sync.Mutex
	rows         []*ModelIQProbeResult
	insertErr    error
	summaryCalls int
}

func (r *iqProbeResultsStub) Insert(ctx context.Context, result *ModelIQProbeResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.insertErr != nil {
		return r.insertErr
	}
	r.rows = append(r.rows, result)
	return nil
}
func (r *iqProbeResultsStub) ListSummaryModels(_ context.Context, accountIDs []int64) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	models := []string{}
	for _, row := range r.rows {
		if row.AccountID != nil && slices.Contains(accountIDs, *row.AccountID) && !slices.Contains(models, row.Model) {
			models = append(models, row.Model)
		}
	}
	return models, nil
}
func (r *iqProbeResultsStub) ListForSummary(_ context.Context, targets []ModelIQProbeTargetRef, _ int) ([]*ModelIQProbeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summaryCalls++
	rows := []*ModelIQProbeResult{}
	for _, row := range r.rows {
		if row.AccountID != nil && slices.Contains(targets, ModelIQProbeTargetRef{AccountID: *row.AccountID, Model: row.Model}) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

type iqProbeUpstreamStub struct {
	HTTPUpstream
	send func(*http.Request) (*http.Response, error)
}

func (u *iqProbeUpstreamStub) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.send(req)
}
func iqProbeResponse() *http.Response {
	h := make(http.Header)
	h.Set("x-codex-primary-used-percent", "20")
	h.Set("x-codex-primary-window-minutes", "300")
	h.Set("x-codex-primary-reset-after-seconds", "1000")
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"答案 21\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"))}
}
func iqProbeAccount() Account {
	return Account{ID: 1, Name: "private@example.com", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 3, Credentials: map[string]any{"access_token": "test-token"}, Extra: map[string]any{}}
}
func iqProbeServiceForTest(t *testing.T, cfg *ModelIQProbeConfig, results *iqProbeResultsStub, upstream HTTPUpstream) *ModelIQProbeService {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	settings := NewSettingService(&iqProbeSettingsStub{values: map[string]string{SettingKeyModelIQProbeConfig: string(raw)}}, nil)
	accounts := &iqProbeAccountsStub{accounts: []Account{iqProbeAccount()}}
	accountTest := &AccountTestService{accountRepo: accounts, httpUpstream: upstream, cfg: &config.Config{}}
	return NewModelIQProbeService(results, settings, accounts, accountTest)
}
func enabledIQProbeConfig() *ModelIQProbeConfig {
	cfg := DefaultModelIQProbeConfig()
	cfg.Enabled = true
	cfg.Models = []string{"gpt-5.4"}
	return cfg
}

func TestModelIQProbeSendsConfiguredQuestion(t *testing.T) {
	for _, kind := range []string{"oauth", "apikey_responses", "apikey_chat"} {
		t.Run(kind, func(t *testing.T) {
			captured := make(chan []byte, 1)
			upstream := &iqProbeUpstreamStub{send: func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				captured <- body
				if kind == "apikey_chat" {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"答案 21\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))}, nil
				}
				return iqProbeResponse(), nil
			}}
			svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, upstream)
			account := iqProbeAccount()
			if kind != "oauth" {
				account.Type = AccountTypeAPIKey
				account.Credentials = map[string]any{"api_key": "test", "base_url": "https://probe.example/v1"}
				account.Extra[openai_compat.ExtraKeyResponsesSupported] = kind != "apikey_chat"
			}
			result := svc.probeTarget(context.Background(), modelIQProbeTarget{account: &account, model: "gpt-5.4"}, enabledIQProbeConfig())
			require.Equal(t, ModelIQProbeStatusPass, result.Status)
			body := <-captured
			path := "input.0.content.0.text"
			if kind == "apikey_chat" {
				path = "messages.0.content"
			}
			require.Equal(t, DefaultModelIQProbeQuestion, gjson.GetBytes(body, path).String())
		})
	}
}

func TestModelIQProbeConcurrentAccountSnapshots(t *testing.T) {
	cfg := enabledIQProbeConfig()
	cfg.Models = []string{"gpt-5.4", "gpt-5.2", "gpt-5.6"}
	results := &iqProbeResultsStub{}
	arrivals := make(chan struct{}, 3)
	release := make(chan struct{})
	go func() {
		for range 3 {
			<-arrivals
		}
		close(release)
	}()
	upstream := &iqProbeUpstreamStub{send: func(req *http.Request) (*http.Response, error) {
		arrivals <- struct{}{}
		select {
		case <-release:
			return iqProbeResponse(), nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}}
	svc := iqProbeServiceForTest(t, cfg, results, upstream)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	summary, err := svc.RunProbeCycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, summary.Pass)
	require.Empty(t, svc.accountRepo.(*iqProbeAccountsStub).accounts[0].Extra)
	targets, err := svc.resolveTargets(ctx, cfg)
	require.NoError(t, err)
	targets[0].account.Credentials["access_token"] = "changed"
	targets[0].account.Extra["changed"] = true
	require.Equal(t, "test-token", targets[1].account.Credentials["access_token"])
	require.NotContains(t, targets[1].account.Extra, "changed")
}

func TestModelIQProbeReportsStorageFailure(t *testing.T) {
	results := &iqProbeResultsStub{insertErr: errors.New("storage unavailable")}
	upstream := &iqProbeUpstreamStub{send: func(*http.Request) (*http.Response, error) { return iqProbeResponse(), nil }}
	svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), results, upstream)
	summary, err := svc.RunProbeCycle(context.Background())
	require.ErrorContains(t, err, "storage unavailable")
	require.Equal(t, 1, summary.Error)
	require.Equal(t, 1, summary.Total)
}

func TestModelIQProbeCancellationIsNotSuccess(t *testing.T) {
	svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, &iqProbeUpstreamStub{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.RunProbeCycle(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestModelIQProbeBackgroundRunSurvivesRequestCancellation(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan context.Context, 1)
	upstream := &iqProbeUpstreamStub{send: func(req *http.Request) (*http.Response, error) {
		entered <- req.Context()
		select {
		case <-release:
			return iqProbeResponse(), nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}}
	svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, upstream)
	runner := NewModelIQProbeRunner(svc)
	t.Cleanup(runner.Stop)
	requestCtx, cancel := context.WithCancel(context.Background())
	state, err := runner.StartRun(requestCtx)
	require.NoError(t, err)
	require.Equal(t, "running", state.Status)
	cancel()
	var upstreamCtx context.Context
	select {
	case upstreamCtx = <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("probe did not start")
	}
	require.NoError(t, upstreamCtx.Err())
	_, err = runner.StartRun(context.Background())
	require.ErrorIs(t, err, ErrModelIQProbeRunInProgress)
	otherInstance := NewModelIQProbeRunner(svc)
	t.Cleanup(otherInstance.Stop)
	_, err = otherInstance.StartRun(context.Background())
	require.ErrorIs(t, err, ErrModelIQProbeRunInProgress)
	close(release)
	require.Eventually(t, func() bool {
		current, err := svc.GetRunState(context.Background())
		return err == nil && current.Status == "completed" && current.Summary.Pass == 1
	}, 3*time.Second, 10*time.Millisecond)
	// 请求响应快照不能被后台进度并发修改。
	require.Equal(t, 0, state.Summary.Pass)
}

func TestModelIQProbeBackgroundRunFailureAndDisabled(t *testing.T) {
	upstream := &iqProbeUpstreamStub{send: func(*http.Request) (*http.Response, error) { return iqProbeResponse(), nil }}
	cfg := enabledIQProbeConfig()
	svc := iqProbeServiceForTest(t, cfg, &iqProbeResultsStub{insertErr: errors.New("disk full")}, upstream)
	runner := NewModelIQProbeRunner(svc)
	defer runner.Stop()
	_, err := runner.StartRun(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		state, err := svc.GetRunState(context.Background())
		return err == nil && state.Status == "failed" && state.Summary.Error == 1 && strings.Contains(state.ErrorMessage, "disk full")
	}, 3*time.Second, 10*time.Millisecond)
	cfg.Enabled = false
	require.NoError(t, svc.SaveConfig(context.Background(), cfg))
	_, err = runner.StartRun(context.Background())
	require.ErrorIs(t, err, ErrModelIQProbeDisabled)
}

func TestModelIQProbeVisibilityAndDisplayProjection(t *testing.T) {
	a, b := int64(1), int64(2)
	now := time.Now()
	rows := []*ModelIQProbeResult{
		{AccountID: &b, AccountName: "secret-b", Model: "gpt-5.4", Status: ModelIQProbeStatusPass, ActualAnswer: "21", CreatedAt: now},
		{AccountID: &a, AccountName: "secret-a", Model: "gpt-5.4", Status: ModelIQProbeStatusWrongAnswer, ActualAnswer: "20", CreatedAt: now.Add(-time.Minute)},
		{AccountID: &a, Model: "gpt-5.2", Status: ModelIQProbeStatusError, CreatedAt: now},
	}
	for _, tc := range []struct {
		name                       string
		admin, public, green, mark bool
	}{
		{name: "hidden"}, {name: "admin always real", admin: true, green: true, mark: true},
		{name: "public real", public: true, mark: true}, {name: "public green", public: true, green: true, mark: true},
		{name: "no marker", public: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := enabledIQProbeConfig()
			cfg.Models = []string{"gpt-5.2", "gpt-5.4"}
			if tc.public {
				cfg.Visibility = ModelIQProbeVisibilityAllUsers
			}
			if tc.green {
				cfg.UserDisplayMode = ModelIQProbeDisplayAlwaysGreen
			}
			cfg.ShowExclusion = tc.mark
			results := &iqProbeResultsStub{rows: rows}
			svc := iqProbeServiceForTest(t, cfg, results, nil)
			second := iqProbeAccount()
			second.ID = 2
			svc.accountRepo.(*iqProbeAccountsStub).accounts = []Account{iqProbeAccount(), second}
			items, err := svc.UserSummary(context.Background(), tc.admin)
			require.NoError(t, err)
			if !tc.public && !tc.admin {
				require.Empty(t, items)
				require.Zero(t, results.summaryCalls)
				return
			}
			require.Len(t, items, 2)
			failed := items[1]
			require.Equal(t, "gpt-5.4", failed.Model)
			require.Equal(t, tc.mark, failed.Latest.MarkedForExclusion)
			require.False(t, items[0].Latest.MarkedForExclusion)
			if tc.green && !tc.admin {
				for _, item := range items {
					require.Equal(t, ModelIQProbeStatusPass, item.Latest.Status)
					require.Empty(t, item.Latest.AnswerExcerpt)
					require.Empty(t, item.Latest.Answer)
					for _, point := range item.Recent {
						require.Equal(t, ModelIQProbeStatusPass, point.Status)
						// always-green 模式下点位摘要同样清空，避免泄露降智回答。
						require.Empty(t, point.AnswerExcerpt)
					}
				}
			} else {
				require.Equal(t, ModelIQProbeStatusWrongAnswer, failed.Latest.Status)
				require.Equal(t, "20", failed.Latest.AnswerExcerpt)
				require.Equal(t, "20", failed.Latest.Answer)
			}
			encoded, err := json.Marshal(items)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "secret")
			require.NotContains(t, string(encoded), "account_id")
			require.Equal(t, ModelIQProbeStatusWrongAnswer, rows[1].Status)
		})
	}
}

func TestModelIQProbeListAvailableModels(t *testing.T) {
	cfg := enabledIQProbeConfig()
	cfg.Models = []string{"gpt-legacy"} // 拉取接口不受已配置模型影响，始终实时发现
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	settings := NewSettingService(&iqProbeSettingsStub{values: map[string]string{SettingKeyModelIQProbeConfig: string(raw)}}, nil)

	// OpenAI 账号参与发现；非 OpenAI 账号被过滤。
	probeAccount := Account{
		ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 3,
		Credentials: map[string]any{"api_key": "sk-upstream", "base_url": "https://models.example/v1"},
	}
	anthropicAccount := Account{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
	accounts := &iqProbeAccountsStub{accounts: []Account{probeAccount, anthropicAccount}}

	upstream := &codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		return ordinaryModelsUpstreamResponse(`{"object":"list","data":[
			{"id":"GPT-5.1"},{"id":"gpt-5.2"},{"id":"gpt-image-1"},{"id":"deepseek-chat"}
		]}`), nil
	}}
	gateway := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	accountTest := &AccountTestService{accountRepo: accounts, openaiGatewayService: gateway, httpUpstream: upstream, cfg: &config.Config{}}
	svc := NewModelIQProbeService(&iqProbeResultsStub{}, settings, accounts, accountTest)

	models, err := svc.ListAvailableModels(context.Background())
	require.NoError(t, err)
	// 小写归一并取 gpt-* 文本模型并集；image 档与非 GPT 模型不进选项。
	require.Equal(t, []string{"gpt-5.1", "gpt-5.2"}, models)
}

func TestModelIQProbeDisabledAccountsStayListedButSkipped(t *testing.T) {
	cfg := enabledIQProbeConfig()
	cfg.AccountIDs = []int64{2, 4}
	cfg.DisabledAccountIDs = []int64{4} // 保留在名单中，临时停用
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	settings := NewSettingService(&iqProbeSettingsStub{values: map[string]string{SettingKeyModelIQProbeConfig: string(raw)}}, nil)

	openaiAccount := func(id int64, apiKey string) Account {
		return Account{
			ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Schedulable: true, Concurrency: 3,
			Credentials: map[string]any{"api_key": apiKey, "base_url": "https://models.example/v1"},
		}
	}
	accounts := &iqProbeAccountsStub{accounts: []Account{openaiAccount(2, "sk-one"), openaiAccount(4, "sk-two")}}

	upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		// 不同账号返回不同目录：停用账号的模型不应出现在并集里。
		if strings.HasSuffix(req.Header.Get("Authorization"), "sk-two") {
			return ordinaryModelsUpstreamResponse(`{"object":"list","data":[{"id":"gpt-5.2"}]}`), nil
		}
		return ordinaryModelsUpstreamResponse(`{"object":"list","data":[{"id":"gpt-5.1"}]}`), nil
	}}
	gateway := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	accountTest := &AccountTestService{accountRepo: accounts, openaiGatewayService: gateway, httpUpstream: upstream, cfg: &config.Config{}}
	svc := NewModelIQProbeService(&iqProbeResultsStub{}, settings, accounts, accountTest)

	models, err := svc.ListAvailableModels(context.Background())
	require.NoError(t, err)
	// 账号 4 被临时停用：其目录里的 gpt-5.2 不参与并集。
	require.Equal(t, []string{"gpt-5.1"}, models)
}

func TestModelIQProbeSummaryRecoversPerAccount(t *testing.T) {
	a, b := int64(1), int64(2)
	rows := []*ModelIQProbeResult{
		{AccountID: &a, Model: "gpt-5.4", Status: ModelIQProbeStatusPass},
		{AccountID: &b, Model: "gpt-5.4", Status: ModelIQProbeStatusPass},
		{AccountID: &a, Model: "gpt-5.4", Status: ModelIQProbeStatusWrongAnswer},
	}
	require.Equal(t, ModelIQProbeStatusPass, userSummaryFromRows(rows)[0].Latest.Status)
	cfg := DecodeModelIQProbeConfig(`{"enabled":true}`)
	require.Equal(t, ModelIQProbeVisibilityAdminOnly, cfg.Visibility)
	require.Equal(t, ModelIQProbeDisplayReal, cfg.UserDisplayMode)
	cfg.Visibility = "invalid"
	require.Error(t, cfg.Validate())
	cfg.Visibility = ModelIQProbeVisibilityAllUsers
	cfg.UserDisplayMode = "invalid"
	require.Error(t, cfg.Validate())
}

func TestModelIQProbeSummaryUsesCurrentTargets(t *testing.T) {
	a, b := int64(1), int64(2)
	now := time.Now()
	rows := []*ModelIQProbeResult{
		{AccountID: &a, Model: "gpt-5.4", Status: ModelIQProbeStatusPass, CreatedAt: now},
		{AccountID: &b, Model: "gpt-5.4", Status: ModelIQProbeStatusWrongAnswer, CreatedAt: now.Add(-time.Hour)},
	}
	for _, tc := range []struct {
		name                                string
		mode                                string
		disabled, deleted, auto, noAccounts bool
		want                                string
	}{
		{name: "model only drops previous targets", mode: ModelIQProbeTargetModeModelOnly, want: ModelIQProbeStatusPass},
		{name: "disabled account", mode: ModelIQProbeTargetModeAccountModel, disabled: true, want: ModelIQProbeStatusPass},
		{name: "deleted account", mode: ModelIQProbeTargetModeAccountModel, deleted: true, want: ModelIQProbeStatusPass},
		{name: "automatic local model discovery", mode: ModelIQProbeTargetModeModelOnly, auto: true, want: ModelIQProbeStatusPass},
		{name: "active failure remains", mode: ModelIQProbeTargetModeAccountModel, want: ModelIQProbeStatusWrongAnswer},
		{name: "no current accounts", mode: ModelIQProbeTargetModeAccountModel, noAccounts: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := enabledIQProbeConfig()
			cfg.TargetMode = tc.mode
			cfg.ShowExclusion = true
			cfg.Visibility = ModelIQProbeVisibilityAllUsers
			if tc.auto {
				cfg.Models = nil
			}
			results := &iqProbeResultsStub{rows: rows}
			svc := iqProbeServiceForTest(t, cfg, results, nil)
			second := iqProbeAccount()
			second.ID = 2
			if tc.disabled {
				second.Status = StatusDisabled
			}
			accounts := []Account{iqProbeAccount(), second}
			if tc.deleted {
				accounts = accounts[:1]
			}
			if tc.noAccounts {
				accounts = nil
			}
			svc.accountRepo.(*iqProbeAccountsStub).accounts = accounts
			items, err := svc.UserSummary(context.Background(), false)
			require.NoError(t, err)
			if tc.noAccounts {
				require.Empty(t, items)
				require.Zero(t, results.summaryCalls)
				return
			}
			require.Len(t, items, 1)
			require.Equal(t, tc.want, items[0].Latest.Status)
			require.Equal(t, tc.want == ModelIQProbeStatusWrongAnswer, items[0].Latest.MarkedForExclusion)
			if tc.want == ModelIQProbeStatusPass {
				require.Len(t, items[0].Recent, 1)
			}
			require.Len(t, results.rows, 2) // 历史明细不删除。
		})
	}
}

type iqProbeFinalWriteFailureStub struct {
	*iqProbeSettingsStub
	muFailures sync.Mutex
	remaining  int
}

func (s *iqProbeFinalWriteFailureStub) Set(ctx context.Context, key, value string) error {
	if key == settingKeyModelIQProbeRunState {
		var state ModelIQProbeRunState
		if err := json.Unmarshal([]byte(value), &state); err != nil {
			return err
		}
		if state.Status != "running" {
			s.muFailures.Lock()
			fail := s.remaining > 0
			if fail {
				s.remaining--
			}
			s.muFailures.Unlock()
			if fail {
				return errors.New("temporary terminal write failure")
			}
		}
	}
	return s.iqProbeSettingsStub.Set(ctx, key, value)
}

func TestModelIQProbeRetriesTerminalWriteAndRecoversStaleState(t *testing.T) {
	for _, failures := range []int{1, 3} {
		t.Run(fmt.Sprintf("failures-%d", failures), func(t *testing.T) {
			upstream := &iqProbeUpstreamStub{send: func(*http.Request) (*http.Response, error) { return iqProbeResponse(), nil }}
			svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, upstream)
			flaky := &iqProbeFinalWriteFailureStub{iqProbeSettingsStub: svc.settings.settingRepo.(*iqProbeSettingsStub), remaining: failures}
			svc.settings = NewSettingService(flaky, nil)
			runner := NewModelIQProbeRunner(svc)
			defer runner.Stop()
			initial, err := runner.StartRun(context.Background())
			require.NoError(t, err)
			runner.wg.Wait()
			raw, err := svc.GetRunState(context.Background())
			require.NoError(t, err)
			if failures == 1 {
				require.Equal(t, "completed", raw.Status)
			} else {
				require.Equal(t, "running", raw.Status)
			}
			state, err := runner.GetRunState(context.Background())
			require.NoError(t, err)
			if failures == 1 {
				require.Equal(t, "completed", state.Status)
			} else {
				require.Equal(t, "failed", state.Status)
				require.NotNil(t, state.FinishedAt)
			}
			stored, err := svc.GetRunState(context.Background())
			require.NoError(t, err)
			require.Equal(t, state.Status, stored.Status)
			next, err := runner.StartRun(context.Background())
			require.NoError(t, err)
			require.NotEqual(t, initial.ID, next.ID)
			runner.wg.Wait()
		})
	}
}

func TestModelIQProbeRecoveryPreservesOtherInstanceActiveRun(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	upstream := &iqProbeUpstreamStub{send: func(req *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return iqProbeResponse(), nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}}
	svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, upstream)
	otherSvc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, nil)
	otherSvc.settings = svc.settings
	cache := &fakeLeaderLockCache{}
	first := NewModelIQProbeRunner(svc)
	defer first.Stop()
	first.SetLeaderLock(cache, nil)
	second := NewModelIQProbeRunner(otherSvc)
	defer second.Stop()
	second.SetLeaderLock(cache, nil)
	_, err := first.StartRun(context.Background())
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("probe did not start")
	}
	state, err := second.GetRunState(context.Background())
	require.NoError(t, err)
	require.Equal(t, "running", state.Status)
	require.Equal(t, first.instanceID, cache.heldBy(modelIQProbeLeaderLockKey))
	_, err = second.StartRun(context.Background())
	require.ErrorIs(t, err, ErrModelIQProbeRunInProgress)
	close(release)
	first.wg.Wait()
	state, err = second.GetRunState(context.Background())
	require.NoError(t, err)
	require.Equal(t, "completed", state.Status)
}

func TestModelIQProbeStartReplacesOrphanedRunningState(t *testing.T) {
	upstream := &iqProbeUpstreamStub{send: func(*http.Request) (*http.Response, error) { return iqProbeResponse(), nil }}
	svc := iqProbeServiceForTest(t, enabledIQProbeConfig(), &iqProbeResultsStub{}, upstream)
	now := time.Now()
	require.NoError(t, svc.saveRunState(context.Background(), ModelIQProbeRunState{ID: "orphan", Status: "running", StartedAt: &now}))
	runner := NewModelIQProbeRunner(svc)
	defer runner.Stop()
	state, err := runner.StartRun(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, "orphan", state.ID)
	runner.wg.Wait()
}
