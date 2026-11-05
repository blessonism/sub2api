// Package prompttext 从模型请求体里抽出「用户说的话」，供观测层单独展示。
//
// 与 securityaudit 的分工：审查台需要 system/developer 段一起做判定，属于审计口径；
// 这里只回答"用户这一轮/这串对话说了什么"，属于展示口径。两者解析规则对同一批
// 协议保持一致，但用途不同，不共享实现以免把审计的语义绑死在展示需求上。
package prompttext

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// DefaultMaxBytes 是抽取结果的字节上限（按 rune 边界回退）。
const DefaultMaxBytes = 16 * 1024

// fallbackBlockLimit 是"整段没有用户文本"时兜底展示的字节上限。
const fallbackBlockLimit = 2 * 1024

// anchorLimit 是锚点消息的字节上限：它只用于会话归组与标题，
// 不参与展示正文。取 4KB 是为了能在同一条消息里看到
// "## My request:" 这类段落标记之后真正的请求内容。
const anchorLimit = 4096

// Result 是一次抽取的结果。
type Result struct {
	// Text 是按出现顺序拼接的用户消息文本。
	Text string
	// Turns 是用户消息条数（用于判断这是单轮还是多轮）。
	Turns int
	// FirstUser 是用于会话归组的锚点消息：第一条「用户真正说的话」，
	// 跳过客户端注入的包装块（如 <environment_context>），已按 anchorLimit 截断。
	FirstUser string
	// HistoryBytes 是拼接后完整用户历史的字节数（截断前），
	// 用来表达"这次请求带了多少上下文"，与实际展示的 LatestUser 区分开。
	HistoryBytes int
	// LatestTruncated 标记 LatestUser 是否因为上限被截断。
	LatestTruncated bool
	// LatestIsFallback 表示整段里没有用户写的文本，LatestUser 退回了客户端机器块
	// （典型场景：上下文压缩后，客户端只发摘要 + 工具块）。
	LatestIsFallback bool
	// LatestUser 是最新一条「用户真正说的话」。
	// 长会话里最后一条 user 消息经常是客户端追加的工具/审批块（Codex 会写
	// ">>> APPROVAL REQUEST START"），直接取末尾会把用户真正的问题埋掉甚至顶掉，
	// 所以这里从后往前找第一条真正的用户文本。
	LatestUser string
	// Protocol 是识别出的协议形态，仅用于排查。
	Protocol string
	// Status 说明抽取结果：ok / no_user / invalid_json / unrecognized。
	Status string
	// StepInput 是「这一步新加进来的东西」：agentic 循环里通常是上一次模型输出
	// 之后的工具执行结果。它填在观测层的步骤 input 上 —— 同一轮每条步骤的用户
	// 消息完全相同，重复发几十份没有信息量，而工具结果每步都不一样。
	StepInput string
	// StepInputKind 标记 StepInput 的来源：tool_result / ""（没有新增内容）。
	StepInputKind string
}

// Extract 用默认上限抽取用户文本。
func Extract(body []byte) Result { return ExtractWithLimit(body, DefaultMaxBytes) }

// ExtractWithLimit 按指定上限抽取用户文本。limit<=0 表示不截断
// （硬顶由调用方按 ingestion 载荷上限钳制）。
//
// 识别顺序：contents（Gemini）→ input[]（Responses）→ messages[]（Chat/Anthropic）→
// 顶层 input 字符串（少数 Responses 客户端）。
func ExtractWithLimit(body []byte, limit int) Result {
	if len(body) == 0 {
		return Result{}
	}
	if !utf8.Valid(body) {
		return Result{Status: "invalid_utf8"}
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return Result{Status: "invalid_json"}
	}

	if contents, ok := root["contents"].([]any); ok {
		texts := geminiUserTexts(contents)
		out := build("gemini", texts, limit)
		out.StepInput, out.StepInputKind = stepInput(contents, "gemini", limit)
		return out
	}
	if items, ok := root["input"].([]any); ok {
		texts := responsesUserTexts(items)
		out := build("openai_responses", texts, limit)
		out.StepInput, out.StepInputKind = stepInput(items, "openai_responses", limit)
		return out
	}
	if items, ok := root["messages"].([]any); ok {
		texts := chatUserTexts(items)
		out := build("openai_chat", texts, limit)
		out.StepInput, out.StepInputKind = stepInput(items, "openai_chat", limit)
		return out
	}
	// Responses 允许 input 直接是字符串。
	if raw, ok := root["input"].(string); ok && strings.TrimSpace(raw) != "" {
		return build("openai_responses_text", []string{raw}, limit)
	}
	return Result{Status: "unrecognized"}
}

