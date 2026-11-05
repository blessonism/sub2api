package langfuse

import (
	"strings"
	"testing"
	"time"

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
	if strings.Contains(name, "th-") == false {
		t.Fatalf("trace 名必须带线程，实际 = %q", name)
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
