package capture

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"net/http/httptest"
)

func TestExtractOpenAIChatSSE(t *testing.T) {
	raw := []byte(`data: {"choices":[{"delta":{"role":"assistant"}}]}

data: {"choices":[{"delta":{"content":"你好"}}]}

data: {"choices":[{"delta":{"content":"，世界"}}]}

data: [DONE]
`)
	got, ok := ExtractAssistantText(raw)
	if !ok || got != "你好，世界" {
		t.Fatalf("got=%q ok=%v, want 你好，世界", got, ok)
	}
}

func TestExtractOpenAIResponsesSSE(t *testing.T) {
	raw := []byte(`event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"第一段"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"第二段"}
`)
	got, ok := ExtractAssistantText(raw)
	if !ok || got != "第一段第二段" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestExtractAnthropicSSE(t *testing.T) {
	raw := []byte(`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hel"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"lo"}}
`)
	got, ok := ExtractAssistantText(raw)
	if !ok || got != "Hello" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestExtractJSONVariants(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want string
	}{
		"openai chat": {`{"choices":[{"message":{"role":"assistant","content":"答案"}}]}`, "答案"},
		"anthropic":   {`{"content":[{"type":"text","text":"答案A"},{"type":"text","text":"答案B"}]}`, "答案A答案B"},
		"responses":   {`{"output":[{"type":"message","content":[{"type":"output_text","text":"responses 正文"}]}]}`, "responses 正文"},
		"output_text": {`{"output_text":"纯文本字段"}`, "纯文本字段"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := ExtractAssistantText([]byte(tc.raw))
			if !ok || got != tc.want {
				t.Fatalf("got=%q ok=%v, want %q", got, ok, tc.want)
			}
		})
	}
}

func TestExtractUnknownShapeFails(t *testing.T) {
	if _, ok := ExtractAssistantText([]byte(`{"foo":"bar"}`)); ok {
		t.Fatal("未知结构必须返回 ok=false，由调用方决定是否落原始字节")
	}
}

func TestRecorderTruncates(t *testing.T) {
	rec := New(Options{Prompt: true, Response: true, MaxBytes: 8})
	rec.SetRequest(strings.Repeat("a", 20))
	if got := len(rec.RequestText()); got != 8 {
		t.Fatalf("请求正文长度 = %d, want 8", got)
	}
	rec.AppendResponse([]byte(strings.Repeat("b", 20)))
	if !rec.RawTruncated() {
		t.Fatal("原始字节超限必须标记 raw truncated")
	}
	if rec.TextTruncated() {
		t.Fatal("原始字节被截不等于正文被截：非 SSE 正文没有增量文本")
	}
	if got := len(rec.ResponseRaw()); got != 8 {
		t.Fatalf("响应缓冲长度 = %d, want 8", got)
	}
}

func TestResponseTextFallsBackToRaw(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 1024})
	rec.AppendResponse([]byte("不是结构化响应，但必须留存"))
	if got := rec.ResponseText(); got != "不是结构化响应，但必须留存" {
		t.Fatalf("回退原文失败: %q", got)
	}
}

func TestTogglesOffRecordNothing(t *testing.T) {
	rec := New(Options{Prompt: false, Response: false})
	rec.SetRequest("不该被记录")
	rec.AppendResponse([]byte("也不该被记录"))
	if rec.RequestText() != "" || rec.ResponseText() != "" {
		t.Fatal("两项开关关闭时不得记录任何正文")
	}
	if rec.TextTruncated() || rec.RawTruncated() {
		t.Fatal("未采集时不应有截断标记")
	}
}

func TestInstallWrapsWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"x"}`))

	rec, restore := Install(c, Options{Prompt: true, Response: true, MaxBytes: 1024})
	if rec == nil {
		t.Fatal("开启采集时必须返回 recorder")
	}
	rec.SetRequest(`{"messages":[{"role":"user","content":"hi"}]}`)
	_, _ = c.Writer.WriteString(`data: {"choices":[{"delta":{"content":"ok"}}]}`)
	restore()

	if got := rec.RequestText(); !strings.Contains(got, `"hi"`) {
		t.Fatalf("请求正文未记录: %q", got)
	}
	if got := rec.ResponseText(); got != "ok" {
		t.Fatalf("响应正文 = %q, want ok", got)
	}
	// restore 之后 writer 应回到原实现，再写不再进入 recorder。
	_, _ = c.Writer.WriteString("after-restore")
	if strings.Contains(rec.ResponseText(), "after-restore") {
		t.Fatal("restore 后仍在校验 recorder，说明包装泄漏")
	}
}

func TestInstallDisabledReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	rec, restore := Install(c, Options{})
	defer restore()
	if rec != nil {
		t.Fatal("关闭态不得安装 recorder")
	}
	if From(c.Request.Context()) != nil {
		t.Fatal("关闭态不应把 recorder 挂到 ctx")
	}
}

// TestRequestTruncatedUsesConfiguredLimit 覆盖截断标记：上限来自配置，
// 不能用内置默认值判断，否则调小上限后标记永远不亮。
func TestRequestTruncatedUsesConfiguredLimit(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 16})
	rec.SetRequest(strings.Repeat("a", 64))
	if !rec.RequestTruncated() {
		t.Fatal("配置上限 16 字节、写入 64 字节时必须标记截断")
	}

	small := New(Options{Prompt: true, MaxBytes: 1024})
	small.SetRequest("短")
	if small.RequestTruncated() {
		t.Fatal("未超限不得标记截断")
	}
}

// TestTruncateKeepsValidUTF8 覆盖按字节截断时的 rune 边界回退。
func TestTruncateKeepsValidUTF8(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 7}) // 每个汉字 3 字节，7 会落在字符中间
	rec.SetRequest("一二三四五")
	got := rec.RequestText()
	if !utf8.ValidString(got) {
		t.Fatalf("截断产出了非法 UTF-8: %q", got)
	}
	if got != "一二" {
		t.Fatalf("got=%q, want 一二（回退到 rune 边界）", got)
	}
	if !rec.RequestTruncated() {
		t.Fatal("被截断却没标记")
	}
}

// TestIncrementalExtractionSurvivesLargePreamble 是这次线上问题的回归：
// Responses 流的 response.created 前导可能几十 KB，正文 delta 排在后面；
// 只靠末尾统一解析会在触发缓冲上限后永远抽不到正文。
func TestIncrementalExtractionSurvivesLargePreamble(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8 * 1024})

	// 4KB 前导，超过上限的一半但仍在小 buffer 内
	rec.AppendResponse([]byte(`event: response.created` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.created","response":{"id":"resp_1","padding":"` +
		strings.Repeat("x", 4096) + `"}}` + "\n"))

	// 正文 delta 逐块到达，且故意从半行切开
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.delta","del`))
	rec.AppendResponse([]byte(`ta":"你好，`))
	rec.AppendResponse([]byte(`我是 assistant"}` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.delta","delta":"。"}` + "\n"))

	if got := rec.ResponseText(); got != "你好，我是 assistant。" {
		t.Fatalf("正文 = %q, want 你好，我是 assistant。", got)
	}
}

// TestIncrementalExtractionHandlesChatSSE 覆盖 chat completions 增量。
func TestIncrementalExtractionHandlesChatSSE(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 4096})
	rec.AppendResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"第一段\"}}]}\n"))
	rec.AppendResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"第二段\"}}]}\n"))
	if got := rec.ResponseText(); got != "第一段第二段" {
		t.Fatalf("got %q", got)
	}
}

// TestJSONResponseStillParsed 覆盖非流式 JSON：没有 data: 行时走整体解析。
func TestJSONResponseStillParsed(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8192})
	rec.AppendResponse([]byte(`{"choices":[{"message":{"role":"assistant","content":"整体答案"}}]}`))
	if got := rec.ResponseText(); got != "整体答案" {
		t.Fatalf("got %q", got)
	}
}

// TestResponsesDoneEventCarriesText 覆盖 response.output_text.done 形态。
func TestResponsesDoneEventCarriesText(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8192})
	rec.AppendResponse([]byte("event: response.created\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.created","response":{"id":"r"}}` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.done","text":"完整答案"}` + "\n"))
	if got := rec.ResponseText(); got != "完整答案" {
		t.Fatalf("got %q", got)
	}
}

