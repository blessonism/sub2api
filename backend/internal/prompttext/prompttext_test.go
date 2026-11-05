package prompttext

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractOpenAIChat(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.2",
		"messages":[
			{"role":"system","content":"You are a coding agent."},
			{"role":"user","content":"帮我把这个函数改成尾递归"},
			{"role":"assistant","content":"好的，先看下当前实现"},
			{"role":"user","content":[{"type":"text","text":"顺便补个单测"}]}
		]}`)
	got := Extract(body)
	if got.Protocol != "openai_chat" {
		t.Fatalf("protocol = %q", got.Protocol)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d, want 2", got.Turns)
	}
	if !strings.Contains(got.Text, "尾递归") || !strings.Contains(got.Text, "补个单测") {
		t.Fatalf("text = %q", got.Text)
	}
	if strings.Contains(got.Text, "coding agent") {
		t.Fatal("system 消息不得混入用户文本")
	}
}

func TestExtractOpenAIResponses(t *testing.T) {
	body := []byte(`{
		"model":"gpt-6-astra","stream":true,
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"You are Codex"}]},
			{"type":"additional_tools","id":"at_1","role":"developer"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"这个报错怎么修"}]},
			{"type":"function_call_output","call_id":"c1","output":"ok"}
		]}`)
	got := Extract(body)
	if got.Protocol != "openai_responses" {
		t.Fatalf("protocol = %q", got.Protocol)
	}
	if got.Turns != 1 || got.Text != "这个报错怎么修" {
		t.Fatalf("turns=%d text=%q", got.Turns, got.Text)
	}
}

func TestExtractAnthropic(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"system":"You are Claude",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"读一下这个文件"},
				{"type":"image","source":{"type":"base64","data":"AAA"}}
			]},
			{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"read"}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file body"}]}
		]}`)
	got := Extract(body)
	if got.Turns != 1 {
		t.Fatalf("turns = %d, want 1（tool_result 不算用户发言）", got.Turns)
	}
	if got.Text != "读一下这个文件" {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestExtractGemini(t *testing.T) {
	body := []byte(`{
		"contents":[
			{"role":"user","parts":[{"text":"总结这段日志"}]},
			{"role":"model","parts":[{"text":"好的"}]},
			{"parts":[{"text":"只保留结论"}]}
		]}`)
	got := Extract(body)
	if got.Protocol != "gemini" {
		t.Fatalf("protocol = %q", got.Protocol)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d, want 2（缺省 role 视为 user）", got.Turns)
	}
	if !strings.Contains(got.Text, "总结这段日志") || !strings.Contains(got.Text, "只保留结论") {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestExtractResponsesStringInput(t *testing.T) {
	got := Extract([]byte(`{"model":"m","input":"一句话提问"}`))
	if got.Text != "一句话提问" || got.Turns != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestExtractKeepsTailWhenOversized(t *testing.T) {
	head := strings.Repeat("旧", DefaultMaxBytes) // 挤满上限的前文
	body := []byte(`{"messages":[{"role":"user","content":"` + head + `"},{"role":"user","content":"这是最后一句提问"}]}`)
	got := Extract(body)
	if len(got.Text) > DefaultMaxBytes {
		t.Fatalf("长度 %d 超过上限", len(got.Text))
	}
	if !strings.HasSuffix(got.Text, "这是最后一句提问") {
		t.Fatalf("截断应保留最近的尾部，实际结尾: %q", got.Text[len(got.Text)-40:])
	}
	if !strings.Contains(got.Text, "前文已省略") {
		t.Fatal("截断应带省略标记")
	}
	for _, r := range got.Text {
		if r == '\uFFFD' {
			t.Fatal("截断产生了替换字符，说明切坏了 rune")
		}
	}
}

func TestExtractEmptyWhenNoUserTurn(t *testing.T) {
	got := Extract([]byte(`{"messages":[{"role":"system","content":"only system"}]}`))
	if got.Text != "" || got.Turns != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestExtractInvalidJSON(t *testing.T) {
	if got := Extract([]byte("{not json")); got.Text != "" || got.Turns != 0 {
		t.Fatalf("非法 JSON 应返回空结果: %+v", got)
	}
}

func TestExtractFirstUserSkipsInjectedFraming(t *testing.T) {
	body := []byte(`{
		"model":"gpt-6-astra",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context><cwd>/Users/suki/code/sub2api</cwd></environment_context>"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"把观测层的会话聚合补上"}]}
		]}`)
	got := Extract(body)
	if got.Status != "ok" {
		t.Fatalf("status = %q", got.Status)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d, want 2（注入块仍算一轮，只影响锚点选择）", got.Turns)
	}
	if got.FirstUser != "把观测层的会话聚合补上" {
		t.Fatalf("FirstUser = %q，包装块必须被跳过", got.FirstUser)
	}
}

func TestExtractFirstUserFallsBackToFirstWhenAllFraming(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"<user_instructions>只用中文</user_instructions>"}]}
	]}`)
	got := Extract(body)
	if got.FirstUser == "" {
		t.Fatal("全是包装块时也要给出锚点，否则会话拿不到标题")
	}
	if !strings.Contains(got.FirstUser, "只用中文") {
		t.Fatalf("FirstUser = %q", got.FirstUser)
	}
}

func TestExtractFirstUserIsHeadTruncated(t *testing.T) {
	long := strings.Repeat("会", 2000)
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"` + long + `"}]}]}`)
	got := Extract(body)
	if len(got.FirstUser) > anchorLimit {
		t.Fatalf("FirstUser len = %d，应受 anchorLimit=%d 约束", len(got.FirstUser), anchorLimit)
	}
}

