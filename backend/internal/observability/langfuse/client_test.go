package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
		Input:                 `{"model":"gpt-5.2","messages":[{"role":"system","content":"sys"},{"role":"user","content":"你好"}]}`,
		InputUser:             "你好",
		UserTurns:             1,
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
		t.Fatalf("单次调用的批内事件数 = %d, want 2 (1 generation + 1 trace)", len(events))
	}

	// 顺序是 generation 在前、卡片在后：步骤即刻上报，卡片等这一轮结束。
	gen, trace := events[0], events[1]
	if gen.Type != eventTypeGenerationCreate {
		t.Fatalf("事件[0].Type = %q, want %q", gen.Type, eventTypeGenerationCreate)
	}
	if trace.Type != eventTypeTraceCreate {
		t.Fatalf("事件[1].Type = %q, want %q", trace.Type, eventTypeTraceCreate)
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
	// 正文分层：卡片放该轮的用户消息；步骤放「这一步新加进来的内容」，
	// 没有工具结果时退回用户消息 —— 总之不能留空让 UI 渲染成 undefined。
	if got := gen.Body["input"]; got != "你好" {
		t.Fatalf("generation.input = %v, want 你好（首步没有工具结果，退回用户消息）", got)
	}
	if got := gen.Body["output"]; got != "世界，你好" {
		t.Fatalf("generation.output = %v", got)
	}
	if got := trace.Body["input"]; got != "你好" {
		t.Fatalf("trace.input = %v, want 你好（用户真正说的话）", got)
	}
	if got := trace.Body["output"]; got != "世界，你好" {
		t.Fatalf("trace.output = %v", got)
	}

	// 原始请求体只抽一份，挂在卡片 metadata 上。
	traceMeta, ok := trace.Body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("trace.metadata 缺失")
	}
	if raw, ok := traceMeta["request_body"].(string); !ok || !strings.Contains(raw, `"system"`) {
		t.Fatalf("trace.metadata.request_body 未保留原始请求体: %v", traceMeta["request_body"])
	}
	// JSON 往返后数字是 float64。
	if turns, ok := meta["user_prompt_turns"].(float64); !ok || turns != 1 {
		t.Fatalf("metadata.user_prompt_turns = %v", meta["user_prompt_turns"])
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
	gen := events[0]
	if gen.Type != eventTypeGenerationCreate {
		t.Fatalf("事件[0] 应为 generation，实际 %q", gen.Type)
	}
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
		t.Fatalf("退出冲刷的事件数 = %d, want 10 (5 调用 → 5 generation + 5 卡片)", total)
	}
	if client.Sent() != 10 {
		t.Fatalf("Sent() = %d, want 10（按事件计）", client.Sent())
	}
}

// TestSendSplitsOnByteBudget 覆盖防 413 的分包：
// Langfuse 单请求硬限 4.5MB，一批事件总体积超预算时必须切成多次 POST。
func TestSendSplitsOnByteBudget(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 64, FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	// 6 个各 1MB 的事件：单批最多只能装下 2 个（3MB 预算）。
	evs := make([]event, 0, 6)
	for i := 0; i < 6; i++ {
		evs = append(evs, event{
			ID:        fmt.Sprintf("ev-%d", i),
			Type:      eventTypeGenerationCreate,
			Timestamp: "2026-11-05T12:00:00.000Z",
			Body:      map[string]any{"output": strings.Repeat("x", 1<<20)},
		})
	}
	client.send(evs)

	if got := rec.hits.Load(); got < 2 {
		t.Fatalf("POST 次数 = %d，6MB 事件必须被拆包", got)
	}
	total := 0
	for _, b := range rec.all() {
		total += len(b.Batch)
		if len(b.Batch) > 2 {
			t.Fatalf("单批事件数 = %d，超过 3MB 预算", len(b.Batch))
		}
	}
	if total != 6 {
		t.Fatalf("总事件数 = %d, want 6（拆包不能丢事件）", total)
	}
}