// TestSSEWithoutTextYieldsEmpty 覆盖纯工具调用轮次：不得把 SSE 前导当正文。
func TestSSEWithoutTextYieldsEmpty(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8192})
	rec.AppendResponse([]byte("event: codex.response.metadata\n"))
	rec.AppendResponse([]byte(`data: {"type":"codex.response.metadata","headers":{"etag":"x"}}` + "\n"))
	if got := rec.ResponseText(); got != "" {
		t.Fatalf("SSE 无正文时应留空，实际 %q", got)
	}
}

// TestNoDuplicateWhenDeltaAndDoneBothArrive 覆盖线上看到的重复正文：
// 增量事件拼出文本后，done 事件携带的全文必须被丢弃。
func TestNoDuplicateWhenDeltaAndDoneBothArrive(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8192})
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.delta","delta":"后端已扩展"}` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.delta","delta":"到 9 个节点"}` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.done","text":"后端已扩展到 9 个节点"}` + "\n"))
	if got := rec.ResponseText(); got != "后端已扩展到 9 个节点" {
		t.Fatalf("正文重复或缺失: %q", got)
	}
}

// TestDoneOnlyStreamStillWorks 覆盖只有 done、没有 delta 的实现。
func TestDoneOnlyStreamStillWorks(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 8192})
	rec.AppendResponse([]byte(`data: {"type":"response.created","response":{"id":"r"}}` + "\n"))
	rec.AppendResponse([]byte(`data: {"type":"response.output_text.done","text":"只有全文"}` + "\n"))
	if got := rec.ResponseText(); got != "只有全文" {
		t.Fatalf("got %q", got)
	}
}

// TestRecorderExtractsUserPrompt 覆盖采集器侧的接线：
// SetRequest 时就把用户消息单独抽出来，供观测层展示。
func TestRecorderExtractsUserPrompt(t *testing.T) {
	rec := New(Options{Prompt: true, Response: true, MaxBytes: 8192})
	rec.SetRequest(`{"model":"gpt-5.2","messages":[
		{"role":"system","content":"你是助手"},
		{"role":"user","content":"帮我看看这个报错"},
		{"role":"assistant","content":"好的"},
		{"role":"user","content":"是超时"}
	]}`)

	text, turns, protocol, _ := rec.UserPrompt()
	if protocol != "openai_chat" {
		t.Fatalf("protocol = %q", protocol)
	}
	if turns != 2 {
		t.Fatalf("turns = %d, want 2", turns)
	}
	if !strings.Contains(text, "报错") || !strings.Contains(text, "超时") {
		t.Fatalf("text = %q", text)
	}
	if strings.Contains(text, "你是助手") {
		t.Fatal("system 内容混进了用户 prompt")
	}
}

// TestRecorderUserPromptEmptyForNonUserBody 覆盖没有用户消息的请求。
func TestRecorderUserPromptEmptyForNonUserBody(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 4096})
	rec.SetRequest(`{"messages":[{"role":"system","content":"only"}]}`)
	if text, turns, _, _ := rec.UserPrompt(); text != "" || turns != 0 {
		t.Fatalf("got text=%q turns=%d", text, turns)
	}
}