func TestExtractFirstUserSkipsMarkdownContextBlocks(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"<user_info>\nOS Version: windows\n</user_info>\n\n<rules>\n<user_rule>只说中文</user_rule>\n</rules>"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"把会话聚合补上"}]}
	]}`)
	got := Extract(body)
	if got.FirstUser != "把会话聚合补上" {
		t.Fatalf("FirstUser = %q，<user_info>/<rules> 属于注入上下文", got.FirstUser)
	}
}

func TestExtractFirstUserKeepsBlockThatCarriesRequest(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"# Files mentioned by the user:\n- a.svg\n\n## My request:\n看下这张图对应哪组实验数据"}]}
	]}`)
	got := Extract(body)
	if !strings.Contains(got.FirstUser, "看下这张图") {
		t.Fatalf("带 ## My request 的消息必须保留，否则真实请求被丢掉：%q", got.FirstUser)
	}
}

func TestExtractRequestSection(t *testing.T) {
	cases := []struct{ in, want string }{
		{"# Files mentioned by the user:\n- a.svg\n\n## My request:\n看下这张图", "看下这张图"},
		{"上下文\n## 我的请求：\n把登录修好", "把登录修好"},
		{"没有段落标记的普通消息", ""},
	}
	for _, c := range cases {
		if got := ExtractRequestSection(c.in); got != c.want {
			t.Fatalf("ExtractRequestSection(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractFirstUserSkipsCompactionSummary(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"...continue with the summary in this summary to assist with your own analysis:\n## Identity"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"继续"}]}
	]}`)
	got := Extract(body)
	if got.FirstUser != "继续" {
		t.Fatalf("FirstUser = %q，压缩摘要不是用户说的话", got.FirstUser)
	}
}

func TestLatestUserSkipsApprovalBlocks(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"帮我看下这个报错"}]},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"好的"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"[153] tool exec call: tools.exec_command({cmd:\"ls\"})\n>>> APPROVAL REQUEST END"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":">>> PARENT TURN PERMISSION CONTEXT START\nThe active permission profile ...\n>>> APPROVAL REQUEST START\nAssess the exact planned action"}]}
	]}`)
	got := Extract(body)
	if got.LatestUser != "帮我看下这个报错" {
		t.Fatalf("LatestUser = %q，末尾的工具/审批块必须被跳过", got.LatestUser)
	}
	if got.Turns != 3 {
		t.Fatalf("turns = %d, want 3", got.Turns)
	}
}

func TestLatestUserFallsBackWhenAllMeta(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":">>> APPROVAL REQUEST START\nonly meta"}]}`)
	got := Extract(body)
	if got.LatestUser == "" {
		t.Fatal("整段都是客户端块时也要退回最后一条，不能空")
	}
}

