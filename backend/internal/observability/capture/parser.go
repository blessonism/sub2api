package capture

import (
	"encoding/json"
	"strings"
)

// ExtractAssistantText 从响应原始字节里抽出 assistant 正文。
//
// 同时覆盖两种传输形态：
//   - SSE（OpenAI chat / OpenAI responses / Anthropic messages 的流式增量）；
//   - 单次 JSON（三类协议的完整响应）。
//
// 解析不出结构化正文时返回 ok=false，调用方据自身策略决定是否落原始字节。
func ExtractAssistantText(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	text := string(raw)
	if looksLikeSSE(text) {
		return extractFromSSE(text)
	}
	return extractFromJSON(raw)
}

func looksLikeSSE(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:")
	}
	return false
}

func extractFromSSE(text string) (string, bool) {
	var b strings.Builder
	found := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if piece, ok := textFromEvent(event); ok {
			b.WriteString(piece)
			found = true
		}
	}
	if !found {
		return "", false
	}
	return b.String(), true
}

func textFromEvent(event map[string]any) (string, bool) {
	// OpenAI chat completions 增量。
	if choices, ok := event["choices"].([]any); ok {
		var b strings.Builder
		for _, item := range choices {
			choice, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if delta, ok := choice["delta"].(map[string]any); ok {
				if s, ok := delta["content"].(string); ok && s != "" {
					b.WriteString(s)
				}
				// responses 协议把文本放在 delta.text。
				if s, ok := delta["text"].(string); ok && s != "" {
					b.WriteString(s)
				}
			}
			if msg, ok := choice["message"].(map[string]any); ok {
				if s, ok := msg["content"].(string); ok && s != "" {
					b.WriteString(s)
				}
			}
			if s, ok := choice["text"].(string); ok && s != "" {
				b.WriteString(s)
			}
		}
		if b.Len() > 0 {
			return b.String(), true
		}
	}
	// OpenAI responses 流式：response.output_text.delta。
	if event["type"] == "response.output_text.delta" {
		if s, ok := event["delta"].(string); ok && s != "" {
			return s, true
		}
	}
	// Anthropic 流式：content_block_delta / content_block_start。
	if event["type"] == "content_block_delta" {
		if delta, ok := event["delta"].(map[string]any); ok {
			if s, ok := delta["text"].(string); ok && s != "" {
				return s, true
			}
		}
	}
	if event["type"] == "content_block_start" {
		if block, ok := event["content_block"].(map[string]any); ok {
			if s, ok := block["text"].(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func extractFromJSON(raw []byte) (string, bool) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", false
	}
	var b strings.Builder

	// OpenAI chat completions。
	if choices, ok := root["choices"].([]any); ok {
		for _, item := range choices {
			choice, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if msg, ok := choice["message"].(map[string]any); ok {
				b.WriteString(jsonContentText(msg["content"]))
			}
			if s, ok := choice["text"].(string); ok {
				b.WriteString(s)
			}
		}
	}
	// Anthropic messages。
	if b.Len() == 0 {
		if content, ok := root["content"]; ok {
			b.WriteString(jsonContentText(content))
		}
	}
	// OpenAI responses。
	if b.Len() == 0 {
		if output, ok := root["output"].([]any); ok {
			for _, item := range output {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				b.WriteString(jsonContentText(entry["content"]))
				if s, ok := entry["text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		if s, ok := root["output_text"].(string); ok {
			b.WriteString(s)
		}
	}

	if b.Len() == 0 {
		return "", false
	}
	return b.String(), true
}

// jsonContentText 处理 content 的两种形态：纯字符串，或 [{type,text}] 数组。
func jsonContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var b strings.Builder
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := block["text"].(string); ok {
				b.WriteString(s)
			}
		}
		return b.String()
	default:
		return ""
	}
}
