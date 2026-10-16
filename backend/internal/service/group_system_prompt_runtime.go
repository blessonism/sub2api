package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	SystemPromptProtocolResponses       = "responses"
	SystemPromptProtocolChatCompletions = "chat_completions"
	SystemPromptProtocolMessages        = "messages"
	SystemPromptProtocolGemini          = "gemini"
	SystemPromptProtocolSystemOne       = "systemone"
)

// PrepareGroupSystemPromptBody 在协议转换前清理客户端指令，不写入分组正文。
func PrepareGroupSystemPromptBody(body []byte, protocol string, cfg GroupSystemPromptInjection) ([]byte, error) {
	if !cfg.Enabled {
		return body, nil
	}
	payload, err := parseGroupSystemPromptBody(body, protocol, cfg.Mode)
	if err != nil {
		return nil, err
	}
	if cfg.Mode != "replace" {
		return body, nil
	}
	if protocol == SystemPromptProtocolSystemOne {
		return prepareGroupSystemPromptSystemOne(payload)
	}
	if protocol == SystemPromptProtocolGemini {
		return prepareGroupSystemPromptGemini(payload, cfg)
	}
	delete(payload, "instructions")
	identity := json.RawMessage(nil)
	if protocol == SystemPromptProtocolMessages {
		identity = groupSystemPromptReplacementIdentity(payload["system"])
	}
	if len(identity) == 0 {
		delete(payload, "system")
	} else {
		payload["system"] = identity
	}
	for _, field := range []string{"messages", "input"} {
		raw := payload[field]
		if field == "input" && len(raw) > 0 && raw[0] == '{' && isGroupSystemPromptMessage(raw) {
			payload[field] = json.RawMessage(`[]`)
			continue
		}
		if len(raw) == 0 || raw[0] != '[' {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		kept := make([]json.RawMessage, 0, len(items))
		for _, item := range items {
			if !isGroupSystemPromptMessage(item) {
				kept = append(kept, item)
			}
		}
		payload[field], _ = json.Marshal(kept)
	}
	return json.Marshal(payload)
}

// InjectGroupSystemPromptBody 仅用于最终出站报文。replace 的客户端清理已在
// Prepare 阶段完成，此处追加正文，保留服务器生成的全局指令和协议身份。
func InjectGroupSystemPromptBody(body []byte, protocol string, cfg GroupSystemPromptInjection) ([]byte, error) {
	if !cfg.Enabled {
		return body, nil
	}
	payload, err := parseGroupSystemPromptBody(body, protocol, cfg.Mode)
	if err != nil {
		return nil, err
	}
	prepend := cfg.Mode == "prepend"
	switch protocol {
	case SystemPromptProtocolSystemOne:
		return injectGroupSystemPromptSystemOne(payload, cfg)
	case SystemPromptProtocolGemini:
		return injectGroupSystemPromptGemini(payload, cfg)
	case SystemPromptProtocolResponses:
		var items []json.RawMessage
		raw := payload["input"]
		if len(raw) > 0 && raw[0] == '[' {
			_ = json.Unmarshal(raw, &items)
		} else if len(raw) > 0 && raw[0] == '{' {
			// Legacy/native 入口允许单个对象；只有追加指令需要将其提升为数组。
			items = []json.RawMessage{raw}
		}
		last := -1
		for i, item := range items {
			if isGroupSystemPromptMessage(item) {
				last = i
			}
		}
		if !prepend && last >= 0 {
			message, _ := json.Marshal(map[string]any{"type": "message", "role": "developer", "content": []map[string]string{{"type": "input_text", "text": cfg.Content}}})
			payload["input"], _ = json.Marshal(insertGroupSystemPromptItem(items, last+1, message))
		} else {
			var current string
			_ = json.Unmarshal(payload["instructions"], &current)
			payload["instructions"], _ = json.Marshal(joinGroupSystemPrompt(current, cfg.Content, prepend))
		}
	case SystemPromptProtocolChatCompletions:
		var items []json.RawMessage
		_ = json.Unmarshal(payload["messages"], &items)
		index := 0
		for i, item := range items {
			if isGroupSystemPromptMessage(item) {
				index = i
				if prepend {
					break
				}
				index++
			}
		}
		message, _ := json.Marshal(map[string]string{"role": "system", "content": cfg.Content})
		payload["messages"], _ = json.Marshal(insertGroupSystemPromptItem(items, index, message))
	case SystemPromptProtocolMessages:
		raw := payload["system"]
		if len(raw) > 0 && raw[0] == '[' {
			var items []json.RawMessage
			_ = json.Unmarshal(raw, &items)
			index := len(items)
			if prepend {
				index = 0
				// 只越过连续的协议身份块，不能把普通用户指令当成身份。
				for index < len(items) && isGroupSystemPromptIdentityBlock(items[index]) {
					index++
				}
			}
			block, _ := json.Marshal(map[string]string{"type": "text", "text": cfg.Content})
			payload["system"], _ = json.Marshal(insertGroupSystemPromptItem(items, index, block))
		} else {
			var current string
			_ = json.Unmarshal(raw, &current)
			if prefix, suffix := groupSystemPromptIdentityPrefix(current); prepend && prefix != "" {
				payload["system"], _ = json.Marshal(prefix + "\n\n" + cfg.Content + suffix)
			} else {
				payload["system"], _ = json.Marshal(joinGroupSystemPrompt(current, cfg.Content, prepend))
			}
		}
	}
	return json.Marshal(payload)
}

func parseGroupSystemPromptBody(body []byte, protocol, mode string) (map[string]json.RawMessage, error) {
	switch protocol {
	case SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions, SystemPromptProtocolMessages, SystemPromptProtocolGemini, SystemPromptProtocolSystemOne:
	default:
		return nil, fmt.Errorf("unsupported system prompt protocol %q", protocol)
	}
	switch mode {
	case "", "prepend", "append", "replace":
	default:
		return nil, fmt.Errorf("unsupported system prompt mode %q", mode)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid system prompt request: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("system prompt request must be a JSON object")
	}
	if protocol == SystemPromptProtocolSystemOne {
		return payload, validateGroupSystemPromptSystemOne(payload)
	}
	if protocol == SystemPromptProtocolGemini {
		if err := validateGroupSystemPromptGemini(payload); err != nil {
			return nil, err
		}
		return payload, nil
	}
	// 这里只校验本协议需要拼接的字段。其他字段交给已有入口和上游处理，
	// 不能因开启注入而收紧无关扩展字段的兼容性。
	for _, field := range []string{"instructions", "system", "messages", "input"} {
		if raw, exists := payload[field]; exists {
			payload[field] = bytes.TrimSpace(raw)
		}
	}
	fields := []string{"instructions", "input"}
	switch protocol {
	case SystemPromptProtocolChatCompletions:
		fields = []string{"messages"}
	case SystemPromptProtocolMessages:
		fields = []string{"system"}
	}
	for _, field := range fields {
		raw := payload[field]
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			continue
		}
		allowString := field != "messages"
		allowArray := field != "instructions"
		if raw[0] == '"' && allowString || raw[0] == '[' && allowArray || field == "input" && raw[0] == '{' {
			continue
		}
		return nil, fmt.Errorf("invalid system prompt request field %q", field)
	}
	return payload, nil
}

