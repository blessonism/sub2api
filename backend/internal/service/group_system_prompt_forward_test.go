//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const groupForwardPrompt = "group-policy-unique-marker"

func groupPromptForwardContext(platform, mode, path string, body []byte) (context.Context, *gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	group := &Group{ID: 501, Platform: platform, SystemPromptInjection: GroupSystemPromptInjection{
		Enabled: true, Mode: mode, Content: groupForwardPrompt,
	}}
	ctx := WithGroupSystemPromptInjection(context.Background(), group)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "third-party-client/1.0")
	c.Set("api_key", &APIKey{ID: 502, Group: group})
	return ctx, c, recorder
}

func groupPromptClaudeAccount(kind string) *Account {
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra["anthropic_passthrough"] = kind == "passthrough"
	if kind == "oauth" {
		account.Type = AccountTypeOAuth
		account.Credentials = map[string]any{"access_token": "mock-oauth-token"}
	}
	return account
}

func groupPromptClaudeService(upstream *httpUpstreamRecorder) *GatewayService {
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	return &GatewayService{
		cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg), httpUpstream: upstream,
		rateLimitService: &RateLimitService{}, deferredService: &DeferredService{},
	}
}

func groupPromptJSONResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(body))}
}

func groupPromptResponsesResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.created","response":{"id":"resp_prompt","model":"gpt-5.4","status":"in_progress","output":[]}}` + "\n\n" +
				`data: {"type":"response.output_text.delta","item_id":"msg_prompt","output_index":0,"content_index":0,"delta":"ok"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"id":"resp_prompt","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_prompt","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}` + "\n\n"))}
}

func groupPromptChatResponse(stream bool) *http.Response {
	if !stream {
		return groupPromptJSONResponse(`{"id":"chatcmpl_prompt","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`)
	}
	return &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"id":"chatcmpl_prompt","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}` + "\n\n" +
				`data: {"id":"chatcmpl_prompt","object":"chat.completion.chunk","model":"gpt-5.4","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}` + "\n\n" + "data: [DONE]\n\n"))}
}

// 只读取最终协议中的指令区，避免把降级到普通 user 消息的正文误判为成功。
func groupPromptFinalInstructions(body []byte, protocol string) string {
	root := gjson.ParseBytes(body)
	var texts []string
	if protocol == SystemPromptProtocolMessages {
		return groupPromptText(root.Get("system"))
	}
	if protocol == SystemPromptProtocolResponses {
		texts = append(texts, root.Get("instructions").String())
	}
	field := "messages"
	if protocol == SystemPromptProtocolResponses {
		field = "input"
	}
	for _, item := range root.Get(field).Array() {
		if item.Get("type").String() != "" && item.Get("type").String() != "message" {
			continue
		}
		role := item.Get("role").String()
		if role == "system" || role == "developer" {
			texts = append(texts, groupPromptText(item.Get("content")))
		}
	}
	return strings.Join(texts, "\n")
}

func groupPromptText(value gjson.Result) string {
	if value.Type == gjson.String {
		return value.String()
	}
	var texts []string
	for _, block := range value.Array() {
		texts = append(texts, block.Get("text").String())
	}
	return strings.Join(texts, "\n")
}

func assertGroupPromptFinalBody(t *testing.T, body []byte, protocol, mode string, originalInSystem bool) {
	t.Helper()
	require.True(t, json.Valid(body), string(body))
	require.Equal(t, 1, strings.Count(string(body), groupForwardPrompt), "正文在一次转发中只能注入一次: %s", body)
	instructions := groupPromptFinalInstructions(body, protocol)
	require.Contains(t, instructions, groupForwardPrompt, "分组正文必须保留最终 system 语义")
	if mode == "replace" {
		for _, marker := range []string{"client-system-marker", "client-developer-marker", "client-instructions-marker"} {
			require.NotContains(t, string(body), marker, "替换不能保留客户端指令")
		}
	} else {
		require.Contains(t, string(body), "client-system-marker")
		if originalInSystem {
			groupIndex := strings.Index(instructions, groupForwardPrompt)
			clientIndex := strings.Index(instructions, "client-system-marker")
			require.GreaterOrEqual(t, clientIndex, 0)
			if mode == "prepend" {
				require.Less(t, groupIndex, clientIndex)
			} else {
				require.Greater(t, groupIndex, clientIndex)
			}
		}
	}
	require.Contains(t, string(body), "hello-user-marker", "用户消息必须保留")
}