func build(protocol string, texts []string, limit int) Result {
	cleaned := make([]string, 0, len(texts))
	for _, t := range texts {
		t = strings.TrimSpace(t)
		if t != "" {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		return Result{Protocol: protocol, Status: "no_user"}
	}
	full := strings.Join(cleaned, "\n\n")
	text := full
	if limit > 0 {
		text = tailRunes(text, limit)
	}
	latest, latestFallback := latestUserMessage(cleaned)
	latestLimit := limit
	if latestFallback {
		// 兜底展示的是机器生成的摘要，只留开头一段，别让它把整屏占满。
		latestLimit = fallbackBlockLimit
	}
	return Result{
		Text:             text,
		Turns:            len(cleaned),
		FirstUser:        truncateRunes(anchorMessage(cleaned), anchorLimit),
		LatestUser:       truncateLimited(latest, latestLimit),
		LatestTruncated:  exceedsLimit(latest, latestLimit),
		LatestIsFallback: latestFallback,
		HistoryBytes:     len(full),
		Protocol:         protocol,
		Status:           "ok",
	}
}

// anchorMessage 选用于会话归组的锚点消息。
//
// 跳过前面所有「客户端注入的包装块」：Codex 会把 <environment_context>、
// <user_instructions> 这类结构性内容塞进 role=user，若直接取第一条，
// 同一客户端的所有会话会塌成同一个锚点、进而被并进同一个会话。
func anchorMessage(texts []string) string {
	for _, t := range texts {
		// 纯注入上下文（没有请求段落）跳过；带请求段落的整条保留，
		// 因为用户真正说的话就在同一条消息的后半段。
		if !isInjectedFraming(t) || ExtractRequestSection(t) != "" {
			return t
		}
	}
	return texts[0]
}

// injectedMarkers 是客户端注入的上下文块特征。命中即视为「不是用户说的话」，
// 只影响锚点选择与标题，不影响展示正文。
var injectedMarkers = []string{
	"<environment_context",
	"<user_instructions",
	"<instructions",
	"<user_info",
	"<rules",
	"<system-reminder",
	"# agents.md instructions",
	"# files mentioned by the user",
	"# context from my ide setup",
}

// compactionSignature 是上下文压缩摘要的特征：它出现在 user 角色里，
// 但内容是模型自己写的摘要，不是用户这一轮说的话。
const compactionSignature = "in this summary to assist with your own analysis"

// requestSectionMarkers 是「真实请求」段落标记：客户端会把用户原话包在
// 一大段上下文后面，用这些标题分隔。取最后一个标记之后的内容作为标题。
var requestSectionMarkers = []string{
	"## my request",
	"## user request",
	"## 我的请求",
	"## 需求",
	"## task",
	"## 任务",
	"## goal",
	"## 目标",
}

// latestUserMessage 从后往前找最新一条真正的用户文本。
// 找不到（整段都是客户端追加的块）时退回最后一条，保持"总有内容可看"。
func latestUserMessage(texts []string) (string, bool) {
	for i := len(texts) - 1; i >= 0; i-- {
		if !isMachineBlock(texts[i]) {
			return texts[i], false
		}
	}
	// 整段都是机器块：退回最后一条，并告诉调用方这是兜底值。
	return texts[len(texts)-1], true
}

// machineBlockPrefixes 是明确的机器生成前缀。这里刻意不复用
// isInjectedFraming 的"以 < 开头且含闭合标签"这类宽判据：
// 用户完全可能把一段 XML/HTML 当问题贴进来，那时它就是他真正的话。
var machineBlockPrefixes = []string{
	"<environment_context",
	"<heartbeat",
	"<user_info",
	"<rules",
	"<system-reminder",
	"# agents.md instructions",
	"# files mentioned by the user",
	"# context from my ide setup",
	// Codex 的上下文压缩指令：它以 role=user 发出，但内容完全是机器指令。
	// 不排除的话，压缩轮次的卡片 input 会显示这句英文，读起来像"用户要求做摘要"。
	"you are performing a context checkpoint compaction",
	// Claude Code 的 /compact 提示词。
	"your task is to create a detailed summary of the conversation so far",
}

// isMachineBlock 判断一条消息是否是客户端机器人化追加的块（不是用户写的）。
func isMachineBlock(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	head := lower
	if len(head) > 512 {
		head = head[:512]
	}
	for _, prefix := range machineBlockPrefixes {
		if strings.HasPrefix(head, prefix) {
			return true
		}
	}
	for _, marker := range clientMetaMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if strings.Contains(lower, compactionSignature) {
		return true
	}
	if strings.HasPrefix(trimmed, "[") {
		if end := strings.IndexByte(trimmed, ']'); end > 0 && end < 12 {
			rest := strings.ToLower(trimmed[end+1:])
			if strings.HasPrefix(rest, " tool exec call") || strings.HasPrefix(rest, " tool call") {
				return true
			}
		}
	}
	return false
}

// truncateLimited 按上限截断；limit<=0 表示不截断。
func truncateLimited(s string, limit int) string {
	if limit <= 0 {
		return s
	}
	return truncateRunes(s, limit)
}

// exceedsLimit 报告 s 是否超过上限（limit<=0 视为不限制）。
func exceedsLimit(s string, limit int) bool {
	return limit > 0 && len(s) > limit
}

// clientMetaMarkers 是客户端写进 user 消息里的结构化块特征：
// 审批请求、转录增量、工具调用回显。它们不是用户说的话。
var clientMetaMarkers = []string{
	"another language model started to solve this problem",
	">>> approval request",
	">>> transcript delta",
	">>> parent turn permission context",
	">>> tool call",
	"tool exec call:",
}

// isInjectedFraming 判断一条消息是否是客户端注入的上下文块。
// 判定刻意保守：只看开头特征，命中即视为注入。
func isInjectedFraming(text string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	if trimmed == "" {
		return false
	}
	head := trimmed
	if len(head) > 512 {
		head = head[:512]
	}
	for _, marker := range injectedMarkers {
		if strings.HasPrefix(head, marker) {
			return true
		}
	}
	if strings.HasPrefix(head, "<") && (strings.Contains(head, "</") || strings.Contains(head, "/>")) {
		return true
	}
	return strings.Contains(trimmed, compactionSignature)
}

// ExtractRequestSection 返回消息里「真实请求」段落之后的正文。
// 没有命中段落标记时返回空串。
func ExtractRequestSection(text string) string {
	lower := strings.ToLower(text)
	best := -1
	bestLen := 0
	for _, marker := range requestSectionMarkers {
		if idx := strings.LastIndex(lower, marker); idx > best {
			best = idx
			bestLen = len(marker)
		}
	}
	if best < 0 {
		return ""
	}
	rest := text[best+bestLen:]
	rest = strings.TrimLeft(rest, " \t:：\r\n")
	return strings.TrimSpace(rest)
}

// chatUserTexts 处理 OpenAI chat 与 Anthropic messages：role=="user" 的 content。
func chatUserTexts(items []any) []string {
	var out []string
	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(msg["role"]))) != "user" {
			continue
		}
		if text := contentText(msg["content"]); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// responsesUserTexts 处理 Responses 的 input 数组：type=="message" 且 role=="user"。
