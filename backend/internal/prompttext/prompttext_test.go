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

// TestStepInputExtractsToolResult 覆盖「这一步新加进来的内容」：
// agentic 循环里每步的请求体都是「历史 + 工具结果」，只有工具结果是新的。
func TestStepInputExtractsToolResult(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "openai chat tool role",
			body: `{"messages":[
				{"role":"user","content":"看下 README"},
				{"role":"assistant","content":null,"tool_calls":[{"id":"c1"}]},
				{"role":"tool","tool_call_id":"c1","content":"# Sub2API\n安装说明…"}]}`,
			want: "# Sub2API\n安装说明…",
		},
		{
			name: "anthropic tool_result block",
			body: `{"messages":[
				{"role":"user","content":"看下 README"},
				{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"read"}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"文件内容在此"}]}]}`,
			want: "文件内容在此",
		},
		{
			name: "responses function_call_output",
			body: `{"input":[
				{"type":"message","role":"user","content":"看下 README"},
				{"type":"function_call_output","call_id":"f1","output":"命令输出：ok"}]}`,
			want: "命令输出：ok",
		},
		{
			name: "first step has no tool result",
			body: `{"messages":[{"role":"user","content":"第一次提问"}]}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractWithLimit([]byte(tc.body), DefaultMaxBytes)
			if got.StepInput != tc.want {
				t.Fatalf("StepInput = %q, want %q", got.StepInput, tc.want)
			}
			if tc.want != "" && got.StepInputKind != "tool_result" {
				t.Fatalf("StepInputKind = %q, want tool_result", got.StepInputKind)
			}
		})
	}
}

// TestCompactionPromptIsNotUserText 覆盖线上实测到的一类误判：
// Codex 的上下文压缩指令以 role=user 发出，但它是机器指令，不是用户说的话。
// 不排除的话，压缩轮次的卡片 input 会显示那句英文，读起来像"用户要求做摘要"。
func TestCompactionPromptIsNotUserText(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":"把观测改成一轮一张卡"},
		{"role":"user","content":"You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary for the next model."}
	]}`
	got := ExtractWithLimit([]byte(body), DefaultMaxBytes)
	if got.LatestUser != "把观测改成一轮一张卡" {
		t.Fatalf("LatestUser = %q，压缩指令不该冒充用户消息", got.LatestUser)
	}
	if got.LatestIsFallback {
		t.Fatal("还有真实用户文本时不该标 fallback")
	}
	if got.Turns != 2 {
		t.Fatalf("Turns = %d，压缩指令仍算一条 user 消息（轮次识别要用）", got.Turns)
	}
}

// TestLatestUserKeepsTail：用户消息超限时必须保留尾部。
//
// 一句话的落点在末尾（"……所以帮我改一下这个函数"），留头部等于把真正的
// 诉求切掉。而观测层 clampCaptured 用的也是保留尾部，两处方向必须一致 ——
// 之前 LatestUser 留头部、下游再留这前半段的尾部，最后既不是原头也不是原尾。
func TestLatestUserKeepsTail(t *testing.T) {
	// 超长的是**最后一条**用户消息本身：截断判据看的是 latest 是否越限。
	long := strings.Repeat("需求描述。", 2000) + "最后这句话才是要问的"
	body := `{"messages":[` +
		`{"role":"user","content":"先前的背景"},` +
		`{"role":"assistant","content":"收到"},` +
		`{"role":"user","content":"` + long + `"}]}`

	got := ExtractWithLimit([]byte(body), 4<<10)
	if !strings.Contains(got.LatestUser, "最后这句话才是要问的") {
		t.Fatalf("LatestUser 丢了尾部诉求：%q", got.LatestUser)
	}
	if !got.LatestTruncated {
		t.Fatal("超限必须标记 LatestTruncated")
	}
}

