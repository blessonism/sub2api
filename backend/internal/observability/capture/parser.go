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
	// JSON 响应体一定以容器开头，先按首字符排除，避免把正文里的 "data:" 误当 SSE。
	trimmed := strings.TrimLeft(text, " \t\r\n")
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return false
	}
	// 流式响应常以 SSE 注释行（": ping"）或空事件开头，
	// 只看第一行会把整条流误判成非 SSE，随后回退原始字节，
	// 正文就变成 "event: xxx data: ..." 的噪声。
	scanned := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, ":") {
			return true
		}
		scanned++
		if scanned >= 64 {
			break
		}
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
	// OpenAI responses 流式：增量与收尾两种形态都要收。
	if event["type"] == "response.output_text.delta" {
		if s, ok := event["delta"].(string); ok && s != "" {
			return s, true
		}
	}
	if event["type"] == "response.output_text.done" {
		if s, ok := event["text"].(string); ok && s != "" {
			return s, true
		}
	}
	// 部分实现把文本放在 content_part 的 delta 里。
	if event["type"] == "response.content_part.delta" {
		if delta, ok := event["delta"].(map[string]any); ok {
			if s, ok := delta["text"].(string); ok && s != "" {
				return s, true
			}
		}
		if s, ok := event["text"].(string); ok && s != "" {
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

// textFromSSELine 从单行 SSE 里抽正文片段。
// fullText 为 true 表示这条事件携带的是整段文本（done/completed），
// 调用方据此避免与增量事件重复累加。
// parseSSELine 一次解析出正文与收尾原因。
// 之前正文和 finish 各解析一遍同一行，流式响应每个事件都要多一次 json.Unmarshal。
func parseSSELine(line []byte) (text string, fullText bool, finish string) {
	trimmed := strings.TrimSpace(string(line))
	if !strings.HasPrefix(trimmed, "data:") {
		return "", false, ""
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" || payload == "[DONE]" {
		return "", false, ""
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return "", false, ""
	}
	finish = finishFromEvent(event)
	piece, ok := textFromEvent(event)
	if !ok {
		return "", false, finish
	}
	if event["type"] == "response.output_text.done" {
		return piece, true, finish
	}
	return piece, false, finish
}

func textFromSSELine(line []byte) (string, bool) {
	trimmed := strings.TrimSpace(string(line))
	if !strings.HasPrefix(trimmed, "data:") {
		return "", false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" || payload == "[DONE]" {
		return "", false
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return "", false
	}
	piece, ok := textFromEvent(event)
	if !ok {
		return "", false
	}
	if event["type"] == "response.output_text.done" {
		return piece, true
	}
	return piece, false
}

// finishFromSSELine 从一条 SSE data 行里抽上游的收尾原因。
//
// 目的：把"模型自己停/被 max tokens 截"和"观测层缓冲截断"区分开。
// 只保留有限的短标识，不外发原文。
func finishFromSSELine(line []byte) string {
	_, _, finish := parseSSELine(line)
	return finish
}

func finishFromEvent(event map[string]any) string {
	// OpenAI responses：`response.incomplete` 优先，它比 completed 更能说明原因。
	if resp, ok := event["response"].(map[string]any); ok {
		if inc, ok := resp["incomplete_details"].(map[string]any); ok {
			if reason, ok := inc["reason"].(string); ok && reason != "" {
				return "incomplete:" + reason
			}
		}
	}
	if choices, ok := event["choices"].([]any); ok {
		for _, item := range choices {
			choice, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
				return reason
			}
		}
	}
	if reason, ok := event["stop_reason"].(string); ok && reason != "" {
		return reason
	}
	if delta, ok := event["delta"].(map[string]any); ok {
		if reason, ok := delta["stop_reason"].(string); ok && reason != "" {
			return reason
		}
	}
	if msg, ok := event["message"].(map[string]any); ok {
		if reason, ok := msg["stop_reason"].(string); ok && reason != "" {
			return reason
		}
	}
	switch event["type"] {
	case "response.incomplete":
		return "incomplete"
	case "response.completed", "message_stop":
		return "stop"
	}
	return ""
}

// FinishReasonFromJSON 从非流式响应体里抽收尾原因，补上 SSE 之外的那条路径。
func FinishReasonFromJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	return finishFromEvent(root)
}
