package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupSystemPromptContextFreezesOriginalGroup(t *testing.T) {
	group := &Group{ID: 1, Platform: PlatformOpenAI, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "original group"}}
	ctx := WithGroupSystemPromptInjection(context.Background(), group)
	group.SystemPromptInjection.Content = "new configuration"
	fallback := &Group{ID: 2, Platform: PlatformAnthropic, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "fallback"}}
	ctx = context.WithValue(ctx, ctxkey.Group, fallback)
	ctx = WithGroupSystemPromptInjection(ctx, fallback)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{Group: fallback})
	require.Equal(t, "original group", groupSystemPromptConfig(ctx, c).Content)
	newRequest := WithGroupSystemPromptInjection(context.Background(), group)
	require.Equal(t, "new configuration", groupSystemPromptConfig(newRequest, c).Content)
}

func TestGroupSystemPromptUnsupportedGroupDoesNotInheritTargetPolicy(t *testing.T) {
	for _, platform := range []string{PlatformComposite, PlatformGemini, PlatformGrok, PlatformAntigravity} {
		t.Run(platform, func(t *testing.T) {
			group := &Group{Platform: platform, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "unsupported"}}
			ctx := WithGroupSystemPromptInjection(context.Background(), group)
			ctx = context.WithValue(ctx, ctxkey.Group, &Group{Platform: PlatformOpenAI, SystemPromptInjection: group.SystemPromptInjection})
			require.False(t, groupSystemPromptConfig(ctx, nil).Enabled)
		})
	}
}

func TestGroupSystemPromptPrepareContextIsPerAttempt(t *testing.T) {
	group := &Group{Platform: PlatformOpenAI, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "group"}}
	ctx := WithGroupSystemPromptInjection(context.Background(), group)
	body := []byte(`{"instructions":"client","input":"hello"}`)
	for attempt := 0; attempt < 2; attempt++ {
		attemptCtx, prepared, err := prepareGroupSystemPromptForForward(ctx, nil, body, SystemPromptProtocolResponses)
		require.NoError(t, err)
		require.JSONEq(t, `{"input":"hello"}`, string(prepared))
		// 后续协议转换/服务调用不能把服务器新生成的全局指令误删。
		globalBody := []byte(`{"instructions":"server global","input":"hello"}`)
		_, prepared, err = prepareGroupSystemPromptForForward(attemptCtx, nil, globalBody, SystemPromptProtocolResponses)
		require.NoError(t, err)
		outbound, err := injectGroupSystemPromptForForward(attemptCtx, nil, prepared, SystemPromptProtocolResponses)
		require.NoError(t, err)
		require.JSONEq(t, `{"instructions":"server global\n\ngroup","input":"hello"}`, string(outbound))
	}
	require.JSONEq(t, `{"instructions":"client","input":"hello"}`, string(body))
}

func TestGroupSystemPromptParsedKeepsCanonicalBody(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","system":"client","messages":[{"role":"user","content":"hello"}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	group := &Group{Platform: PlatformAnthropic, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "group"}}
	ctx := WithGroupSystemPromptInjection(context.Background(), group)
	_, attempt, err := prepareGroupSystemPromptParsed(ctx, nil, parsed)
	require.NoError(t, err)
	require.NotSame(t, parsed, attempt)
	require.Equal(t, string(body), string(parsed.Body.Bytes()))
	require.NotContains(t, string(attempt.Body.Bytes()), "client")
}

func TestGroupSystemPromptSkipsStandaloneImageRequests(t *testing.T) {
	group := &Group{Platform: PlatformOpenAI, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "group"}}
	ctx := withOpenAIImagesSelfBuiltRequest(WithGroupSystemPromptInjection(context.Background(), group))
	body := []byte(`{"instructions":"image protocol","input":"draw a cat"}`)
	got, err := injectGroupSystemPromptForForward(ctx, nil, body, SystemPromptProtocolResponses)
	require.NoError(t, err)
	require.Equal(t, string(body), string(got))
}
