//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/observability/langfuse"
	"github.com/stretchr/testify/require"
)

// ingestionRecorder 是本地 mock 的 Langfuse ingestion 端点。
type ingestionRecorder struct {
	mu     sync.Mutex
	events []map[string]any
	hits   int
}

func (r *ingestionRecorder) handler(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Batch []struct {
			Type string         `json:"type"`
			Body map[string]any `json:"body"`
		} `json:"batch"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.hits++
	for _, ev := range body.Batch {
		body := ev.Body
		body["__type"] = ev.Type
		r.events = append(r.events, body)
	}
	r.mu.Unlock()
	w.WriteHeader(http.StatusMultiStatus)
}

func (r *ingestionRecorder) snapshot() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]any(nil), r.events...)
}

func (r *ingestionRecorder) hitCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hits
}

// TestRecordUsageEmitsOneTraceOneGenerationWithActualCost 是 G2 的端到端联调探针：
// 走真实 RecordUsage 计费链路 → mock ingestion 端点 → 断言 1 trace + 1 generation、
// 检索键落位、且 cost 与业务库落库的实扣金额逐位一致。
func TestRecordUsageEmitsOneTraceOneGenerationWithActualCost(t *testing.T) {
	rec := &ingestionRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	client := langfuse.NewClient(langfuse.Options{
		Host:          srv.URL,
		PublicKey:     "pk-lf-e2e",
		SecretKey:     "sk-lf-e2e",
		QueueSize:     16,
		BatchSize:     8,
		FlushInterval: time.Hour,
		Timeout:       3 * time.Second,
	})
	langfuse.SetDefault(client)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
		langfuse.SetDefault(nil)
	})

	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_obs_e2e",
			Usage:     ClaudeUsage{InputTokens: 1200, OutputTokens: 400},
			Model:     "claude-sonnet-4",
			Duration:  800 * time.Millisecond,
		},
		APIKey:          &APIKey{ID: 501, Quota: 100},
		User:            &User{ID: 601},
		Account:         &Account{ID: 701},
		SessionID:       "sess-e2e",
		InboundEndpoint: "/v1/messages",
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)

	// 冲刷并断言只有一次 ingestion 调用（一条模型调用 = 一条 trace + 一条 generation）。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Close(ctx))

	require.Equal(t, 1, rec.hitCount())
	events := rec.snapshot()
	require.Len(t, events, 2)

	byType := map[string]map[string]any{}
	for _, ev := range events {
		byType[ev["__type"].(string)] = ev
	}
	trace, ok := byType["trace-create"]
	require.True(t, ok, "缺少 trace-create 事件")
	gen, ok := byType["generation-create"]
	require.True(t, ok, "缺少 generation-create 事件")

	require.Equal(t, "601", trace["userId"])
	require.Equal(t, "sess-e2e", trace["sessionId"])
	require.Equal(t, trace["id"], gen["traceId"])

	// cost 必须等于业务库落库的实扣金额。
	costDetails, ok := gen["costDetails"].(map[string]any)
	require.True(t, ok, "generation 缺少 costDetails")
	emitted, ok := costDetails["total"].(float64)
	require.True(t, ok)
	require.InDelta(t, usageRepo.lastLog.ActualCost, emitted, 1e-12,
		"观测层 cost 必须等于 usage_logs.ActualCost（实扣），当前落库=%v 上报=%v",
		usageRepo.lastLog.ActualCost, emitted)
	require.Greater(t, emitted, 0.0, "本用例应产生非零实扣，便于验证口径而非零值通过")

	usage, ok := gen["usageDetails"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(usageRepo.lastLog.InputTokens), usage["input"])
	require.Equal(t, float64(usageRepo.lastLog.OutputTokens), usage["output"])
}

// TestRecordUsageWithObservabilityDisabledEmitsNothing 是 G2 探针 3 的端到端版本：
// 开关关闭时，同一条计费链路不产生任何出网请求。
func TestRecordUsageWithObservabilityDisabledEmitsNothing(t *testing.T) {
	rec := &ingestionRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	// 地址与密钥齐全，但 enabled=false —— 必须整体空转。
	langfuse.SetDefault(langfuse.New(langfuse.Options{
		Enabled:   false,
		Host:      srv.URL,
		PublicKey: "pk-lf-off",
		SecretKey: "sk-lf-off",
	}))
	t.Cleanup(func() { langfuse.SetDefault(nil) })

	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_obs_disabled",
			Usage:     ClaudeUsage{InputTokens: 1200, OutputTokens: 400},
			Model:     "claude-sonnet-4",
			Duration:  800 * time.Millisecond,
		},
		APIKey:  &APIKey{ID: 501, Quota: 100},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)

	time.Sleep(120 * time.Millisecond)
	require.Equal(t, 0, rec.hitCount(), "关闭态必须零出网")
}
