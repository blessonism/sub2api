package service

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var (
	conversationEmailPattern       = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	conversationPhonePattern       = regexp.MustCompile(`(?m)(?:\+?\d[\d\s().-]{7,}\d)`)
	conversationAPIKeyPattern      = regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9_\-]{16,}|rk-[A-Za-z0-9_\-]{16,}|api[_-]?key\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?|token\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?)`)
	conversationBearerTokenPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._\-]{16,}`)
	conversationURLPattern         = regexp.MustCompile(`https?://[^\s"'<>]+`)
)

func redactConversationMessages(messages []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		var decoded any
		if err := json.Unmarshal(message, &decoded); err != nil {
			out = append(out, json.RawMessage([]byte(jsonString(redactSensitiveText(string(message))))))
			continue
		}
		redacted := redactConversationValue(decoded)
		raw, err := json.Marshal(redacted)
		if err != nil {
			out = append(out, message)
			continue
		}
		out = append(out, raw)
	}
	return out
}

func redactConversationValue(value any) any {
	switch typed := value.(type) {
	case string:
		return redactSensitiveText(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactConversationValue(item))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = redactConversationValue(item)
		}
		return out
	default:
		return value
	}
}

func redactSensitiveText(input string) string {
	out := conversationEmailPattern.ReplaceAllString(input, "[REDACTED_EMAIL]")
	out = conversationBearerTokenPattern.ReplaceAllString(out, "Bearer [REDACTED_TOKEN]")
	out = conversationAPIKeyPattern.ReplaceAllStringFunc(out, func(match string) string {
		lower := strings.ToLower(match)
		switch {
		case strings.HasPrefix(lower, "api"):
			return "api_key=[REDACTED_TOKEN]"
		case strings.HasPrefix(lower, "token"):
			return "token=[REDACTED_TOKEN]"
		default:
			return "[REDACTED_TOKEN]"
		}
	})
	out = conversationURLPattern.ReplaceAllStringFunc(out, redactURLQuerySecrets)
	out = conversationPhonePattern.ReplaceAllStringFunc(out, func(match string) string {
		digits := 0
		for _, r := range match {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < 8 {
			return match
		}
		return "[REDACTED_PHONE]"
	})
	return out
}

func redactURLQuerySecrets(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}
	values := parsed.Query()
	changed := false
	for key := range values {
		if isSecretQueryKey(key) {
			values.Set(key, "[REDACTED]")
			changed = true
		}
	}
	if !changed {
		return rawURL
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func isSecretQueryKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return normalized == "key" ||
		normalized == "api_key" ||
		normalized == "apikey" ||
		normalized == "token" ||
		normalized == "access_token" ||
		normalized == "refresh_token" ||
		normalized == "secret" ||
		normalized == "signature" ||
		normalized == "sig"
}

func jsonString(value string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(raw)
}
