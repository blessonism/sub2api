package service

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/observability/capture"
	"github.com/Wei-Shaw/sub2api/internal/observability/langfuse"
)

// captureSink 记录投递到观测层的 Call，供断言字段映射。
type captureSink struct {
	mu    sync.Mutex
	calls []langfuse.Call
}

func (c *captureSink) Observe(call langfuse.Call) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *captureSink) snapshot() []langfuse.Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]langfuse.Call(nil), c.calls...)
}

func installCaptureSink(t *testing.T) *captureSink {
	t.Helper()
	sink := &captureSink{}
	langfuse.SetDefault(sink)
	t.Cleanup(func() { langfuse.SetDefault(nil) })
	return sink
}

func obsStrPtr(v string) *string   { return &v }
func obsF64Ptr(v float64) *float64 { return &v }
func obsI64Ptr(v int64) *int64     { return &v }
func obsIntPtr(v int) *int         { return &v }

// TestObserveModelCallMapsUsageLog 覆盖观测层字段映射：检索键与用量桶。
func TestObserveModelCallMapsUsageLog(t *testing.T) {
	sink := installCaptureSink(t)

	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	usageLog := &UsageLog{
		UserID:                42,
		APIKeyID:              9,
		AccountID:             7,
		RequestID:             "req-1",
		Model:                 "gpt-5.2",
		InboundEndpoint:       obsStrPtr("/v1/chat/completions"),
		BillingMode:           obsStrPtr("token"),
		ServiceTier:           obsStrPtr("priority"),
		GroupID:               obsI64Ptr(3),
		RateMultiplier:        0.8,
		VisibleRateMultiplier: obsF64Ptr(1.0),
		AccountRateMultiplier: obsF64Ptr(1.2),
		BillingType:           BillingTypeBalance,
		Stream:                true,
		DurationMs:            obsIntPtr(1500),
		SessionID:             obsStrPtr("sess-abc"),
		InputTokens:           100,
		OutputTokens:          20,
		CacheReadTokens:       30,
		CacheCreationTokens:   5,
		CreatedAt:             createdAt,
	}

	observeModelCall(context.Background(), usageLog, 0.1234, observeStatusOK, "")

	calls := sink.snapshot()
	if len(calls) != 1 {
		t.Fatalf("投递条数 = %d, want 1（一次调用 = 一条 trace + 一条 generation）", len(calls))
	}
	got := calls[0]
	if got.UserID != 42 || got.SessionID != "sess-abc" || got.RequestID != "req-1" {
		t.Fatalf("检索键 = %d/%q/%q", got.UserID, got.SessionID, got.RequestID)
	}
	if got.Model != "gpt-5.2" || got.Endpoint != "/v1/chat/completions" {
		t.Fatalf("模型/端点 = %q/%q", got.Model, got.Endpoint)
	}
	// cost 必须原样透传实扣值，不得由 token 重算。
	if got.Cost != 0.1234 {
		t.Fatalf("Cost = %v, want 0.1234（实扣）", got.Cost)
	}
	if got.InputTokens != 100 || got.OutputTokens != 20 || got.CacheReadTokens != 30 || got.CacheWriteTokens != 5 {
		t.Fatalf("用量桶 = %d/%d/%d/%d", got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens)
	}
	if got.Duration != 1500*time.Millisecond {
		t.Fatalf("Duration = %s, want 1.5s", got.Duration)
	}
	if !got.StartedAt.Equal(createdAt.Add(-1500 * time.Millisecond)) {
		t.Fatalf("StartedAt = %s, want %s", got.StartedAt, createdAt.Add(-1500*time.Millisecond))
	}
	if got.BillingType != "balance" || got.BillingMode != "token" || got.ServiceTier != "priority" {
		t.Fatalf("计费口径 = %q/%q/%q", got.BillingType, got.BillingMode, got.ServiceTier)
	}
	if !got.Stream || got.GroupID != 3 || got.AccountID != 7 || got.APIKeyID != 9 {
		t.Fatalf("归属字段 = %v/%d/%d/%d", got.Stream, got.GroupID, got.AccountID, got.APIKeyID)
	}
}

