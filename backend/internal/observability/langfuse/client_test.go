package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// capturedBatch 是测试侧对 ingestion 请求体的解包视图。
type capturedBatch struct {
	Batch []struct {
		ID        string         `json:"id"`
		Type      string         `json:"type"`
		Timestamp string         `json:"timestamp"`
		Body      map[string]any `json:"body"`
	} `json:"batch"`
}

type recorder struct {
	mu      sync.Mutex
	hits    atomic.Int64
	auth    string
	path    string
	batches []capturedBatch
}

func newRecorder() *recorder { return &recorder{} }

func (r *recorder) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, req *http.Request) {
		r.hits.Add(1)
		var b capturedBatch
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
			t.Errorf("decode ingestion body: %v", err)
		}
		r.mu.Lock()
		r.auth = req.Header.Get("Authorization")
		r.path = req.URL.Path
		r.batches = append(r.batches, b)
		r.mu.Unlock()
		w.WriteHeader(http.StatusMultiStatus)
	}
}

func (r *recorder) all() []capturedBatch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]capturedBatch(nil), r.batches...)
}

// TestBatchCarriesTraceAndGenerationWithActualCost 覆盖目标单 G2 探针 2：
// batch 结构、字段映射、cost 来源为实扣。
func TestBatchCarriesTraceAndGenerationWithActualCost(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host:          srv.URL,
		PublicKey:     "pk-lf-test",
		SecretKey:     "sk-lf-test",
		QueueSize:     16,
		BatchSize:     8,
		FlushInterval: time.Hour, // 关掉定时器，冲刷完全由 Close 驱动
		Timeout:       3 * time.Second,
	})

	start := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	client.Observe(Call{
		UserID:                42,
		SessionID:             "sess-abc",
		RequestID:             "req-1",
		Model:                 "gpt-5.2",
		Endpoint:              "/v1/chat/completions",
		Status:                "ok",
		StartedAt:             start,
		Duration:              1500 * time.Millisecond,
		InputTokens:           100,
		OutputTokens:          20,
		CacheReadTokens:       30,
		CacheWriteTokens:      5,
		Cost:                  0.1234, // 实扣值：与 token×标价无关，必须原样上报
		Input:                 `{"messages":[{"role":"user","content":"你好"}]}`,
		Output:                "世界，你好",
		Currency:              "USD",
		RateMultiplier:        0.8,
		VisibleRateMultiplier: 1.0,
		BillingType:           "balance",
		BillingMode:           "token",
		Stream:                true,
		AccountID:             7,
		APIKeyID:              9,
		GroupID:               3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("close client: %v", err)
	}

	if got := rec.hits.Load(); got != 1 {
		t.Fatalf("ingestion 请求次数 = %d, want 1", got)
	}
	if rec.path != ingestionPath {
		t.Fatalf("上报路径 = %q, want %q", rec.path, ingestionPath)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-lf-test:sk-lf-test"))
	if rec.auth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", rec.auth, wantAuth)
	}

	batches := rec.all()
	if len(batches) != 1 {
		t.Fatalf("批次数 = %d, want 1", len(batches))
	}
	events := batches[0].Batch
	if len(events) != 2 {
		t.Fatalf("单次调用的批内事件数 = %d, want 2 (1 trace + 1 generation)", len(events))
	}

	trace, gen := events[0], events[1]
	if trace.Type != eventTypeTraceCreate {
		t.Fatalf("事件[0].Type = %q, want %q", trace.Type, eventTypeTraceCreate)
	}
	if gen.Type != eventTypeGenerationCreate {
		t.Fatalf("事件[1].Type = %q, want %q", gen.Type, eventTypeGenerationCreate)
	}
	if gen.Body["traceId"] != trace.Body["id"] {
		t.Fatalf("generation.traceId = %v, want trace.id = %v", gen.Body["traceId"], trace.Body["id"])
	}

	// 检索维度：userId / sessionId 必须落在 trace 上。
	if got := trace.Body["userId"]; got != "42" {
		t.Fatalf("trace.userId = %v, want \"42\"", got)
	}
	if got := trace.Body["sessionId"]; got != "sess-abc" {
		t.Fatalf("trace.sessionId = %v, want \"sess-abc\"", got)
	}
	if got := trace.Body["timestamp"]; got != "2026-03-01T10:00:00.000Z" {
		t.Fatalf("trace.timestamp = %v, want 2026-03-01T10:00:00.000Z", got)
	}

	// 费用口径：costDetails.total 必须等于实扣值。
	costDetails, ok := gen.Body["costDetails"].(map[string]any)
	if !ok {
		t.Fatalf("generation.costDetails 缺失或类型错误: %#v", gen.Body["costDetails"])
	}
	if got := costDetails["total"]; got != 0.1234 {
		t.Fatalf("costDetails.total = %v, want 0.1234（实扣）", got)
	}

	// 用量映射：input/output/total，缓存桶单列。
	usage, ok := gen.Body["usageDetails"].(map[string]any)
	if !ok {
		t.Fatalf("generation.usageDetails 缺失: %#v", gen.Body["usageDetails"])
	}
	if usage["input"] != float64(100) || usage["output"] != float64(20) {
		t.Fatalf("usageDetails input/output = %v/%v, want 100/20", usage["input"], usage["output"])
	}
	if usage["total"] != float64(155) {
		t.Fatalf("usageDetails.total = %v, want 155", usage["total"])
	}
	if usage["cache_read"] != float64(30) || usage["cache_write"] != float64(5) {
		t.Fatalf("cache 桶 = %v/%v, want 30/5", usage["cache_read"], usage["cache_write"])
	}

	if gen.Body["level"] != levelDefault {
		t.Fatalf("generation.level = %v, want %q", gen.Body["level"], levelDefault)
	}

	meta, ok := gen.Body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("generation.metadata 缺失")
	}
	if meta["cost_source"] != costSourceActual {
		t.Fatalf("metadata.cost_source = %v, want %q", meta["cost_source"], costSourceActual)
	}
	if meta["request_id"] != "req-1" {
		t.Fatalf("metadata.request_id = %v, want req-1", meta["request_id"])
	}
	// 正文层：input/output 必须原样落到 trace 与 generation。
	if got := gen.Body["input"]; got != `{"messages":[{"role":"user","content":"你好"}]}` {
		t.Fatalf("generation.input = %v", got)
	}
	if got := gen.Body["output"]; got != "世界，你好" {
		t.Fatalf("generation.output = %v", got)
	}
	if got := trace.Body["output"]; got != "世界，你好" {
		t.Fatalf("trace.output = %v", got)
	}
}