// TestWorkerFlushesWhenPendingBytesExceedBudget 覆盖事实采集场景下的提前冲刷：
// 事件条数没到 batch_size，但体积已经超预算时，不能继续攒。
func TestWorkerFlushesWhenPendingBytesExceedBudget(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 16, BatchSize: 64, MaxBatchBytes: 8 * 1024,
		FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	body := strings.Repeat("x", 4*1024)
	for i := 0; i < 4; i++ {
		client.Observe(Call{UserID: int64(i), Model: "m", Output: body, StartedAt: time.Now()})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Close(ctx)

	// 4 条各约 4.6KB 的 generation 事件 → 至少要在中途冲刷一次。
	if got := rec.hits.Load(); got < 2 {
		t.Fatalf("POST 次数 = %d，字节预算没有触发提前冲刷", got)
	}
}

// TestSendSplitsLargeEventsByBudget 覆盖"单条就能顶满预算"的情形：
// 3 条各 2MB 的事件不能塞进同一次 POST，必须逐条切开。
func TestSendSplitsLargeEventsByBudget(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 64, FlushInterval: time.Hour, Timeout: 5 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	evs := make([]event, 0, 3)
	for i := 0; i < 3; i++ {
		evs = append(evs, event{
			ID:        fmt.Sprintf("ev-%d", i),
			Type:      eventTypeGenerationCreate,
			Timestamp: "2026-11-05T12:00:00.000Z",
			Body:      map[string]any{"output": strings.Repeat("x", 2<<20)},
		})
	}
	client.send(evs)

	if got := rec.hits.Load(); got < 3 {
		t.Fatalf("POST 次数 = %d，2MB 事件必须一条一批", got)
	}
	total := 0
	for _, b := range rec.all() {
		total += len(b.Batch)
	}
	if total != 3 {
		t.Fatalf("总事件数 = %d, want 3", total)
	}
}

// TestOversizeSingleEventIsDroppedNotSent 覆盖单条事件本身就超上限的情形：
// 发出去只会拿 413，必须直接丢弃并计数，而不是让整批失败。
func TestOversizeSingleEventIsDroppedNotSent(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 8, FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	// `&` 在 JSON 里编成 `\u0026`：1MB 原始字节 → 约 6MB 载荷，超过 ingestion 硬限。
	client.send([]event{{
		ID: "ev-huge", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z",
		Body: map[string]any{"output": strings.Repeat("&", 1<<20)},
	}})

	if got := rec.hits.Load(); got != 0 {
		t.Fatalf("超限事件不该被发出去，实际 POST %d 次", got)
	}
	if client.Dropped() == 0 {
		t.Fatal("超限事件必须计入丢弃，不能静默消失")
	}
}

// TestPoisonEventIsIsolated 覆盖 4xx 隔离：
// 一条事件被服务端判为非法时，不能连坐同批其它事件（尤其是卡片）。
func TestPoisonEventIsIsolated(t *testing.T) {
	var mu sync.Mutex
	accepted := make([]string, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b capturedBatch
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, e := range b.Batch {
			if e.ID == "ev-poison" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		mu.Lock()
		for _, e := range b.Batch {
			accepted = append(accepted, e.ID)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 8, FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	client.send([]event{
		{ID: "ev-a", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
		{ID: "ev-poison", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
		{ID: "ev-b", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
		{ID: "ev-c", Type: eventTypeTraceCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(accepted) != 3 {
		t.Fatalf("成功事件数 = %v, want 3（毒事件只应拉掉它自己）", accepted)
	}
	for _, id := range accepted {
		if id == "ev-poison" {
			t.Fatal("毒事件不该被算作成功")
		}
	}
}

// TestPartialIngestionFailureIsCounted 覆盖 207 的部分失败：
// Langfuse 的逐事件拒绝是 207 响应体里的 errors[]，不是 4xx。
// 只看状态码会让 Sent/Failed/Dropped 三个自检指标一起说谎。
func TestPartialIngestionFailureIsCounted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"successes":[{"id":"ev-a","status":201}],"errors":[{"id":"ev-b","status":400,"message":"bad shape"}]}`))
	}))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		QueueSize: 8, BatchSize: 8, FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	client.send([]event{
		{ID: "ev-a", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
		{ID: "ev-b", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z", Body: map[string]any{}},
	})

	if got := client.Sent(); got != 1 {
		t.Fatalf("Sent() = %d, want 1（207 里只成功了一条）", got)
	}
	if got := client.Failed(); got != 1 {
		t.Fatalf("Failed() = %d, want 1", got)
	}
	if got := client.Dropped(); got != 1 {
		t.Fatalf("Dropped() = %d, want 1（被拒的事件要计入丢弃）", got)
	}
}

// TestRequestTimeoutShrinksOnExit 覆盖退出预算：
// main 只等几秒，单请求默认却允许 10s；不收口的话卡片还没发出去进程就没了。
func TestRequestTimeoutShrinksOnExit(t *testing.T) {
	client := NewClient(Options{
		Host: "http://127.0.0.1:1", PublicKey: "p", SecretKey: "s",
		QueueSize: 2, BatchSize: 2, FlushInterval: time.Hour, Timeout: 10 * time.Second,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()

	if got := client.requestTimeout(); got != 10*time.Second {
		t.Fatalf("非退出路径超时 = %s, want 10s", got)
	}
	client.exitDeadline.Store(time.Now().Add(500 * time.Millisecond).UnixNano())
	if got := client.requestTimeout(); got > time.Second {
		t.Fatalf("退出路径超时 = %s，没有按剩余预算收口", got)
	}
	client.exitDeadline.Store(time.Now().Add(-time.Second).UnixNano())
	if got := client.requestTimeout(); got > 0 {
		t.Fatalf("预算耗尽后超时 = %s, want <=0（不再发）", got)
	}
}

// TestMultiStepTurnProducesOneCard 是端到端验收：
// 同一轮 4 次上游调用，经真实 Client → ingestion 后必须只出现 1 条 trace，
// 且 4 条 generation 都挂在它下面、卡片正文取首步的用户消息与末条非空回答。
func TestMultiStepTurnProducesOneCard(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	client := NewClient(Options{
		Host: srv.URL, PublicKey: "p", SecretKey: "s",
		// 每条事件单独成批：这样测试能通过 Batches() 精确知道 worker 处理到哪一步，
		// 不用靠 sleep 猜（关得太早会让这一轮在退出路径上合理地裂成两张卡）。
		QueueSize: 32, BatchSize: 1, FlushInterval: time.Hour, Timeout: 3 * time.Second,
	})

	base := time.Now().Add(-20 * time.Minute)
	outputs := []string{"先看仓库结构", "", "改完了", ""}
	for i, out := range outputs {
		client.Observe(Call{
			UserID: 55, SessionID: "sess-e2e", UserTurns: 2,
			FirstUserPrompt: "把观测改成一轮一张卡", RequestFingerprint: fmt.Sprintf("fp-%d", i),
			Model: "gpt-6", Endpoint: "/v1/responses", Status: "ok",
			StartedAt: base.Add(time.Duration(i) * time.Minute), Duration: 5 * time.Second,
			Cost: 0.25, Currency: "USD", InputTokens: 10, OutputTokens: 5,
			InputUser: "把观测改成一轮一张卡", Input: `{"messages":[{"role":"user","content":"把观测改成一轮一张卡"}]}`,
			Output: out, OutputChars: len(out),
		})
	}

	// 等 4 条 generation 真的发出去：那才代表 worker 已经把这一轮的步骤都吸收了。
	deadline := time.Now().Add(3 * time.Second)
	for client.Batches() < uint64(len(outputs)) && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := client.Batches(); got < uint64(len(outputs)) {
		t.Fatalf("只有 %d 条 generation 被发送，worker 没吃完全部步骤", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}

	traceCount, genCount := 0, 0
	var traceBody, genSample map[string]any
	for _, b := range rec.all() {
		for _, e := range b.Batch {
			switch e.Type {
			case eventTypeTraceCreate:
				traceCount++
				traceBody = e.Body
			case eventTypeGenerationCreate:
				genCount++
				genSample = e.Body
			}
		}
	}

	if traceCount != 1 {
		for _, b := range rec.all() {
			for _, e := range b.Batch {
				t.Logf("EV type=%s id=%v traceId=%v", e.Type, e.Body["id"], e.Body["traceId"])
			}
		}
		t.Fatalf("卡片数 = %d, want 1（4 步必须收敛成 1 条 trace）", traceCount)
	}
	if genCount != 4 {
		t.Fatalf("generation 数 = %d, want 4", genCount)
	}
	if traceBody["input"] != "把观测改成一轮一张卡" {
		t.Fatalf("卡片 input = %v", traceBody["input"])
	}
	if want := "先看仓库结构\n\n改完了"; traceBody["output"] != want {
		t.Fatalf("卡片 output = %q, want %q（该轮全部非空文本）", traceBody["output"], want)
	}
	if traceBody["sessionId"] != "sess-e2e" {
		t.Fatalf("卡片 sessionId = %v", traceBody["sessionId"])
	}
	if genSample["traceId"] != traceBody["id"] {
		t.Fatalf("generation 没挂到卡片上：%v vs %v", genSample["traceId"], traceBody["id"])
	}

}