// TestObserveModelCallCarriesCapturedBody 覆盖正文层接线：
// ctx 上装有 recorder 时，投递的 Call 必须带上请求正文与解析后的响应正文。
func TestObserveModelCallCarriesCapturedBody(t *testing.T) {
	sink := installCaptureSink(t)

	rec := capture.New(capture.Options{Prompt: true, Response: true, MaxBytes: 4096})
	rec.SetRequest(`{"model":"gpt-5.2","messages":[{"role":"user","content":"解释一下这个函数"}]}`)
	rec.AppendResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"它做的事是\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"解析参数\"}}]}\n"))

	ctx := capture.With(context.Background(), rec)
	observeModelCall(ctx, &UsageLog{UserID: 11, CreatedAt: time.Now()}, 0.5, observeStatusOK, "")

	calls := sink.snapshot()
	if len(calls) != 1 {
		t.Fatalf("投递条数 = %d, want 1", len(calls))
	}
	if !strings.Contains(calls[0].Input, "解释一下这个函数") {
		t.Fatalf("input 未携带请求正文: %q", calls[0].Input)
	}
	if calls[0].Output != "它做的事是解析参数" {
		t.Fatalf("output = %q, want 它做的事是解析参数", calls[0].Output)
	}
}

// TestObserveModelCallWithoutRecorderHasNoBody 覆盖未安装采集时正文为空。
func TestObserveModelCallWithoutRecorderHasNoBody(t *testing.T) {
	sink := installCaptureSink(t)
	observeModelCall(context.Background(), &UsageLog{UserID: 12, CreatedAt: time.Now()}, 0.5, observeStatusOK, "")
	calls := sink.snapshot()
	if len(calls) != 1 {
		t.Fatalf("投递条数 = %d, want 1", len(calls))
	}
	if calls[0].Input != "" || calls[0].Output != "" {
		t.Fatalf("未采集时不得带正文: input=%q output=%q", calls[0].Input, calls[0].Output)
	}
}

// TestObserveModelCallDisabledIsNoop 覆盖关闭态零投递。
func TestObserveModelCallDisabledIsNoop(t *testing.T) {
	langfuse.SetDefault(nil)
	if langfuse.Active() {
		t.Fatal("未装配时 Active() 必须为 false")
	}
	// 不 panic、不投递。
	observeModelCall(context.Background(), &UsageLog{UserID: 1}, 9.9, observeStatusOK, "")
}

// TestObserveModelCallNilUsageLog 覆盖防御性分支。
func TestObserveModelCallNilUsageLog(t *testing.T) {
	sink := installCaptureSink(t)
	observeModelCall(context.Background(), nil, 1, observeStatusOK, "")
	if len(sink.snapshot()) != 0 {
		t.Fatal("nil usageLog 不得投递")
	}
}

// TestRecordUsagePassesActualCostToObservability 是取数口径的源码契约：
// 两条计费入口（OpenAI 网关 / 通用网关）的成功路径都必须把落库后的
// usageLog.ActualCost 传给观测层，不得从 token 或余额差反推。
func TestRecordUsagePassesActualCostToObservability(t *testing.T) {
	cases := []struct {
		file string
		// sites 是该文件里 observeModelCall 的期望调用点数。
		sites int
	}{
		{"openai_gateway_usage.go", 3},
		{"gateway_usage_billing.go", 3},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tc.file, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}

			type callSite struct {
				args []ast.Expr
				line int
			}
			var sites []callSite

			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, ok := call.Fun.(*ast.Ident)
				if !ok || ident.Name != "observeModelCall" {
					return true
				}
				sites = append(sites, callSite{args: call.Args, line: fset.Position(call.Pos()).Line})
				return true
			})

			if len(sites) != tc.sites {
				t.Fatalf("observeModelCall 调用点 = %d, want %d（simple 模式 / 扣费失败 / 成功）",
					len(sites), tc.sites)
			}

			actualCostPassed := 0
			for _, site := range sites {
				if len(site.args) < 3 {
					t.Fatalf("第 %d 行的 observeModelCall 参数不足", site.line)
				}
				for _, arg := range site.args {
					sel, ok := arg.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					base, ok := sel.X.(*ast.Ident)
					if !ok {
						continue
					}
					if base.Name == "usageLog" && sel.Sel.Name == "ActualCost" {
						actualCostPassed++
						break
					}
				}
			}
			if actualCostPassed != 1 {
				t.Fatalf("以 usageLog.ActualCost 作为实扣入参的调用点 = %d, want 1", actualCostPassed)
			}
		})
	}
}

