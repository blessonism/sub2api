package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// normalizeGroupSystemPromptInjection 只校验空白，不修改管理员正文。
func normalizeGroupSystemPromptInjection(platform string, cfg GroupSystemPromptInjection) (GroupSystemPromptInjection, error) {
	cfg = cfg.WithDefaults()
	switch cfg.Mode {
	case "prepend", "append", "replace":
	default:
		return cfg, infraerrors.BadRequest("INVALID_SYSTEM_PROMPT_INJECTION", "system prompt injection mode must be prepend, append or replace")
	}
	if cfg.Enabled {
		if platform != PlatformOpenAI && platform != PlatformAnthropic {
			return cfg, infraerrors.BadRequest("INVALID_SYSTEM_PROMPT_INJECTION", "system prompt injection is only supported for openai and anthropic groups")
		}
		if strings.TrimSpace(cfg.Content) == "" {
			return cfg, infraerrors.BadRequest("INVALID_SYSTEM_PROMPT_INJECTION", "system prompt injection content cannot be blank when enabled")
		}
	}
	return cfg, nil
}
