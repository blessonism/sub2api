package langfuse

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/observability/sessiongroup"
)

// traceBodyOf 取第 n 条 Call 展开出的 trace-create body。
func traceBodyOf(t *testing.T, b batch, n int) map[string]any {
	t.Helper()
	idx := n * 2
	if idx >= len(b.Batch) {
		t.Fatalf("batch 只有 %d 条事件，取不到第 %d 条 trace", len(b.Batch), n)
	}
	if b.Batch[idx].Type != eventTypeTraceCreate {
		t.Fatalf("event[%d].type = %q", idx, b.Batch[idx].Type)
	}
	return b.Batch[idx].Body
}

func metaOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	meta, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata 缺失或类型不对：%T", body["metadata"])
	}
	return meta
}

func sampleCall(sessionID, firstUser, fp string) Call {
	return Call{
		UserID:             120,
		APIKeyID:           7,
		SessionID:          sessionID,
		FirstUserPrompt:    firstUser,
		RequestFingerprint: fp,
		Model:              "gpt-6-astra",
		Endpoint:           "/v1/responses",
		Status:             "ok",
		StartedAt:          time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC),
		Input:              "raw-body",
		InputUser:          firstUser,
	}
}

func TestTraceCarriesClientSessionIDAndThreadMeta(t *testing.T) {
	sessiongroup.Reset()
	body := traceBodyOf(t, buildBatch([]Call{sampleCall("01a1ab68-7e7d", "把会话聚合补上", "fp-1")}, ""), 0)

	if got := body["sessionId"]; got != "01a1ab68-7e7d" {
		t.Fatalf("sessionId = %v", got)
	}
	meta := metaOf(t, body)
	if meta["session_source"] != sessiongroup.SourceClient {
		t.Fatalf("session_source = %v", meta["session_source"])
	}
	if meta["session_title"] != "把会话聚合补上" {
		t.Fatalf("session_title = %v", meta["session_title"])
	}
	if meta["thread_id"] == nil || meta["thread_id"] == "" {
		t.Fatalf("thread_id 缺失：%v", meta)
	}
	if meta["thread_step"] != 1 || meta["step_attempt"] != 1 {
		t.Fatalf("thread_step/step_attempt = %v/%v", meta["thread_step"], meta["step_attempt"])
	}
}

func TestTraceDerivesSessionWhenClientSendsNone(t *testing.T) {
	sessiongroup.Reset()
	body := traceBodyOf(t, buildBatch([]Call{sampleCall("", "重构网关调度", "fp-1")}, ""), 0)

	sid, _ := body["sessionId"].(string)
	if !strings.HasPrefix(sid, "auto-") {
		t.Fatalf("无会话头时应落推导 id，实际 = %v", body["sessionId"])
	}
	if metaOf(t, body)["session_source"] != sessiongroup.SourceAuto {
		t.Fatalf("session_source = %v", metaOf(t, body)["session_source"])
	}
}

func TestTraceOmitsSessionWhenNothingToGroupBy(t *testing.T) {
	sessiongroup.Reset()
	body := traceBodyOf(t, buildBatch([]Call{sampleCall("", "", "")}, ""), 0)
	if _, ok := body["sessionId"]; ok {
		t.Fatalf("既无会话头也无提示词时不得落 sessionId：%v", body["sessionId"])
	}
	meta := metaOf(t, body)
	if _, ok := meta["thread_id"]; ok {
		t.Fatalf("无锚点时不该有 thread_id：%v", meta)
	}
}

func TestTraceRetryCollapsesToSameStep(t *testing.T) {
	sessiongroup.Reset()
	calls := []Call{
		sampleCall("sess-x", "同一句话", "fp-same"),
		sampleCall("sess-x", "同一句话", "fp-same"),
		sampleCall("sess-x", "同一句话", "fp-next"),
	}
	b := buildBatch(calls, "")
	first := metaOf(t, traceBodyOf(t, b, 0))
	retry := metaOf(t, traceBodyOf(t, b, 1))
	next := metaOf(t, traceBodyOf(t, b, 2))

	if first["thread_step"] != 1 || first["step_attempt"] != 1 {
		t.Fatalf("首次 = %v/%v", first["thread_step"], first["step_attempt"])
	}
	if retry["thread_step"] != 1 || retry["step_attempt"] != 2 {
		t.Fatalf("重复请求体应折叠成同一步的第二次尝试：%v/%v", retry["thread_step"], retry["step_attempt"])
	}
	if next["thread_step"] != 2 || next["step_attempt"] != 1 {
		t.Fatalf("换了请求体应进入下一步：%v/%v", next["thread_step"], next["step_attempt"])
	}
}

