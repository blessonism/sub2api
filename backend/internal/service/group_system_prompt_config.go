package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
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
		if !domain.IsGroupPlatform(platform) {
			return cfg, infraerrors.BadRequest("INVALID_SYSTEM_PROMPT_INJECTION", "system prompt injection requires a registered group platform")
		}
		if strings.TrimSpace(cfg.Content) == "" {
			return cfg, infraerrors.BadRequest("INVALID_SYSTEM_PROMPT_INJECTION", "system prompt injection content cannot be blank when enabled")
		}
	}
	return cfg, nil
}
