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
	if !rec.ResponseTruncated() {
		t.Fatal("响应超限必须标记 truncated")
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
	if rec.ResponseTruncated() {
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

// TestRequestFingerprintSkipsWorkWhenDisabled 保证未开启采集时零开销。
func TestRequestFingerprintSkipsWorkWhenDisabled(t *testing.T) {
	rec := New(Options{Response: true, MaxBytes: 4096})
	rec.SetRequest(`{"messages":[{"role":"user","content":"hi"}]}`)
	if rec.RequestFingerprint() != "" || rec.FirstUserPrompt() != "" {
		t.Fatal("关闭提示词采集时不该记录任何锚点信息")
	}
}