func responsesUserTexts(items []any) []string {
	var out []string
	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(stringValue(msg["type"])))
		if kind != "" && kind != "message" {
			// additional_tools / function_call_output 等不是用户输入。
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(msg["role"]))) != "user" {
			continue
		}
		if text := contentText(msg["content"]); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// geminiUserTexts 处理 Gemini 的 contents[].parts[].text。
func geminiUserTexts(contents []any) []string {
	var out []string
	for _, item := range contents {
		turn, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(stringValue(turn["role"])))
		// Gemini 省略 role 时默认是 user。
		if role != "" && role != "user" {
			continue
		}
		parts, ok := turn["parts"].([]any)
		if !ok {
			continue
		}
		var b strings.Builder
		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := part["text"].(string); ok {
				b.WriteString(s)
			}
		}
		if text := strings.TrimSpace(b.String()); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// contentText 从 content 抽纯文本：兼容字符串与 block 数组，跳过非文本块。
func contentText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		var b strings.Builder
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			kind := strings.ToLower(strings.TrimSpace(stringValue(block["type"])))
			switch kind {
			case "", "text", "input_text", "output_text":
				if s, ok := block["text"].(string); ok {
					b.WriteString(s)
				}
			default:
				// image / tool_result / input_image 等一律跳过。
			}
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// tailRunes 超长时保留尾部：多轮会话里"最近说了什么"比开头更有价值，
// 正序截断会把上限全耗在最早那几轮上。
func tailRunes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	const marker = "…（前文已省略）\n\n"
	cut := len(s) - (limit - len(marker))
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	return marker + s[cut:]
}