// TestLatestUserNoLimitKeepsWhole：limit<=0 表示不截断，
// 不能被省略标记（26 字节）反算出负的切点、把整段替换成一个标记。
func TestLatestUserNoLimitKeepsWhole(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"完整的一句话"}]}`
	got := ExtractWithLimit([]byte(body), 0)
	if got.LatestUser != "完整的一句话" {
		t.Fatalf("LatestUser = %q，limit<=0 应原样返回", got.LatestUser)
	}
}

// TestSystemPromptAcrossProtocols：四种协议的 system 落点都要能抽到。
//
// 它在一轮里恒定，之前完全没被单独抽出来，只以「请求体前 8KB 抽样」的形式
// 部分存在 —— system 一旦不在 JSON 开头就整个看不到。
func TestSystemPromptAcrossProtocols(t *testing.T) {
	cases := map[string]string{
		"openai_chat": `{"messages":[
			{"role":"system","content":"你是严谨的工程师"},
			{"role":"user","content":"改个 bug"}]}`,
		"openai_responses": `{"instructions":"你是严谨的工程师",
			"input":[{"role":"user","content":"改个 bug"}]}`,
		"anthropic": `{"system":"你是严谨的工程师",
			"messages":[{"role":"user","content":"改个 bug"}]}`,
		"anthropic_blocks": `{"system":[{"type":"text","text":"你是严谨的工程师"}],
			"messages":[{"role":"user","content":"改个 bug"}]}`,
		"gemini": `{"systemInstruction":{"parts":[{"text":"你是严谨的工程师"}]},
			"contents":[{"role":"user","parts":[{"text":"改个 bug"}]}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := Extract([]byte(body))
			if got.System != "你是严谨的工程师" {
				t.Fatalf("System = %q，want 你是严谨的工程师", got.System)
			}
			if got.SystemTruncated {
				t.Fatal("未超限不该标记 SystemTruncated")
			}
		})
	}
}

// TestDeveloperRoleCountsAsSystem：OpenAI 的 role=developer 是新版 system。
func TestDeveloperRoleCountsAsSystem(t *testing.T) {
	body := `{"messages":[
		{"role":"developer","content":"遵守这些规则"},
		{"role":"user","content":"开始"}]}`
	got := Extract([]byte(body))
	if got.System != "遵守这些规则" {
		t.Fatalf("System = %q", got.System)
	}
	// developer 消息不该被当成用户说的话。
	if strings.Contains(got.Text, "遵守这些规则") {
		t.Fatalf("developer 消息混进了用户文本：%q", got.Text)
	}
}

// TestToolDefinitionsCaptured：工具定义要原样留 JSON，含 schema 与枚举。
func TestToolDefinitionsCaptured(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"读个文件"}],
		"tools":[{"type":"function","function":{"name":"read_file",
			"parameters":{"type":"object","properties":{"path":{"type":"string"}},
			"required":["path"]}}}]}`
	got := Extract([]byte(body))
	if got.Tools == "" {
		t.Fatal("Tools 为空，工具定义没被抽到")
	}
	if !strings.Contains(got.Tools, "read_file") {
		t.Fatalf("Tools 丢了工具名：%q", got.Tools)
	}
	if !strings.Contains(got.Tools, `"required":["path"]`) {
		t.Fatalf("Tools 丢了 schema 细节：%q", got.Tools)
	}
	if got.ToolsTruncated {
		t.Fatal("未超限不该标记 ToolsTruncated")
	}
}

// TestToolDefinitionsEmptyAndMissing：没有 tools 或空数组时返回空串，
// 不要写一个 "[]" 让卡片看起来有内容。
func TestToolDefinitionsEmptyAndMissing(t *testing.T) {
	for name, body := range map[string]string{
		"missing": `{"messages":[{"role":"user","content":"hi"}]}`,
		"empty":   `{"messages":[{"role":"user","content":"hi"}],"tools":[]}`,
		"null":    `{"messages":[{"role":"user","content":"hi"}],"tools":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			got := Extract([]byte(body))
			if got.Tools != "" {
				t.Fatalf("Tools = %q，want 空", got.Tools)
			}
		})
	}
}

// TestSystemAndToolsTruncationFlagged：超限必须报告，不能静默截断。
func TestSystemAndToolsTruncationFlagged(t *testing.T) {
	sys := strings.Repeat("规", 8000) // 24KB > 16KB
	tools := `[{"type":"function","function":{"name":"x","description":"` +
		strings.Repeat("描", 30000) + `"}}]` // 90KB > 64KB
	body := `{"messages":[{"role":"system","content":"` + sys + `"},` +
		`{"role":"user","content":"hi"}],"tools":` + tools + `}`

	got := Extract([]byte(body))
	if !got.SystemTruncated {
		t.Fatal("system 超 16KB 必须标记 SystemTruncated")
	}
	if len(got.System) > systemLimit {
		t.Fatalf("System 长度 = %d，超过 %d", len(got.System), systemLimit)
	}
	if !got.ToolsTruncated {
		t.Fatal("tools 超 64KB 必须标记 ToolsTruncated")
	}
	if len(got.Tools) > toolsLimit {
		t.Fatalf("Tools 长度 = %d，超过 %d", len(got.Tools), toolsLimit)
	}
}
