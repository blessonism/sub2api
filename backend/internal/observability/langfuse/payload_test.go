package langfuse

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func sampleCall(sessionID, firstUser, fp string) Call {
	return Call{
		UserID:             120,
		APIKeyID:           7,
		SessionID:          sessionID,
		FirstUserPrompt:    firstUser,
		RequestFingerprint: fp,
		UserTurns:          1,
		Model:              "gpt-6-astra",
		Endpoint:           "/v1/responses",
		Status:             "ok",
		StartedAt:          time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC),
		Input:              "raw-body",
		InputUser:          firstUser,
	}
}

func TestClampCapturedKeepsPayloadUnderIngestionLimit(t *testing.T) {
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
	call := sampleCall("sess-small", "普通提问", "fp")
	call.Output = "普通回答"
	clamped := clampCaptured(call)
	if clamped.InputUser != call.InputUser || clamped.Output != call.Output ||
		clamped.PromptTruncated || clamped.OutputTruncated || clamped.RawRequestTruncated {
		t.Fatalf("小载荷不该被动过：%+v", clamped)
	}
}

// TestClampCapturedWorstCaseStaysUnderPayloadLimit 覆盖 review 发现的预算漏洞：
// 旧算法会把合计推到 3.25MB（载荷 6.5MB → 413）。
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

// TestTruncationFlagsAreSeparated：原始样本被截 与 用户消息被截 是两件事。
func TestTruncationFlagsAreSeparated(t *testing.T) {
	b := newTestBuffer(t)
	call := sampleCall("sess-trunc", "看一下", "fp")
	call.RawRequestTruncated = true
	call.Output = "答案"
	gens := b.add(call)
	meta := metaOfEvent(t, gens[0])
	if meta["raw_request_truncated"] != true {
		t.Fatalf("raw_request_truncated 缺失：%v", meta)
	}
	if _, ok := meta["user_prompt_truncated"]; ok {
		t.Fatal("原始样本被截不能冒充用户消息被截")
	}

	b2 := newTestBuffer(t)
	call2 := sampleCall("sess-trunc2", "看一下", "fp")
	call2.PromptTruncated = true
	call2.Output = "答案"
	meta2 := metaOfEvent(t, b2.add(call2)[0])
	if meta2["user_prompt_truncated"] != true {
		t.Fatalf("user_prompt_truncated 缺失：%v", meta2)
	}
	if _, ok := meta2["raw_request_truncated"]; ok {
		t.Fatal("用户消息被截不能冒充原始样本被截")
	}
}

// TestPromptFallbackIsMarked：整段没有用户文本时，卡片要如实标出来。
func TestPromptFallbackIsMarked(t *testing.T) {
	b := newTestBuffer(t)
	call := sampleCall("sess-fb", "摘要兜底", "fp")
	call.PromptFallback = true
	call.PromptStatus = "no_user_text"
	gens := b.add(call)
	if metaOfEvent(t, gens[0])["user_prompt_fallback"] != true {
		t.Fatalf("generation 侧缺少兜底标记：%v", metaOfEvent(t, gens[0]))
	}
	card := b.drain()[0]
	meta := metaOfEvent(t, card)
	if meta["user_prompt_fallback"] != true || meta["user_prompt_status"] != "no_user_text" {
		t.Fatalf("卡片侧兜底标记缺失：%v", meta)
	}
}

// TestOutputCompletenessMetadata：正文触顶与原始字节触顶必须分开标记。
func TestOutputCompletenessMetadata(t *testing.T) {
	b := newTestBuffer(t)
	call := sampleCall("sess-out", "看看输出", "fp")
	call.Output = "完整答案"
	call.OutputChars = 4
	call.FinishReason = "length"
	call.RawResponseTruncated = true

	meta := metaOfEvent(t, b.add(call)[0])
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
	b := newTestBuffer(t)
	call := sampleCall("sess-out2", "看看输出", "fp")
	call.Output = "被切的答案"
	call.OutputChars = 5
	call.OutputTruncated = true

	meta := metaOfEvent(t, b.add(call)[0])
	if meta["output_truncated"] != true {
		t.Fatalf("正文触顶必须标记：%v", meta)
	}
}

// TestRequestBodySampledOncePerTurn：原始请求体只是排障抽样，
// 一轮里每步都存会让卡片没有主次，也会把载荷撑大。
func TestRequestBodySampledOncePerTurn(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		c := stepCall("sess-body", "抽样", 1, "fp-"+string(rune('a'+i)), base.Add(time.Duration(i)*time.Minute))
		c.Input = "raw-" + string(rune('a'+i))
		b.add(c)
	}
	card := b.drain()[0]
	raw, _ := metaOfEvent(t, card)["request_body"].(string)
	if raw != "raw-a" {
		t.Fatalf("卡片 request_body = %q，应保留首步抽样", raw)
	}
}

// TestEventBytesCoversJSONEscaping 覆盖 review 发现的低估：
// encoding/json 默认把 `&` `<` `>` 编成 `\u00XX`（6 字节），控制字符同理。
// 按原始字节估算会让"估 1MB、实发 6MB"的批次撞上 4.5MB 上限，整批被 413 掉。
func TestEventBytesCoversJSONEscaping(t *testing.T) {
	cases := map[string]string{
		"ampersand": strings.Repeat("&", 4096),
		"angle":     strings.Repeat("<div>", 1024),
		"newline":   strings.Repeat("a\n", 2048),
		"control":   strings.Repeat("\x01", 4096),
		"utf8":      strings.Repeat("中文内容", 1024),
		// U+2028/U+2029 会被 encoding/json 编成 6 字节（JSONP 防护）。
		"u2028": strings.Repeat("\u2028", 4096),
		"u2029": strings.Repeat("\u2029", 4096),
		// 非法 UTF-8 每个坏字节变成一个 6 字节的 \ufffd。
		"invalid_utf8": strings.Repeat("\xff\xfe", 2048),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ev := event{
				ID: "ev-1", Type: eventTypeGenerationCreate, Timestamp: "2026-11-05T12:00:00.000Z",
				Body: map[string]any{"output": body},
			}
			real, err := json.Marshal(batch{Batch: []event{ev}})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if est := eventBytes(ev); est < len(real) {
				t.Fatalf("估算 %d 字节 < 实际 %d 字节：低估会让批次越过 ingestion 上限", est, len(real))
			}
		})
	}
}
