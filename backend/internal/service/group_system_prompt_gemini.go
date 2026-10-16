package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

var groupSystemPromptGeminiWrappers = []string{"request", "generateContentRequest", "generate_content_request"}

// countTokens 允许包裹 GenerateContentRequest，估算与身份组装使用内部请求。
func unwrapGroupSystemPromptGeminiBody(body []byte) []byte {
	for _, field := range groupSystemPromptGeminiWrappers {
		if nested := gjson.GetBytes(body, field); nested.IsObject() {
			return unwrapGroupSystemPromptGeminiBody([]byte(nested.Raw))
		}
	}
	return body
}

func estimateGeminiCountTokensForGroup(ctx context.Context, c *gin.Context, body []byte) int {
	if groupSystemPromptConfig(ctx, c).Enabled {
		body = unwrapGroupSystemPromptGeminiBody(body)
	}
	return estimateGeminiCountTokens(body)
}

func validateGroupSystemPromptGemini(payload map[string]json.RawMessage) error {
	for _, field := range groupSystemPromptGeminiWrappers {
		if raw, ok := payload[field]; ok {
			if _, err := parseGroupSystemPromptBody(raw, SystemPromptProtocolGemini, "append"); err != nil {
				return fmt.Errorf("invalid Gemini %s: %w", field, err)
			}
		}
	}
	for _, field := range []string{"systemInstruction", "system_instruction"} {
		raw := bytes.TrimSpace(payload[field])
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			continue
		}
		var content map[string]json.RawMessage
		if err := json.Unmarshal(raw, &content); err != nil || content == nil {
			return fmt.Errorf("invalid Gemini %s", field)
		}
		if parts, ok := content["parts"]; ok {
			var items []json.RawMessage
			if err := json.Unmarshal(parts, &items); err != nil {
				return fmt.Errorf("invalid Gemini %s.parts", field)
			}
		}
	}
	return nil
}

func prepareGroupSystemPromptGemini(payload map[string]json.RawMessage, cfg GroupSystemPromptInjection) ([]byte, error) {
	for _, field := range []string{"systemInstruction", "system_instruction", "instructions", "system"} {
		delete(payload, field)
	}
	for _, field := range groupSystemPromptGeminiWrappers {
		if raw, ok := payload[field]; ok {
			next, err := PrepareGroupSystemPromptBody(raw, SystemPromptProtocolGemini, cfg)
			if err != nil {
				return nil, err
			}
			payload[field] = next
		}
	}
	return json.Marshal(payload)
}

func injectGroupSystemPromptGemini(payload map[string]json.RawMessage, cfg GroupSystemPromptInjection) ([]byte, error) {
	for _, field := range groupSystemPromptGeminiWrappers {
		if raw, ok := payload[field]; ok {
			next, err := InjectGroupSystemPromptBody(raw, SystemPromptProtocolGemini, cfg)
			if err != nil {
				return nil, err
			}
			payload[field] = next
			return json.Marshal(payload)
		}
	}
	content := map[string]json.RawMessage{}
	var parts []json.RawMessage
	for _, field := range []string{"systemInstruction", "system_instruction"} {
		var existing map[string]json.RawMessage
		_ = json.Unmarshal(payload[field], &existing)
		for key, raw := range existing {
			if key != "parts" {
				if _, present := content[key]; !present {
					content[key] = raw
				}
			}
		}
		var items []json.RawMessage
		_ = json.Unmarshal(existing["parts"], &items)
		parts = append(parts, items...)
		delete(payload, field)
	}
	index := len(parts)
	if cfg.Mode == "prepend" {
		index = 0
		// Antigravity 的固定身份必须继续位于客户端指令及分组正文之前。
		identity := strings.TrimSpace(antigravity.GetDefaultIdentityPatch())
		for index < len(parts) {
			var part map[string]json.RawMessage
			_ = json.Unmarshal(parts[index], &part)
			var text string
			_ = json.Unmarshal(part["text"], &text)
			if strings.TrimSpace(text) == identity {
				index++
				continue
			}
			if suffix, ok := strings.CutPrefix(text, identity+"\n"); ok {
				part["text"], _ = json.Marshal(identity + "\n\n" + cfg.Content + "\n" + suffix)
				parts[index], _ = json.Marshal(part)
				content["parts"], _ = json.Marshal(parts)
				payload["systemInstruction"], _ = json.Marshal(content)
				return json.Marshal(payload)
			}
			break
		}
	}
	part, _ := json.Marshal(map[string]string{"text": cfg.Content})
	content["parts"], _ = json.Marshal(insertGroupSystemPromptItem(parts, index, part))
	payload["systemInstruction"], _ = json.Marshal(content)
	return json.Marshal(payload)
}

// Gemini 兼容网关的签名修复会重新转换 canonical Claude body；每次转换均
// 从该基础输入生成一份正文，不能在已经注入的 Gemini 报文上再次追加。
func convertClaudeMessagesToGeminiForForward(ctx context.Context, c *gin.Context, body []byte) ([]byte, error) {
	next, err := convertClaudeMessagesToGeminiGenerateContent(body)
	if err != nil {
		return nil, err
	}
	return injectGroupSystemPromptForForward(ctx, c, next, SystemPromptProtocolGemini)
}