// TestRecorderExposesSessionAnchor 覆盖会话归组需要的两个事实：
// 锚点消息（跳过客户端注入块）与请求体指纹。
func TestRecorderExposesSessionAnchor(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 4096})
	rec.SetRequest(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context><cwd>/tmp</cwd></environment_context>"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"把会话聚合补上"}]}
	]}`)

	if got := rec.FirstUserPrompt(); got != "把会话聚合补上" {
		t.Fatalf("FirstUserPrompt = %q，注入块必须被跳过", got)
	}
	if rec.RequestFingerprint() == "" {
		t.Fatal("请求指纹为空，重试折叠就没依据了")
	}
}

// TestRequestFingerprintTracksTailGrowth 覆盖"会话推进落在尾部"的现实：
// 超大请求体只有尾部变化，指纹必须随之变化。
func TestRequestFingerprintTracksTailGrowth(t *testing.T) {
	head := strings.Repeat("H", 200*1024)
	a := head + `{"tail":"call-1"}`
	b := head + `{"tail":"call-1","more":"call-2"}`

	if requestFingerprint(a) == requestFingerprint(b) {
		t.Fatal("尾部变化必须改变指纹，否则每一步都会被当成同一步的重试")
	}
	if requestFingerprint(b) != requestFingerprint(head+`{"tail":"call-1","more":"call-2"}`) {
		t.Fatal("同一请求体的指纹必须稳定")
	}
}

// TestRequestIdentitySkipsWorkWhenPromptDisabled 覆盖"关掉提示词采集就整体退化"：
// 不解析、不留指纹、不留锚点 —— 半开状态（解析后丢正文）会让每个请求白付
// 一次完整 JSON 解析，换来的却是没有正文的空卡片，不如明确退化。
func TestRequestIdentitySkipsWorkWhenPromptDisabled(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 4096})
	rec.SetRequest(`{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.RequestFingerprint() != "" {
		t.Fatal("关闭提示词采集时不该再算请求指纹")
	}
	if _, turns, _, _ := rec.UserPrompt(); turns != 0 {
		t.Fatalf("用户消息条数 = %d, want 0（未采集）", turns)
	}
	if rec.FirstUserPrompt() != "" || rec.LatestUserPrompt() != "" || rec.RequestText() != "" {
		t.Fatal("关闭提示词采集时不得留存任何正文")
	}
}

// TestSSEWithLeadingCommentIsRecognized 覆盖以 SSE 注释行开头的流：
// 这类流曾被误判成非 SSE，回复正文退化成 "event: ... data: ..." 噪声。
func TestSSEWithLeadingCommentIsRecognized(t *testing.T) {
	rec := New(Options{Prompt: false, Response: true, MaxBytes: 64 * 1024})
	stream := ": ping\n\n" +
		"event: codex.response.metadata\n" +
		"data: {\"type\":\"codex.response.metadata\"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"修好了\"}\n\n"
	rec.AppendResponse([]byte(stream))
	if got := rec.ResponseText(); got != "修好了" {
		t.Fatalf("ResponseText = %q，不能回退成原始 SSE 字节", got)
	}
}

// TestJSONBodyWithDataWordStaysJSON 保证首字符判定不会把 JSON 误判成 SSE。
func TestJSONBodyWithDataWordStaysJSON(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 32 * 1024})
	rec.AppendResponse([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi data: not sse"}}]}`))
	if got := rec.ResponseText(); got != "hi data: not sse" {
		t.Fatalf("ResponseText = %q", got)
	}
}

// TestTrailingLineWithoutNewlineIsKept 覆盖流末尾没有换行符的情况：
// 这一段曾经固定被丢掉，表现为"回答结尾少一截"。
func TestTrailingLineWithoutNewlineIsKept(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 64 * 1024})
	rec.AppendResponse([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"前半\"}\n\n"))
	rec.AppendResponse([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"尾巴\"}"))
	if got := rec.ResponseText(); got != "前半尾巴" {
		t.Fatalf("ResponseText = %q，最后一段不得丢", got)
	}
}

