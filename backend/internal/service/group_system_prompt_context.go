package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

type groupSystemPromptContextKey struct{}
type groupSystemPromptPreparedKey struct{}

// WithGroupSystemPromptInjection 在鉴权时冻结原绑定分组的配置。兜底分组、
// 账号切换和 WS 每轮计费上下文刷新都不能更换本次请求的注入策略。
func WithGroupSystemPromptInjection(ctx context.Context, group *Group) context.Context {
	if _, bound := ctx.Value(groupSystemPromptContextKey{}).(GroupSystemPromptInjection); bound {
		return ctx
	}
	var cfg GroupSystemPromptInjection
	if group != nil && domain.IsGroupPlatform(group.Platform) {
		cfg = group.SystemPromptInjection
	}
	return context.WithValue(ctx, groupSystemPromptContextKey{}, cfg)
}

func groupSystemPromptConfig(ctx context.Context, c *gin.Context) GroupSystemPromptInjection {
	if cfg, ok := ctx.Value(groupSystemPromptContextKey{}).(GroupSystemPromptInjection); ok {
		return cfg
	}
	// 服务层直接调用（含测试）也使用原 API Key 分组，而非当前调度组。
	var group *Group
	if c != nil {
		if value, ok := c.Get("api_key"); ok {
			if key, ok := value.(*APIKey); ok && key != nil {
				group = key.Group
			}
		}
	}
	if group == nil {
		group, _ = ctx.Value(ctxkey.Group).(*Group)
	}
	bound := WithGroupSystemPromptInjection(ctx, group)
	cfg, _ := bound.Value(groupSystemPromptContextKey{}).(GroupSystemPromptInjection)
	return cfg
}

// prepareGroupSystemPromptForForward 仅处理客户端指令。prepared 标记随当前
// 转发调用的 context 传递，不写回 Gin/鉴权上下文，下一账号仍从原请求开始。
func prepareGroupSystemPromptForForward(ctx context.Context, c *gin.Context, body []byte, protocol string) (context.Context, []byte, error) {
	cfg := groupSystemPromptConfig(ctx, c)
	if !cfg.Enabled {
		return ctx, body, nil
	}
	ctx = context.WithValue(ctx, groupSystemPromptContextKey{}, cfg)
	if prepared, _ := ctx.Value(groupSystemPromptPreparedKey{}).(bool); prepared {
		return ctx, body, nil
	}
	next, err := PrepareGroupSystemPromptBody(body, protocol, cfg)
	if err != nil {
		return ctx, nil, fmt.Errorf("prepare group system prompt: %w", err)
	}
	return context.WithValue(ctx, groupSystemPromptPreparedKey{}, true), next, nil
}

func injectGroupSystemPromptForForward(ctx context.Context, c *gin.Context, body []byte, protocol string) ([]byte, error) {
	// 独立图片接口借用 Responses 构建器，但不属于文本对话注入范围。
	if isOpenAIImagesSelfBuiltRequest(ctx) {
		return body, nil
	}
	next, err := InjectGroupSystemPromptBody(body, protocol, groupSystemPromptConfig(ctx, c))
	if err != nil {
		return nil, fmt.Errorf("inject group system prompt: %w", err)
	}
	return next, nil
}

func prepareGroupSystemPromptParsed(ctx context.Context, c *gin.Context, parsed *ParsedRequest) (context.Context, *ParsedRequest, error) {
	if !groupSystemPromptConfig(ctx, c).Enabled {
		return ctx, parsed, nil
	}
	// 客户端身份从原报文判断，不能受 replace 删除业务指令的影响。
	userAgent := ""
	if c != nil {
		userAgent = c.GetHeader("User-Agent")
	}
	if IsClaudeCodeClient(ctx) || isClaudeCodeClient(userAgent, parsed.MetadataUserID) ||
		(parsed.MetadataUserID != "" && systemHasBillingAttributionBlock(parsed.Body.Bytes())) {
		ctx = SetClaudeCodeClient(ctx, true)
	}
	nextCtx, body, err := prepareGroupSystemPromptForForward(ctx, c, parsed.Body.Bytes(), SystemPromptProtocolMessages)
	if err != nil {
		return ctx, nil, err
	}
	// ParsedRequest 也属于调用方的 canonical 输入；只修改本次尝试的副本。
	nextParsed, err := parsed.CloneForBody(body)
	return nextCtx, nextParsed, err
}