func TestGroupSystemPromptClaudeForwardFinalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{"passthrough", "apikey", "oauth"} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%t", kind, mode, stream), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"model":"claude-sonnet-4-5","max_tokens":32,"stream":%t,"instructions":"client-instructions-marker","system":[{"type":"text","text":"client-system-marker","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hello-user-marker"}]}`, stream))
					original := append([]byte(nil), body...)
					parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					ctx, c, _ := groupPromptForwardContext(PlatformAnthropic, mode, "/v1/messages", body)
					response := nativeAnthropicBufferedResponse()
					if stream {
						response = nativeAnthropicStreamResponse()
					}
					upstream := &httpUpstreamRecorder{resp: response}
					result, err := groupPromptClaudeService(upstream).Forward(ctx, c, groupPromptClaudeAccount(kind), parsed)
					require.NoError(t, err)
					require.NotNil(t, result)
					assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolMessages, mode, kind != "oauth")
					if kind == "oauth" {
						require.NotContains(t, gjson.GetBytes(upstream.lastBody, "messages").Raw, groupForwardPrompt)
						system := gjson.GetBytes(upstream.lastBody, "system").Array()
						require.Contains(t, system[0].Get("text").String(), "x-anthropic-billing-header:")
						require.Equal(t, claudeCodeSystemPrompt, system[1].Get("text").String())
					} else if mode != "replace" {
						index := 0
						if mode == "prepend" {
							index = 1
						}
						require.Equal(t, "ephemeral", gjson.GetBytes(upstream.lastBody, fmt.Sprintf("system.%d.cache_control.type", index)).String())
					}
					require.Equal(t, original, body)
					require.Equal(t, original, parsed.Body.Bytes(), "转发不能污染 canonical ParsedRequest")
				})
			}
		}
	}
}

func TestGroupSystemPromptClaudeBridgesFinalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions} {
		for _, kind := range []string{"apikey", "oauth"} {
			for _, mode := range []string{"prepend", "append", "replace"} {
				for _, stream := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/stream=%t", protocol, kind, mode, stream), func(t *testing.T) {
						var body []byte
						path := "/v1/responses"
						if protocol == SystemPromptProtocolResponses {
							body = []byte(fmt.Sprintf(`{"model":"claude-sonnet-4-5","stream":%t,"instructions":"client-instructions-marker","input":[{"type":"message","role":"system","content":[{"type":"input_text","text":"client-system-marker"}]},{"type":"message","role":"developer","content":[{"type":"input_text","text":"client-developer-marker"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}]}`, stream))
						} else {
							path = "/v1/chat/completions"
							body = []byte(fmt.Sprintf(`{"model":"claude-sonnet-4-5","stream":%t,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, stream))
						}
						original := append([]byte(nil), body...)
						ctx, c, _ := groupPromptForwardContext(PlatformAnthropic, mode, path, body)
						upstream := &httpUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
						svc := groupPromptClaudeService(upstream)
						account := groupPromptClaudeAccount(kind)
						var err error
						if protocol == SystemPromptProtocolResponses {
							_, err = svc.ForwardAsResponses(ctx, c, account, body, nil)
						} else {
							_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, nil)
						}
						require.NoError(t, err)
						assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolMessages, mode, kind != "oauth")
						if kind == "oauth" {
							require.NotContains(t, gjson.GetBytes(upstream.lastBody, "messages").Raw, groupForwardPrompt)
							system := gjson.GetBytes(upstream.lastBody, "system").Array()
							require.Contains(t, system[0].Get("text").String(), "x-anthropic-billing-header:")
							require.Equal(t, claudeCodeSystemPrompt, system[1].Get("text").String())
						}
						require.Equal(t, original, body)
					})
				}
			}
		}
	}
}

func groupPromptOpenAIAccount(kind string) *Account {
	account := structuredInputRetryTestAccount()
	account.Status, account.Schedulable = StatusActive, true
	if kind == "chat" {
		account = forceChatMessagesFallbackAccount()
	} else if kind == "native_anthropic" {
		// 原生 Anthropic 出口属于多协议账号，分组策略仍来自原绑定的 OpenAI 分组。
		account = nativeAnthropicTestAccount()
	} else if kind == "oauth" || kind == "oauth_passthrough" {
		account.Type = AccountTypeOAuth
		account.Credentials = map[string]any{"access_token": "mock-oauth-token", "chatgpt_account_id": "mock-chatgpt-account"}
		account.Extra["openai_passthrough"] = kind == "oauth_passthrough"
		account.Extra["openai_oauth_responses_websockets_v2_mode"] = OpenAIWSIngressModeOff
	}
	return account
}

func TestGroupSystemPromptOpenAIForwardFinalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ingress := range []string{SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions, SystemPromptProtocolMessages} {
		for _, kind := range []string{"responses", "chat", "oauth", "oauth_passthrough", "native_anthropic"} {
			for _, mode := range []string{"prepend", "append", "replace"} {
				for _, stream := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/stream=%t", ingress, kind, mode, stream), func(t *testing.T) {
						path := "/v1/responses"
						body := []byte(fmt.Sprintf(`{"model":"gpt-5.4","stream":%t,"instructions":"client-instructions-marker","input":[{"type":"message","role":"system","content":[{"type":"input_text","text":"client-system-marker"}]},{"type":"message","role":"developer","content":[{"type":"input_text","text":"client-developer-marker"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}]}`, stream))
						if ingress == SystemPromptProtocolChatCompletions {
							path = "/v1/chat/completions"
							body = []byte(fmt.Sprintf(`{"model":"gpt-5.4","stream":%t,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, stream))
						} else if ingress == SystemPromptProtocolMessages {
							path = "/v1/messages"
							body = []byte(fmt.Sprintf(`{"model":"gpt-5.4","max_tokens":32,"stream":%t,"system":[{"type":"text","text":"client-system-marker"}],"messages":[{"role":"user","content":"hello-user-marker"}]}`, stream))
						}
						original := append([]byte(nil), body...)
						ctx, c, _ := groupPromptForwardContext(PlatformOpenAI, mode, path, body)
						protocol := SystemPromptProtocolResponses
						response := groupPromptResponsesResponse()
						if kind == "chat" {
							protocol = SystemPromptProtocolChatCompletions
							response = groupPromptChatResponse(stream)
						} else if kind == "native_anthropic" {
							protocol = SystemPromptProtocolMessages
							response = nativeAnthropicBufferedResponse()
							// 两条跨协议转换出口强制上游流式，直通 Messages 跟随客户端。
							if stream || ingress != SystemPromptProtocolMessages {
								response = nativeAnthropicStreamResponse()
							}
						}
						upstream := &httpUpstreamRecorder{resp: response}
						svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
						account := groupPromptOpenAIAccount(kind)
						var result *OpenAIForwardResult
						var err error
						switch ingress {
						case SystemPromptProtocolResponses:
							result, err = svc.Forward(ctx, c, account, body)
						case SystemPromptProtocolChatCompletions:
							result, err = svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
						case SystemPromptProtocolMessages:
							result, err = svc.ForwardAsAnthropic(ctx, c, account, body, "", "")
						}
						require.NoError(t, err)
						require.NotNil(t, result)
						assertGroupPromptFinalBody(t, upstream.lastBody, protocol, mode, true)
						require.Equal(t, original, body, "转发不能污染 canonical 输入")
					})
				}
			}
		}
	}
}

