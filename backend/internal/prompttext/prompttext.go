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

// Result 是一次抽取的结果。
type Result struct {
	// Text 是按出现顺序拼接的用户消息文本。
	Text string
	// Turns 是用户消息条数（用于判断这是单轮还是多轮）。
	Turns int
	// Protocol 是识别出的协议形态，仅用于排查。
	Protocol string
	// Status 说明抽取结果：ok / no_user / invalid_json / unrecognized。
	Status string
}

// Extract 自动识别协议并从请求体里抽取用户文本。
//
// 识别顺序：contents（Gemini）→ input[]（Responses）→ messages[]（Chat/Anthropic）→
// 顶层 input 字符串（少数 Responses 客户端）。
func Extract(body []byte) Result {
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
		return build("gemini", texts)
	}
	if items, ok := root["input"].([]any); ok {
		texts := responsesUserTexts(items)
		return build("openai_responses", texts)
	}
	if items, ok := root["messages"].([]any); ok {
		texts := chatUserTexts(items)
		return build("openai_chat", texts)
	}
	// Responses 允许 input 直接是字符串。
	if raw, ok := root["input"].(string); ok && strings.TrimSpace(raw) != "" {
		return build("openai_responses_text", []string{raw})
	}
	return Result{Status: "unrecognized"}
}

func build(protocol string, texts []string) Result {
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
	return Result{
		Text:     truncateRunes(strings.Join(cleaned, "\n\n"), DefaultMaxBytes),
		Turns:    len(cleaned),
		Protocol: protocol,
		Status:   "ok",
	}
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