// TestTextTruncationCutsOnRuneBoundary 覆盖中文被截时不能切出半个字。
func TestTextTruncationCutsOnRuneBoundary(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 100})
	chunk := strings.Repeat("汉", 40) // 120 字节，超过上限
	rec.AppendResponse([]byte(`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":"` + chunk + `"}` + "\n\n"))
	got := rec.ResponseText()
	if !rec.TextTruncated() {
		t.Fatal("正文超过上限必须标记 text truncated")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("截断后出现非法 UTF-8：%q", got)
	}
	if utf8.RuneCountInString(got)*3 != len(got) {
		t.Fatalf("截断位置不在汉字边界：%q", got)
	}
}

// TestRawTruncationDoesNotImplyTextTruncation 保证"原始字节被截"不污染"正文被截"。
func TestRawTruncationDoesNotImplyTextTruncation(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 512})
	// 大量前导（无正文）把原始缓冲撑满，正文只有最后一小段。
	for i := 0; i < 20; i++ {
		rec.AppendResponse([]byte("event: response.created\ndata: {\"type\":\"response.created\",\"pad\":\"" + strings.Repeat("x", 200) + "\"}\n\n"))
	}
	rec.AppendResponse([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"答案\"}\n\n"))
	if got := rec.ResponseText(); got != "答案" {
		t.Fatalf("正文 = %q", got)
	}
	if !rec.RawTruncated() {
		t.Fatal("原始字节应该已被截断")
	}
	if rec.TextTruncated() {
		t.Fatal("正文没触顶就不该标 text truncated")
	}
}

func TestFinishReasonFromSSE(t *testing.T) {
	cases := map[string]string{
		`data: {"choices":[{"delta":{},"finish_reason":"length"}]}`:                                            "length",
		`data: {"type":"response.incomplete","response":{"status":"incomplete"}}`:                              "incomplete",
		`data: {"type":"response.completed","response":{"status":"completed"}}`:                                "stop",
		`data: {"type":"response.completed","response":{"incomplete_details":{"reason":"max_output_tokens"}}}`: "incomplete:max_output_tokens",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`:                                    "end_turn",
	}
	for line, want := range cases {
		rec := New(Options{Response: true, MaxBytes: 4096})
		rec.AppendResponse([]byte(line + "\n\n"))
		if got := rec.FinishReason(); got != want {
			t.Fatalf("%s → finish = %q, want %q", line, got, want)
		}
	}
}

func TestFinishReasonFromNonStreamJSON(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 4096})
	rec.AppendResponse([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	if got := rec.ResponseText(); got != "ok" {
		t.Fatalf("正文 = %q", got)
	}
	if got := rec.FinishReason(); got != "stop" {
		t.Fatalf("非流式响应的 finish_reason 未补上：%q", got)
	}
}

// TestPromptCapDefaultsWhenUnset 保证零值 Options 不会变成"无上限"。
func TestPromptCapDefaultsWhenUnset(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 4096})
	if rec.promptCap != defaultPromptMaxBytes {
		t.Fatalf("promptCap = %d, want %d", rec.promptCap, defaultPromptMaxBytes)
	}
}

// TestPromptCapAppliesToUserText 覆盖用户消息上限独立于请求体上限。
func TestPromptCapAppliesToUserText(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"` + strings.Repeat("问", 400) + `"}]}`
	rec := New(Options{Prompt: true, MaxBytes: 64 * 1024, PromptMaxBytes: 300})
	rec.SetRequest(body)
	text, _, _, status := rec.UserPrompt()
	if status != "ok" {
		t.Fatalf("status = %q", status)
	}
	if len(text) > 300+len("…（前文已省略）\n\n") {
		t.Fatalf("用户文本长度 %d，未按 PromptMaxBytes 截断", len(text))
	}
	if !utf8.ValidString(text) {
		t.Fatal("截断后必须是合法 UTF-8")
	}
}

// TestRecorderExposesLatestUserPrompt 覆盖 input 只展示最新那条用户消息。
func TestRecorderExposesLatestUserPrompt(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 64 * 1024})
	rec.SetRequest(`{"messages":[
		{"role":"user","content":"第一轮很长很长的历史"},
		{"role":"assistant","content":"回答"},
		{"role":"user","content":"最新的一句"}
	]}`)
	if got := rec.LatestUserPrompt(); got != "最新的一句" {
		t.Fatalf("LatestUserPrompt = %q", got)
	}
	text, turns, _, _ := rec.UserPrompt()
	if turns != 2 || !strings.Contains(text, "第一轮") {
		t.Fatalf("完整历史仍要保留：turns=%d text=%q", turns, text)
	}
}