func TestGroupSystemPromptReplacePreservesToolData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{"responses", "oauth_passthrough"} {
		t.Run(kind, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","instructions":"client-instructions-marker","system":"client-system-marker","input":[{"type":"message","role":"system","content":[{"type":"input_text","text":"client-system-marker"}]},{"type":"message","role":"developer","content":[{"type":"input_text","text":"client-developer-marker"}]},{"type":"function_call","role":"developer","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","role":"developer","call_id":"call_1","output":"tool-result-marker"},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}],"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`)
			ctx, c, _ := groupPromptForwardContext(PlatformOpenAI, "replace", "/v1/responses", body)
			upstream := &httpUpstreamRecorder{resp: groupPromptResponsesResponse()}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			_, err := svc.Forward(ctx, c, groupPromptOpenAIAccount(kind), body)
			require.NoError(t, err)
			assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolResponses, "replace", true)
			require.Equal(t, "function_call", gjson.GetBytes(upstream.lastBody, "input.0.type").String())
			require.Equal(t, "function_call_output", gjson.GetBytes(upstream.lastBody, "input.1.type").String())
			require.Equal(t, "tool-result-marker", gjson.GetBytes(upstream.lastBody, "input.1.output").String())
			require.Equal(t, "lookup", gjson.GetBytes(upstream.lastBody, "tools.0.name").String())
		})
	}
}

func TestGroupSystemPromptReplacePreservesChatToolConversation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{"claude", "chat", "responses", "oauth"} {
		t.Run(kind, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","stream":false,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"},{"role":"assistant","content":"assistant-history-marker","tool_calls":[{"type":"function","id":"call_tool_marker","function":{"name":"lookup","arguments":"{\"query\":\"tool-argument-marker\"}"}}]},{"role":"tool","tool_call_id":"call_tool_marker","content":"tool-result-marker"}],"tools":[{"type":"function","function":{"name":"lookup","description":"tool-definition-marker","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}}]}`)
			platform := PlatformOpenAI
			if kind == "claude" {
				platform = PlatformAnthropic
			}
			ctx, c, _ := groupPromptForwardContext(platform, "replace", "/v1/chat/completions", body)
			upstream := &httpUpstreamRecorder{}
			protocol := SystemPromptProtocolResponses
			if kind == "claude" {
				upstream.resp = nativeAnthropicStreamResponse()
				protocol = SystemPromptProtocolMessages
				_, err := groupPromptClaudeService(upstream).ForwardAsChatCompletions(ctx, c, groupPromptClaudeAccount("apikey"), body, nil)
				require.NoError(t, err)
			} else {
				upstream.resp = groupPromptResponsesResponse()
				if kind == "chat" {
					upstream.resp = groupPromptChatResponse(false)
					protocol = SystemPromptProtocolChatCompletions
				}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				_, err := svc.ForwardAsChatCompletions(ctx, c, groupPromptOpenAIAccount(kind), body, "", "")
				require.NoError(t, err)
			}
			assertGroupPromptFinalBody(t, upstream.lastBody, protocol, "replace", true)
			for _, marker := range []string{"assistant-history-marker", "tool-argument-marker", "tool-result-marker", "tool-definition-marker"} {
				require.Contains(t, string(upstream.lastBody), marker, "替换只删除客户端指令")
			}
			// Codex OAuth 会按既有协议将 call_ 改为 fc_。验证关联完整，
			// 不能把正常的协议规范化误报为注入功能删除工具数据。
			var callID, resultID string
			switch protocol {
			case SystemPromptProtocolResponses:
				for _, item := range gjson.GetBytes(upstream.lastBody, "input").Array() {
					switch item.Get("type").String() {
					case "function_call":
						callID = item.Get("call_id").String()
					case "function_call_output":
						resultID = item.Get("call_id").String()
					}
				}
			case SystemPromptProtocolChatCompletions:
				for _, message := range gjson.GetBytes(upstream.lastBody, "messages").Array() {
					if message.Get("role").String() == "tool" {
						resultID = message.Get("tool_call_id").String()
					}
					for _, call := range message.Get("tool_calls").Array() {
						callID = call.Get("id").String()
					}
				}
			case SystemPromptProtocolMessages:
				for _, message := range gjson.GetBytes(upstream.lastBody, "messages").Array() {
					for _, block := range message.Get("content").Array() {
						switch block.Get("type").String() {
						case "tool_use":
							callID = block.Get("id").String()
						case "tool_result":
							resultID = block.Get("tool_use_id").String()
						}
					}
				}
			}
			require.NotEmpty(t, callID)
			require.Equal(t, callID, resultID, "工具结果仍关联同一条工具调用")
		})
	}
}