func truncateRunes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// stepInput 从请求体里找「这一步新加进来的内容」。
//
// agentic 客户端的一次提问会拆成几十次上游调用：每一步的请求体都是
// 「历史 + 上一步模型输出 + 新的工具结果」。历史与用户消息在整轮里不变，
// 真正每步不同的只有工具结果 —— 把它放到观测层的步骤 input 上，
// 步骤才有可读的"这一步模型看到了什么"。
//
// 找不到（首步、纯文本对话、或客户端不按标准放工具结果）时返回空，
// 观测层会退回展示该轮的用户消息。
func stepInput(items []any, protocol string, limit int) (string, string) {
	for i := len(items) - 1; i >= 0; i-- {
		msg, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(stringValue(msg["role"])))
		kind := strings.ToLower(strings.TrimSpace(stringValue(msg["type"])))

		switch protocol {
		case "openai_responses":
			// function_call_output / custom_tool_call_output / computer_call_output …
			if strings.HasSuffix(kind, "_output") {
				if text := firstText(msg["output"], msg["content"]); text != "" {
					return truncateLimited(text, limit), "tool_result"
				}
			}
		case "gemini":
			if text := geminiFunctionText(msg); text != "" {
				return truncateLimited(text, limit), "tool_result"
			}
		default:
			// OpenAI chat / Anthropic：工具结果有独立 role。
			if role == "tool" || role == "function" {
				if text := contentText(msg["content"]); text != "" {
					return truncateLimited(text, limit), "tool_result"
				}
			}
			// Anthropic 更常见：role=user 里塞 tool_result 块。
			if role == "user" {
				if text := toolResultBlocks(msg["content"]); text != "" {
					return truncateLimited(text, limit), "tool_result"
				}
			}
		}
	}
	return "", ""
}

// firstText 取第一个非空的文本形态值。
func firstText(values ...any) string {
	for _, v := range values {
		if text := contentText(v); text != "" {
			return text
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// toolResultBlocks 从 Anthropic 的 content 数组里抽 tool_result 块的文本。
func toolResultBlocks(value any) string {
	blocks, ok := value.([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(block["type"]))) != "tool_result" {
			continue
		}
		if text := contentText(block["content"]); text != "" {
			b.WriteString(text)
		}
	}
	return strings.TrimSpace(b.String())
}

// geminiFunctionText 抽 Gemini 的 functionResponse.parts[].functionResponse.response。
func geminiFunctionText(msg map[string]any) string {
	parts, ok := msg["parts"].([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, item := range parts {
		part, ok := item.(map[string]any)
		if !ok {
			continue
		}
		resp, ok := part["functionResponse"].(map[string]any)
		if !ok {
			continue
		}
		if text := contentText(resp["response"]); text != "" {
			b.WriteString(text)
			continue
		}
		// response 常是对象：退一步把整体序列化出来。
		if raw, err := json.Marshal(resp["response"]); err == nil {
			b.Write(raw)
		}
	}
	return strings.TrimSpace(b.String())
}