func TestGenerationCarriesSessionMetaToo(t *testing.T) {
	sessiongroup.Reset()
	b := buildBatch([]Call{sampleCall("sess-y", "同一个会话", "fp")}, "")
	gen := b.Batch[1].Body
	if gen["traceId"] != b.Batch[0].Body["id"] {
		t.Fatalf("generation 必须挂在同一 trace 上")
	}
	if metaOf(t, gen)["session_title"] != "同一个会话" {
		t.Fatal("generation 侧也要带会话结构，UI 从任一侧都要能读")
	}
}

func TestTraceNameCarriesThreadAndStep(t *testing.T) {
	sessiongroup.Reset()
	b := buildBatch([]Call{
		sampleCall("sess-z", "把会话聚合补上", "fp-a"),
		sampleCall("sess-z", "把会话聚合补上", "fp-a"),
	}, "")
	first := b.Batch[0].Body
	second := b.Batch[2].Body

	name, _ := first["name"].(string)
	if !strings.HasPrefix(name, traceName) || !strings.Contains(name, "#1") {
		t.Fatalf("trace 名必须带步骤，实际 = %q", name)
	}
	if !regexp.MustCompile(` · [0-9a-f]{4} · #1$`).MatchString(name) {
		t.Fatalf("trace 名必须带短线程与步骤，实际 = %q", name)
	}
	if retryName, _ := second["name"].(string); !strings.Contains(retryName, "try2") {
		t.Fatalf("重试要标出来，实际 = %q", retryName)
	}

	tags, ok := first["tags"].([]string)
	if !ok || len(tags) != 1 || !strings.HasPrefix(tags[0], "thread:th-") {
		t.Fatalf("tags = %v", first["tags"])
	}
}

func TestTraceNameFallsBackWhenNoGroup(t *testing.T) {
	sessiongroup.Reset()
	body := traceBodyOf(t, buildBatch([]Call{sampleCall("", "", "")}, ""), 0)
	if body["name"] != traceName {
		t.Fatalf("无归组时 trace 名应保持原样：%v", body["name"])
	}
	if _, ok := body["tags"]; ok {
		t.Fatalf("无归组时不该有 tags：%v", body["tags"])
	}
}

func TestOutputCompletenessMetadata(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-out", "看看输出", "fp")
	call.Output = "完整答案"
	call.OutputChars = 4
	call.FinishReason = "length"
	call.RawResponseTruncated = true

	meta := metaOf(t, traceBodyOf(t, buildBatch([]Call{call}, ""), 0))
	if meta["output_chars"] != 4 {
		t.Fatalf("output_chars = %v", meta["output_chars"])
	}
	if meta["finish_reason"] != "length" {
		t.Fatalf("finish_reason = %v", meta["finish_reason"])
	}
	if meta["raw_stream_truncated"] != true {
		t.Fatalf("raw_stream_truncated = %v", meta["raw_stream_truncated"])
	}
	// 关键：原始字节被截，但正文没触顶 → 不能出现 output_truncated。
	if _, ok := meta["output_truncated"]; ok {
		t.Fatalf("原始字节截断不得冒充正文截断：%v", meta["output_truncated"])
	}
}

func TestOutputTruncatedOnlyWhenTextHitsCap(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-out2", "看看输出", "fp")
	call.Output = "被切的答案"
	call.OutputChars = 5
	call.OutputTruncated = true

	meta := metaOf(t, traceBodyOf(t, buildBatch([]Call{call}, ""), 0))
	if meta["output_truncated"] != true {
		t.Fatalf("正文触顶必须标记：%v", meta)
	}
}