// TestBillingTypeLabel 覆盖计费类型标签映射。
func TestBillingTypeLabel(t *testing.T) {
	if got := billingTypeLabel(BillingTypeBalance); got != "balance" {
		t.Fatalf("balance 标签 = %q", got)
	}
	if got := billingTypeLabel(BillingTypeSubscription); got != "subscription" {
		t.Fatalf("subscription 标签 = %q", got)
	}
	if got := billingTypeLabel(9); got != "" {
		t.Fatalf("未知标签 = %q, want 空", got)
	}
}

// TestObserveModelCallCarriesSystemToolsAndStepInput 覆盖装配层：
// recorder 抽到的 system / tools / 工具结果必须真的传进 Call。
//
// 0797e36b7 声明了 stepInput / stepInputKind 两个变量，却没从 recorder 取值 ——
// Call.StepInput 恒为空，于每个步骤面板显示的都是同一句用户消息，
// 「步骤 input 应为工具结果」那版修复在线上从未生效。
// 那个提交的单测只覆盖 langfuse 包内的 stepInput() 函数（用构造的 Call 调用），
// 装配这一环没有测试兜住，所以漏了。
func TestObserveModelCallCarriesSystemToolsAndStepInput(t *testing.T) {
	sink := installCaptureSink(t)

	rec := capture.New(capture.Options{Prompt: true, Response: true, MaxBytes: 64 << 10})
	rec.SetRequest(`{"model":"gpt-5.2",
		"messages":[
			{"role":"system","content":"你是严谨的工程师"},
			{"role":"user","content":"读一下 a.go"},
			{"role":"tool","content":"package a\nfunc Run() {}"}],
		"tools":[{"type":"function","function":{"name":"read_file"}}]}`)

	ctx := capture.With(context.Background(), rec)
	observeModelCall(ctx, &UsageLog{UserID: 13, CreatedAt: time.Now()}, 0.1, observeStatusOK, "")

	calls := sink.snapshot()
	if len(calls) != 1 {
		t.Fatalf("投递条数 = %d, want 1", len(calls))
	}
	got := calls[0]
	if got.SystemPrompt != "你是严谨的工程师" {
		t.Fatalf("SystemPrompt = %q，装配层没把 system 传下来", got.SystemPrompt)
	}
	if !strings.Contains(got.ToolsDefinition, "read_file") {
		t.Fatalf("ToolsDefinition = %q，装配层没把工具定义传下来", got.ToolsDefinition)
	}
	if !strings.Contains(got.StepInput, "func Run()") {
		t.Fatalf("StepInput = %q，装配层没把这一步的工具结果传下来", got.StepInput)
	}
	if got.StepInputKind != "tool_result" {
		t.Fatalf("StepInputKind = %q, want tool_result", got.StepInputKind)
	}
	// system 与工具定义不是「用户说的话」，不能混进 InputUser。
	if strings.Contains(got.InputUser, "你是严谨的工程师") {
		t.Fatalf("system 混进了用户消息：%q", got.InputUser)
	}
}