func TestGroupSystemPromptFailoverStartsFromCanonicalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("claude OAuth then APIKey", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":32,"system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
		original := append([]byte(nil), body...)
		parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
		require.NoError(t, err)
		ctx, c, _ := groupPromptForwardContext(PlatformAnthropic, "append", "/v1/messages", body)
		upstream := &httpUpstreamRecorder{responses: []*http.Response{
			{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"overloaded_error","message":"retry another account"}}`))},
			nativeAnthropicBufferedResponse(),
		}}
		svc := groupPromptClaudeService(upstream)
		// 不配置错误处置服务：这里只验证 handler 换账号时重用 canonical 输入的契约。
		svc.rateLimitService = nil
		_, err = svc.Forward(ctx, c, groupPromptClaudeAccount("oauth"), parsed)
		require.Error(t, err)
		require.Equal(t, original, parsed.Body.Bytes())
		_, err = svc.Forward(ctx, c, groupPromptClaudeAccount("passthrough"), parsed)
		require.NoError(t, err)
		require.Len(t, upstream.bodies, 2)
		for _, outbound := range upstream.bodies {
			assertGroupPromptFinalBody(t, outbound, SystemPromptProtocolMessages, "append", false)
		}
		require.NotContains(t, string(upstream.bodies[1]), claudeCodeSystemPrompt, "OAuth 身份改写不能泄漏到后续 APIKey 尝试")
		require.Equal(t, original, parsed.Body.Bytes())
	})
	t.Run("OpenAI OAuth then APIKey", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.4","instructions":"client-instructions-marker","input":[{"type":"message","role":"system","content":[{"type":"input_text","text":"client-system-marker"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}]}`)
		original := append([]byte(nil), body...)
		ctx, c, _ := groupPromptForwardContext(PlatformOpenAI, "append", "/v1/responses", body)
		upstream := &httpUpstreamRecorder{responses: []*http.Response{
			{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"server_error","message":"retry another account"}}`))},
			groupPromptResponsesResponse(),
		}}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
		_, err := svc.Forward(ctx, c, groupPromptOpenAIAccount("oauth"), body)
		require.Error(t, err)
		_, err = svc.Forward(ctx, c, groupPromptOpenAIAccount("responses"), body)
		require.NoError(t, err)
		require.Len(t, upstream.bodies, 2)
		for _, outbound := range upstream.bodies {
			assertGroupPromptFinalBody(t, outbound, SystemPromptProtocolResponses, "append", true)
		}
		require.Equal(t, original, body)
	})
}

func TestGroupSystemPromptTokenCountFinalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{"passthrough", "apikey", "oauth"} {
		t.Run("claude/"+kind, func(t *testing.T) {
			body := []byte(`{"model":"claude-sonnet-4-5","system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			ctx, c, rec := groupPromptForwardContext(PlatformAnthropic, "append", "/v1/messages/count_tokens", body)
			upstream := &httpUpstreamRecorder{resp: groupPromptJSONResponse(`{"input_tokens":42}`)}
			err = groupPromptClaudeService(upstream).ForwardCountTokens(ctx, c, groupPromptClaudeAccount(kind), parsed)
			require.NoError(t, err)
			require.Equal(t, int64(42), gjson.Get(rec.Body.String(), "input_tokens").Int())
			assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolMessages, "append", true)
			require.Equal(t, body, parsed.Body.Bytes())
		})
	}
	for _, ingress := range []string{SystemPromptProtocolResponses, SystemPromptProtocolMessages} {
		t.Run("openai/"+ingress, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","instructions":"client-system-marker","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}]}`)
			path := "/v1/responses/input_tokens"
			if ingress == SystemPromptProtocolMessages {
				path = "/v1/messages/count_tokens"
				body = []byte(`{"model":"gpt-5.4","system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
			}
			ctx, c, rec := groupPromptForwardContext(PlatformOpenAI, "append", path, body)
			upstream := &httpUpstreamRecorder{resp: groupPromptJSONResponse(`{"object":"response.input_tokens","input_tokens":42}`)}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			account := groupPromptOpenAIAccount("responses")
			// 自定义 relay 的 Responses 计数按既有约定使用本地估算。
			account.Credentials["base_url"] = "https://api.openai.com"
			var err error
			if ingress == SystemPromptProtocolResponses {
				err = svc.ForwardResponsesInputTokens(ctx, c, account, body)
			} else {
				err = svc.ForwardCountTokensAsAnthropic(ctx, c, account, body, "gpt-5.4")
			}
			require.NoError(t, err)
			require.Equal(t, int64(42), gjson.Get(rec.Body.String(), "input_tokens").Int())
			assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolResponses, "append", true)
		})
	}
}