func TestLatestUserIsTheNewestRealMessage(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"第一轮问题"},
		{"role":"assistant","content":"回答"},
		{"role":"user","content":"第二轮：这一句才是最新的"}
	]}`)
	got := Extract(body)
	if got.LatestUser != "第二轮：这一句才是最新的" {
		t.Fatalf("LatestUser = %q", got.LatestUser)
	}
	if !strings.Contains(got.Text, "第一轮问题") {
		t.Fatal("完整历史仍要留在 Text 里（用于字节统计与归档）")
	}
}

func TestHistoryBytesIgnoresTruncation(t *testing.T) {
	long := strings.Repeat("历", 500)
	body := []byte(`{"messages":[{"role":"user","content":"` + long + `"},{"role":"user","content":"最新"}]}`)
	got := ExtractWithLimit(body, 200)
	if got.HistoryBytes < 500*3 {
		t.Fatalf("HistoryBytes = %d，必须是截断前的真实历史字节数", got.HistoryBytes)
	}
	if len(got.Text) > 200+len("…（前文已省略）\n\n") {
		t.Fatalf("Text 仍应按上限截断：%d", len(got.Text))
	}
}

// TestLatestUserUsesPromptLimitNotAnchorLimit 覆盖 review 发现的回归：
// LatestUser 曾被锚点上限（4KB）截断，比它替代的 16KB 展示还小。
func TestLatestUserUsesPromptLimitNotAnchorLimit(t *testing.T) {
	msg := strings.Repeat("问", 5000) // 15KB
	body := []byte(`{"messages":[{"role":"user","content":"` + msg + `"}]}`)
	got := ExtractWithLimit(body, 512*1024)
	if len(got.LatestUser) < 10*1024 {
		t.Fatalf("LatestUser 只有 %d 字节，被锚点上限截了", len(got.LatestUser))
	}
	if got.LatestTruncated {
		t.Fatal("未超 512KB 上限时不应标截断")
	}

	small := ExtractWithLimit(body, 4096)
	if !small.LatestTruncated {
		t.Fatal("超过上限必须标 LatestTruncated")
	}
	if !utf8.ValidString(small.LatestUser) {
		t.Fatal("截断后必须是合法 UTF-8")
	}
}

// TestLatestUserKeepsUserPastedXML 覆盖宽判据误伤：用户自己贴的 XML 就是他真正的话。
func TestLatestUserKeepsUserPastedXML(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"先看这个"},
		{"role":"assistant","content":"好"},
		{"role":"user","content":"<config><host>10.0.0.1</host></config> 这个配置有问题吗"}
	]}`)
	got := Extract(body)
	if !strings.HasPrefix(got.LatestUser, "<config>") {
		t.Fatalf("LatestUser = %q，用户贴的 XML 不能被当成注入块跳过", got.LatestUser)
	}
}

// TestLatestUserSkipsHeartbeatAndCompactionSummary 覆盖线上真实噪声：
// Codex 的 <heartbeat> 自动化块和"另一个模型接手"的压缩摘要都不是用户原话。
func TestLatestUserSkipsHeartbeatAndCompactionSummary(t *testing.T) {
	cases := []string{
		`{"messages":[{"role":"user","content":"真问题"},{"role":"user","content":"<heartbeat>\r\n  <automation_id>automation</automation_id>\r\n</heartbeat>"}]}`,
		`{"messages":[{"role":"user","content":"真问题"},{"role":"user","content":"Another language model started to solve this problem and produced a summary of it."}]}`,
	}
	for _, body := range cases {
		if got := Extract([]byte(body)).LatestUser; got != "真问题" {
			t.Fatalf("LatestUser = %q，机器块必须被跳过", got)
		}
	}
}

// TestLatestUserFallbackIsCappedAndFlagged 覆盖线上真实场景：
// 上下文压缩后客户端只发摘要 + 工具块，此时不能把 15KB 摘要当成用户原话。
func TestLatestUserFallbackIsCappedAndFlagged(t *testing.T) {
	summary := "Another language model started to solve this problem and produced a summary of its thinking process. " + strings.Repeat("摘要正文", 2000)
	body := []byte(`{"messages":[
		{"role":"user","content":"<environment_context><cwd>/tmp</cwd></environment_context>"},
		{"role":"user","content":"` + summary + `"}]}`)
	got := Extract(body)
	if !got.LatestIsFallback {
		t.Fatal("整段都是机器块时必须标记 fallback")
	}
	if len(got.LatestUser) > fallbackBlockLimit {
		t.Fatalf("兜底展示长度 %d 超过 %d", len(got.LatestUser), fallbackBlockLimit)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d", got.Turns)
	}
}
