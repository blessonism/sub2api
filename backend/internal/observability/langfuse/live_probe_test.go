//go:build langfuselive

package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLiveIngestionReachesLangfuse 是 G2 探针 4 的实机版本：
// 用真实 Client 投递一条调用记录到已部署的 Langfuse，再通过公共检索 API
// 回读 trace / generation / session，确认 cost 与投递值一致。
//
// 运行方式：
//
//	LF_HOST=http://127.0.0.1:13000 \
//	LF_PUBLIC_KEY=pk-lf-... LF_SECRET_KEY=sk-lf-... \
//	go test -tags langfuselive ./internal/observability/langfuse/ -run TestLiveIngestionReachesLangfuse -v
func TestLiveIngestionReachesLangfuse(t *testing.T) {
	host := os.Getenv("LF_HOST")
	pk := os.Getenv("LF_PUBLIC_KEY")
	sk := os.Getenv("LF_SECRET_KEY")
	if host == "" || pk == "" || sk == "" {
		t.Skip("需要 LF_HOST / LF_PUBLIC_KEY / LF_SECRET_KEY")
	}

	sessionID := "sess-live-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	const charge = 0.4242
	started := time.Now().Add(-2 * time.Second).UTC()
	// 观察窗口长度（秒）决定重试上限。
	client := NewClient(Options{
		Host:          host,
		PublicKey:     pk,
		SecretKey:     sk,
		QueueSize:     8,
		BatchSize:     1,
		FlushInterval: time.Hour,
		Timeout:       10 * time.Second,
	})
	client.Observe(Call{
		UserID:          4242,
		SessionID:       sessionID,
		RequestID:       "live-probe-" + sessionID,
		Model:           "sub2api-live-probe",
		Endpoint:        "/v1/chat/completions",
		Status:          "ok",
		StartedAt:       started,
		Duration:        1234 * time.Millisecond,
		InputTokens:     1000,
		OutputTokens:    250,
		CacheReadTokens: 100,
		Cost:            charge,
		Currency:        "USD",
		BillingType:     "balance",
		BillingMode:     "token",
		Stream:          true,
		Input:           `{"model":"sub2api-live-probe","messages":[{"role":"system","content":"你是助手"},{"role":"user","content":"帮我写个快排"}]}`,
		InputUser:       "帮我写个快排",
		UserTurns:       1,
		Output:          "好的，下面是快排实现：func quickSort(...)",
		AccountID:       77,
		APIKeyID:        88,
		GroupID:         99,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if got := client.Failed(); got != 0 {
		t.Fatalf("ingestion 上报失败批次数 = %d", got)
	}

	deadline := time.Now().Add(120 * time.Second)
	for {
		trace := findTrace(t, host, pk, sk, sessionID)
		if trace != nil {
			if trace.UserID != "4242" {
				t.Fatalf("trace.userId = %q, want 4242", trace.UserID)
			}
			if trace.SessionID != sessionID {
				t.Fatalf("trace.sessionId = %q, want %q", trace.SessionID, sessionID)
			}
			// trace 与 observation 是两条独立事件，索引落定的时刻不同步，
			// 这里单独给 generation 一段轮询窗口。
			var gen *liveGeneration
			genDeadline := time.Now().Add(60 * time.Second)
			for {
				if gen = findGeneration(t, host, pk, sk, trace.ID); gen != nil {
					break
				}
				if time.Now().After(genDeadline) {
					t.Fatalf("trace %s 下 60s 内没有 generation", trace.ID)
				}
				time.Sleep(2 * time.Second)
			}
			if gen.Model != "sub2api-live-probe" {
				t.Fatalf("generation.model = %q", gen.Model)
			}
			if got := gen.cost(); got != charge {
				t.Fatalf("generation cost = %v, want %v（实扣）", got, charge)
			}
			// trace 级 totalCost 由聚合侧补齐，晚于 generation 落库；单独给一段轮询窗口。
			costDeadline := time.Now().Add(45 * time.Second)
			for {
				refreshed := findTrace(t, host, pk, sk, sessionID)
				if refreshed != nil && refreshed.TotalCost != nil && *refreshed.TotalCost == charge {
					break
				}
				if time.Now().After(costDeadline) {
					got := "nil"
					if refreshed != nil && refreshed.TotalCost != nil {
						got = fmt.Sprintf("%v", *refreshed.TotalCost)
					}
					t.Fatalf("trace.totalCost 45s 内未收敛到 %v（当前 %s）", charge, got)
				}
				time.Sleep(3 * time.Second)
			}
			if got := strings.Trim(string(gen.Input), `"`); got != "帮我写个快排" {
				t.Fatalf("generation.input 应为用户 prompt，实际 %q", got)
			}
			if !strings.Contains(string(gen.Output), "下面是快排实现") {
				t.Fatalf("generation.output 未携带响应正文: %s", string(gen.Output))
			}
			t.Logf("trace=%s session=%s userId=%s cost=%v model=%s input_bytes=%d output_bytes=%d",
				trace.ID, trace.SessionID, trace.UserID, gen.cost(), gen.Model, len(gen.Input), len(gen.Output))
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("120s 内未在 Langfuse 检索到 session=%s 的 trace", sessionID)
		}
		time.Sleep(3 * time.Second)
	}
}

type liveTrace struct {
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	UserID    string          `json:"userId"`
	SessionID string          `json:"sessionId"`
	TotalCost *float64        `json:"totalCost"`
}

type liveGeneration struct {
	ID      string `json:"id"`
	TraceID string `json:"traceId"`
	Model   string `json:"model"`
	// 正文层：Langfuse 存的是任意 JSON 值，这里按原始字节比对。
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
	// totalCost 与 calculatedTotalCost 在不同版本里择一出现，取到哪个用哪个。
	TotalCost           *float64 `json:"totalCost"`
	CalculatedTotalCost *float64 `json:"calculatedTotalCost"`
}

func (g liveGeneration) cost() float64 {
	if g.TotalCost != nil {
		return *g.TotalCost
	}
	if g.CalculatedTotalCost != nil {
		return *g.CalculatedTotalCost
	}
	return 0
}

func lfGet(t *testing.T, url, pk, sk string, out any) bool {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.SetBasicAuth(pk, sk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(out) == nil
}

func findTrace(t *testing.T, host, pk, sk, sessionID string) *liveTrace {
	t.Helper()
	var payload struct {
		Data []liveTrace `json:"data"`
	}
	url := host + "/api/public/traces?limit=50&sessionId=" + sessionID
	if !lfGet(t, url, pk, sk, &payload) {
		return nil
	}
	for i := range payload.Data {
		if payload.Data[i].SessionID == sessionID {
			return &payload.Data[i]
		}
	}
	return nil
}

func findGeneration(t *testing.T, host, pk, sk, traceID string) *liveGeneration {
	t.Helper()
	var payload struct {
		Data []liveGeneration `json:"data"`
	}
	url := host + "/api/public/observations?limit=50&traceId=" + traceID
	if !lfGet(t, url, pk, sk, &payload) {
		return nil
	}
	for i := range payload.Data {
		if payload.Data[i].TraceID == traceID {
			return &payload.Data[i]
		}
	}
	return nil
}
