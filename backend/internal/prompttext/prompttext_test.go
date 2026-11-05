package prompttext

import (
	"strings"
	"testing"
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