// 非消息项即使带有 role，也不能按客户端指令删除。
func isGroupSystemPromptMessage(item json.RawMessage) bool {
	var message struct {
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if json.Unmarshal(item, &message) != nil || (message.Type != "" && message.Type != "message") {
		return false
	}
	return message.Role == "system" || message.Role == "developer"
}

func insertGroupSystemPromptItem(items []json.RawMessage, index int, item json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(items)+1)
	out = append(out, items[:index]...)
	out = append(out, item)
	return append(out, items[index:]...)
}

func joinGroupSystemPrompt(current, content string, prepend bool) string {
	if current == "" {
		return content
	}
	if prepend {
		return content + "\n\n" + current
	}
	return current + "\n\n" + content
}

func isGroupSystemPromptIdentityBlock(raw json.RawMessage) bool {
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &block) != nil || block.Type != "text" {
		return false
	}
	text := strings.TrimSpace(block.Text)
	if strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) && strings.Contains(text, claudeCodeEntrypointMarker) {
		return true
	}
	prefix, suffix := groupSystemPromptIdentityPrefix(text)
	return prefix != "" && strings.TrimSpace(suffix) == ""
}

func groupSystemPromptIdentityPrefix(text string) (string, string) {
	// 使用完整身份句，而非 hasClaudeCodePrefix 的宽泛前缀，防止用户的扩充正文
	// 被整个视为身份块。最后一个模板是长提示词片段，不是独立身份。
	for _, identity := range claudeCodeSystemPrompts[:len(claudeCodeSystemPrompts)-1] {
		if strings.HasPrefix(text, identity) {
			suffix := text[len(identity):]
			if suffix == "" || strings.HasPrefix(suffix, "\n") || strings.HasPrefix(suffix, " ") || strings.HasPrefix(suffix, "\t") {
				return identity, suffix
			}
		}
	}
	return "", text
}