// TestBillingErrorGenerationIsMarkedError 覆盖失败调用仍成 generation 的约定。
func TestBillingErrorGenerationIsMarkedError(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 8, FlushInterval: time.Hour, Timeout: 2 * time.Second,
	})
	client.Observe(Call{
		UserID: 7, RequestID: "req-err", Model: "gpt-5.2",
		Status: "billing_error", ErrorKind: "quota_exhausted",
		StartedAt: time.Now(), Cost: 0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Close(ctx)

	events := rec.all()[0].Batch
	if len(events) != 2 {
		t.Fatalf("事件数 = %d, want 2", len(events))
	}
	gen := events[1]
	if gen.Body["level"] != levelError {
		t.Fatalf("level = %v, want %q", gen.Body["level"], levelError)
	}
	if gen.Body["statusMessage"] != "quota_exhausted" {
		t.Fatalf("statusMessage = %v, want quota_exhausted", gen.Body["statusMessage"])
	}
}

// TestDisabledConfigEmitsNoHTTPRequest 覆盖目标单 G2 探针 3：
// 关开关后零出网、零副作用。
func TestDisabledConfigEmitsNoHTTPRequest(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	cases := []struct {
		name string
		opts Options
	}{
		{"enabled=false 但地址与密钥齐全", Options{Enabled: false, Host: srv.URL, PublicKey: "p", SecretKey: "s"}},
		{"enabled=true 但私钥缺失", Options{Enabled: true, Host: srv.URL, PublicKey: "p"}},
		{"enabled=true 但地址缺失", Options{Enabled: true, PublicKey: "p", SecretKey: "s"}},
		{"零值配置", Options{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := rec.hits.Load()
			sink := New(tc.opts)
			if _, ok := sink.(*Client); ok {
				t.Fatalf("关闭态返回了真实客户端")
			}

			SetDefault(sink)
			if Active() {
				t.Fatalf("关闭态 Active() = true")
			}
			for i := 0; i < 50; i++ {
				Observe(Call{UserID: int64(i), Model: "gpt-5.2", Cost: 1, StartedAt: time.Now()})
			}
			time.Sleep(80 * time.Millisecond)
			if after := rec.hits.Load(); after != before {
				t.Fatalf("关闭态发出了 %d 次 HTTP 请求", after-before)
			}
		})
	}

	SetDefault(nil)
}

// TestUnsetDefaultIsNoop 覆盖"从未装配"这一默认路径。
func TestUnsetDefaultIsNoop(t *testing.T) {
	SetDefault(nil)
	if Active() {
		t.Fatal("未装配时 Active() 必须为 false")
	}
	// 必须不 panic，且不分配上报端。
	Observe(Call{UserID: 1})
	Observe(Call{})
}

// TestQueueFullDropsWithoutBlocking 覆盖"队列满即丢、不阻塞业务侧"。
func TestQueueFullDropsWithoutBlocking(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()
	defer close(block)

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 2, BatchSize: 1, FlushInterval: time.Hour, Timeout: time.Second,
	})

	start := time.Now()
	for i := 0; i < 2000; i++ {
		client.Observe(Call{UserID: int64(i), Model: "gpt-5.2"})
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("Observe 阻塞了 %s，队列满时必须立即返回", elapsed)
	}
	if client.Dropped() == 0 {
		t.Fatal("队列满时未记录丢弃")
	}
	if client.Dropped() > 2000 {
		t.Fatalf("丢弃数 %d 超过投递总数", client.Dropped())
	}
}

