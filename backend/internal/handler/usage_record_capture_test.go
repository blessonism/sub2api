package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/observability/capture"
	"github.com/Wei-Shaw/sub2api/internal/observability/langfuse"
)

// TestUsageRecordContextCarriesCapture 是正文层的保命约束：
// 计费任务跑在异步 worker 的新 ctx 上，若正文采集器不随任务转移，
// 观测层就永远收不到提示词与响应正文。
func TestUsageRecordContextCarriesCapture(t *testing.T) {
	rec := capture.New(capture.Options{Prompt: true, Response: true})
	rec.SetRequest(`{"messages":[{"role":"user","content":"survive"}]}`)

	parent := capture.With(context.Background(), rec)
	workerCtx := usageRecordContext(parent, context.Background())

	got := capture.From(workerCtx)
	if got == nil {
		t.Fatal("计费任务的 ctx 上没有采集器，正文会丢")
	}
	if got.RequestText() == "" {
		t.Fatal("采集器内容没有随 ctx 转移")
	}
}

// TestUsageRecordContextWithoutCaptureStaysClean 覆盖未安装采集器的常规路径。
func TestUsageRecordContextWithoutCaptureStaysClean(t *testing.T) {
	workerCtx := usageRecordContext(context.Background(), context.Background())
	if capture.From(workerCtx) != nil {
		t.Fatal("未安装采集器时不得凭空产生一个")
	}
}

// TestOpenAIUsageTaskFallbackCarriesCapture 覆盖 worker 池未注入/已停止时的同步回退：
// 回退路径用的是新 base ctx，不搬运采集器就会丢正文。
func TestOpenAIUsageTaskFallbackCarriesCapture(t *testing.T) {
	rec := capture.New(capture.Options{Prompt: true, Response: true})
	rec.SetRequest(`{"messages":[{"role":"user","content":"fallback"}]}`)
	parent := capture.With(context.Background(), rec)

	h := &OpenAIGatewayHandler{}
	var got *capture.Recorder
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		got = capture.From(ctx)
	})
	if got == nil {
		t.Fatal("OpenAI 同步回退路径丢了采集器")
	}
}

// TestAnthropicUsageTaskFallbackCarriesCapture 同上，覆盖 /v1/messages 路径。
func TestAnthropicUsageTaskFallbackCarriesCapture(t *testing.T) {
	rec := capture.New(capture.Options{Prompt: true, Response: true})
	rec.SetRequest(`{"messages":[{"role":"user","content":"fallback"}]}`)
	parent := capture.With(context.Background(), rec)

	h := &GatewayHandler{}
	var got *capture.Recorder
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		got = capture.From(ctx)
	})
	if got == nil {
		t.Fatal("Anthropic 同步回退路径丢了采集器")
	}
}

// TestCaptureOptionsGatedBySink 覆盖：观测层未装配时不得缓冲正文。
func TestCaptureOptionsGatedBySink(t *testing.T) {
	cfg := &config.Config{}
	cfg.Observability.Langfuse = config.LangfuseConfig{
		Enabled: true, Host: "http://x", PublicKey: "pk", SecretKey: "sk",
		CapturePrompt: true, CaptureResponse: true, CaptureMaxBytes: 1024,
	}
	// 未调用 SetDefault / 已置为 nil 时 Active()=false。
	langfuse.SetDefault(nil)
	if opts := captureOptions(cfg); opts.Enabled() {
		t.Fatal("观测层不可用时不得开启正文采集")
	}

	// 装配真实客户端后按配置开启。
	client := langfuse.NewClient(langfuse.Options{Host: "http://127.0.0.1:1", PublicKey: "pk", SecretKey: "sk"})
	langfuse.SetDefault(client)
	defer func() {
		_ = client.Close(context.Background())
		langfuse.SetDefault(nil)
	}()
	opts := captureOptions(cfg)
	if !opts.Prompt || !opts.Response || opts.MaxBytes != 1024 {
		t.Fatalf("采集项未按配置透传: %+v", opts)
	}
}