func TestClampCapturedKeepsPayloadUnderIngestionLimit(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-big", strings.Repeat("问", 3*1024*1024/3), "fp")
	call.Output = strings.Repeat("答", 3*1024*1024/3)
	call.Input = strings.Repeat("b", 512*1024)
	clamped := clampCaptured(call)

	total := len(clamped.Input) + len(clamped.InputUser) + len(clamped.Output)
	if total > maxCapturedPerCall+(1<<10) {
		t.Fatalf("单条调用正文 %d 字节，超过 %d 的载荷上限", total, maxCapturedPerCall)
	}
	if !clamped.OutputTruncated || !clamped.PromptTruncated || !clamped.RawRequestTruncated {
		t.Fatalf("截断必须分侧打标记：out=%v prompt=%v raw=%v",
			clamped.OutputTruncated, clamped.PromptTruncated, clamped.RawRequestTruncated)
	}
	if !strings.Contains(clamped.InputUser, "前文已省略") {
		t.Fatal("用户 prompt 截断应保留尾部并带省略标记")
	}
	if !utf8.ValidString(clamped.InputUser) || !utf8.ValidString(clamped.Output) {
		t.Fatal("截断后必须是合法 UTF-8")
	}
}

func TestClampCapturedLeavesSmallCallsAlone(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-small", "普通提问", "fp")
	call.Output = "普通回答"
	clamped := clampCaptured(call)
	if clamped.InputUser != call.InputUser || clamped.Output != call.Output ||
		clamped.PromptTruncated || clamped.OutputTruncated || clamped.RawRequestTruncated {
		t.Fatalf("小载荷不该被动过：%+v", clamped)
	}
}

// TestClampCapturedWorstCaseStaysUnderPayloadLimit 覆盖 review 发现的预算漏洞：
// 原始样本 2MB + 输出 2MB 时，旧算法会把合计推到 3.25MB（载荷 6.5MB → 413）。
func TestClampCapturedWorstCaseStaysUnderPayloadLimit(t *testing.T) {
	call := Call{
		Input:     strings.Repeat("b", 2<<20),
		Output:    strings.Repeat("答", (2<<20)/3),
		InputUser: strings.Repeat("问", (2<<20)/3),
	}
	clamped := clampCaptured(call)
	total := len(clamped.Input) + len(clamped.InputUser) + len(clamped.Output)
	if total > maxCapturedPerCall {
		t.Fatalf("合计 %d 字节超过 %d，载荷会被 ingestion 拒掉", total, maxCapturedPerCall)
	}
	if !clamped.RawRequestTruncated {
		t.Fatal("原始样本超 256KB 应标记 raw_request_truncated")
	}
}

func TestTruncationFlagsAreSeparated(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-trunc", "看一下", "fp")
	call.RawRequestTruncated = true
	call.Output = "答案"
	meta := metaOf(t, traceBodyOf(t, buildBatch([]Call{call}, ""), 0))
	if meta["raw_request_truncated"] != true {
		t.Fatalf("raw_request_truncated 缺失：%v", meta)
	}
	if _, ok := meta["user_prompt_truncated"]; ok {
		t.Fatal("原始样本被截不能冒充用户消息被截")
	}

	sessiongroup.Reset()
	call2 := sampleCall("sess-trunc2", "看一下", "fp")
	call2.PromptTruncated = true
	call2.Output = "答案"
	meta2 := metaOf(t, traceBodyOf(t, buildBatch([]Call{call2}, ""), 0))
	if meta2["user_prompt_truncated"] != true {
		t.Fatalf("user_prompt_truncated 缺失：%v", meta2)
	}
	if _, ok := meta2["raw_request_truncated"]; ok {
		t.Fatal("用户消息被截不能冒充原始样本被截")
	}
}

func TestPromptFallbackIsMarked(t *testing.T) {
	sessiongroup.Reset()
	call := sampleCall("sess-fb", "摘要兜底", "fp")
	call.PromptFallback = true
	call.PromptStatus = "no_user_text"
	meta := metaOf(t, traceBodyOf(t, buildBatch([]Call{call}, ""), 0))
	if meta["user_prompt_fallback"] != true || meta["user_prompt_status"] != "no_user_text" {
		t.Fatalf("兜底标记缺失：%v", meta)
	}
}