// TestCloseFlushesPendingCalls 覆盖退出冲刷。
func TestCloseFlushesPendingCalls(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 32, BatchSize: 32, FlushInterval: time.Hour, Timeout: 2 * time.Second,
	})
	for i := 0; i < 5; i++ {
		client.Observe(Call{UserID: int64(i), Model: "m", StartedAt: time.Now()})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	batches := rec.all()
	total := 0
	for _, b := range batches {
		total += len(b.Batch)
	}
	if total != 10 {
		t.Fatalf("退出冲刷的事件数 = %d, want 10 (5 调用 × 2)", total)
	}
	if client.Sent() != 5 {
		t.Fatalf("Sent() = %d, want 5", client.Sent())
	}
}

// TestBatchSplitsOnByteBudget 覆盖开了正文采集后的字节预算：
// 条数没到上限、但单批体积超限时必须提前冲刷，避免一次 POST 顶到数 MB。
func TestBatchSplitsOnByteBudget(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 8, MaxBatchBytes: 8 * 1024,
		FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	body := strings.Repeat("x", 4*1024) // 单条 ~4KB，加固定开销后两条就超 8KB
	for i := 0; i < 3; i++ {
		client.Observe(Call{UserID: int64(i), Model: "m", Input: body, Output: body})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Close(ctx)

	if got := rec.hits.Load(); got < 2 {
		t.Fatalf("批次数量 = %d, want >=2（字节预算必须触发拆分）", got)
	}
	for _, b := range rec.all() {
		if len(b.Batch) > 2 {
			t.Fatalf("单批事件数 = %d, want <=2", len(b.Batch))
		}
	}
}