func TestGroupSystemPromptLocalTokenCountIncludesContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ingress := range []string{SystemPromptProtocolResponses, SystemPromptProtocolMessages} {
		t.Run(ingress, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","instructions":"client-system-marker","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello-user-marker"}]}]}`)
			path := "/v1/responses/input_tokens"
			account := groupPromptOpenAIAccount("responses")
			if ingress == SystemPromptProtocolMessages {
				body = []byte(`{"model":"k3","system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
				path = "/v1/messages/count_tokens"
				account = nativeAnthropicTestAccount()
			}
			original := append([]byte(nil), body...)
			var counts []int64
			for _, enabled := range []bool{false, true} {
				ctx, c, rec := groupPromptForwardContext(PlatformOpenAI, "append", path, body)
				if !enabled {
					ctx = WithGroupSystemPromptInjection(context.Background(), nil)
					c.Request = c.Request.WithContext(ctx)
				}
				upstream := &httpUpstreamRecorder{}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				var err error
				if ingress == SystemPromptProtocolResponses {
					err = svc.ForwardResponsesInputTokens(ctx, c, account, body)
				} else {
					err = svc.ForwardCountTokensAsAnthropic(ctx, c, account, body, "")
				}
				require.NoError(t, err)
				require.Empty(t, upstream.requests, "本地计数不能发出上游请求")
				counts = append(counts, gjson.Get(rec.Body.String(), "input_tokens").Int())
			}
			require.Positive(t, counts[0])
			require.Greater(t, counts[1], counts[0], "本地估算必须计入分组正文")
			require.Equal(t, original, body)
		})
	}
}