// TestSSELineParsedOnce 保证同一行只解析一次（正文 + finish 共用）。
func TestSSELineParsedOnce(t *testing.T) {
	line := []byte(`data: {"type":"response.completed","response":{"status":"completed"},"choices":[]}`)
	piece, full, finish := parseSSELine(line)
	if piece != "" || full {
		t.Fatalf("completed 事件不应产出正文：%q full=%v", piece, full)
	}
	if finish != "stop" {
		t.Fatalf("finish = %q", finish)
	}
	text, _, finish2 := parseSSELine([]byte(`data: {"type":"response.output_text.delta","delta":"嗨"}`))
	if text != "嗨" || finish2 != "" {
		t.Fatalf("text=%q finish=%q", text, finish2)
	}
}

// TestLatestUserTruncationIsReported 覆盖展示用 prompt 的截断标记。
func TestLatestUserTruncationIsReported(t *testing.T) {
	rec := New(Options{Prompt: true, MaxBytes: 64 * 1024, PromptMaxBytes: 200})
	rec.SetRequest(`{"messages":[{"role":"user","content":"` + strings.Repeat("问", 300) + `"}]}`)
	if !rec.LatestUserTruncated() {
		t.Fatal("超过 PromptMaxBytes 必须标记")
	}
	if got := rec.LatestUserPrompt(); len(got) > 200+len("…（前文已省略）\n\n") {
		t.Fatalf("LatestUserPrompt 长度 %d 超上限", len(got))
	}
}

// TestLargeCompletedEventStillYieldsFinishReason 覆盖线上主因：
// response.completed 整对象事件在长回答时可达数百 KB，被 64KB 上限丢掉后
// finish_reason 就永远缺失。
func TestLargeCompletedEventStillYieldsFinishReason(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 64 * 1024})
	pad := strings.Repeat("x", 300*1024)
	rec.AppendResponse([]byte(`event: response.completed` + "\n" +
		`data: {"type":"response.completed","response":{"status":"completed","pad":"` + pad + `"}}` + "\n\n"))
	if got := rec.FinishReason(); got != "stop" {
		t.Fatalf("finish = %q，300KB 的 completed 事件必须仍能解析", got)
	}
}

// TestOversizedLineDoesNotSwallowFollowingEvents 保证超长行只丢自己。
func TestOversizedLineDoesNotSwallowFollowingEvents(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 64 * 1024})
	huge := strings.Repeat("y", maxPendingBytes+1024)
	rec.AppendResponse([]byte(`event: response.metadata` + "\n" + `data: {"pad":"` + huge))
	rec.AppendResponse([]byte(`"}` + "\n\n"))
	rec.AppendResponse([]byte(`event: response.output_text.delta` + "\n" +
		`data: {"type":"response.output_text.delta","delta":"后续正文"}` + "\n\n"))
	if got := rec.ResponseText(); got != "后续正文" {
		t.Fatalf("ResponseText = %q，超长行之后的正文不得被吃掉", got)
	}
}