// 真实 Claude Code 客户端不会经过模拟身份流程，replace 也要保留协议身份。
// 混在身份块里的客户端业务指令必须移除，不能因为有身份前缀而整体保留。
func groupSystemPromptReplacementIdentity(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var text string
		_ = json.Unmarshal(raw, &text)
		if prefix, _ := groupSystemPromptIdentityPrefix(text); prefix != "" {
			identity, _ := json.Marshal(claudeCodeSystemPrompt)
			return identity
		}
		return nil
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	kept := make([]json.RawMessage, 0, len(blocks))
	identityKept := false
	for _, rawBlock := range blocks {
		var block map[string]json.RawMessage
		_ = json.Unmarshal(rawBlock, &block)
		var text, blockType string
		_ = json.Unmarshal(block["text"], &text)
		_ = json.Unmarshal(block["type"], &blockType)
		if blockType != "text" {
			continue
		}
		if billing := groupSystemPromptBillingIdentity(text); billing != "" {
			block["text"], _ = json.Marshal(billing)
			identity, _ := json.Marshal(block)
			kept = append(kept, identity)
			continue
		}
		if prefix, _ := groupSystemPromptIdentityPrefix(text); prefix != "" && !identityKept {
			block["text"], _ = json.Marshal(claudeCodeSystemPrompt)
			identity, _ := json.Marshal(block)
			kept = append(kept, identity)
			identityKept = true
		}
	}
	if len(kept) == 0 {
		return nil
	}
	identity, _ := json.Marshal(kept)
	return identity
}

// 计费块也可能混入业务正文。replace 只保留协议已知字段及其受限的标量值，
// 不把“具有 billing 前缀”当成保留整段客户端文本的理由。
var groupSystemPromptBillingFields = map[string]*regexp.Regexp{
	"cc_version":    regexp.MustCompile(`^\d+\.\d+\.\d+(?:\.[a-zA-Z0-9]+)?$`),
	"cc_entrypoint": regexp.MustCompile(`^[a-zA-Z0-9_-]+$`),
	"cch":           regexp.MustCompile(`^[a-fA-F0-9]+$`),
}

func groupSystemPromptBillingIdentity(text string) string {
	text, ok := strings.CutPrefix(text, claudeCodeBillingHeaderPrefix+":")
	if !ok {
		return ""
	}
	fields := make([]string, 0, len(groupSystemPromptBillingFields))
	seen := make(map[string]bool, len(groupSystemPromptBillingFields))
	for _, segment := range strings.Split(text, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(segment), "=")
		pattern := groupSystemPromptBillingFields[key]
		if !found || pattern == nil || !pattern.MatchString(value) {
			break
		}
		if !seen[key] {
			fields = append(fields, key+"="+value+";")
			seen[key] = true
		}
	}
	if !seen["cc_entrypoint"] {
		return ""
	}
	return claudeCodeBillingHeaderPrefix + ": " + strings.Join(fields, " ")
}
