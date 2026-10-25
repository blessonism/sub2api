//go:build unit

package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func groupPromptGeminiResponse(stream, wrapped bool) *http.Response {
	body := `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`
	if wrapped {
		body = `{"response":` + body + `}`
	}
	if !stream {
		return groupPromptJSONResponse(body)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: " + body + "\n\n"))}
}

func assertGroupPromptGeminiForward(t *testing.T, wire []byte, mode string, identity bool) {
	t.Helper()
	root := gjson.ParseBytes(wire)
	if root.Get("request").IsObject() {
		root = root.Get("request")
	}
	instructions := root.Get("systemInstruction.parts").Raw
	require.Equal(t, 1, strings.Count(string(wire), groupForwardPrompt))
	require.Contains(t, instructions, groupForwardPrompt)
	require.Contains(t, string(wire), "hello-user-marker")
	if mode == "replace" {
		require.NotContains(t, string(wire), "client-system-marker")
		require.NotContains(t, string(wire), "client-developer-marker")
	} else {
		client := strings.Index(instructions, "client-system-marker")
		group := strings.Index(instructions, groupForwardPrompt)
		require.GreaterOrEqual(t, client, 0)
		if mode == "prepend" {
			require.Less(t, group, client)
		} else {
			require.Greater(t, group, client)
		}
	}
	if identity {
		require.Contains(t, root.Get("systemInstruction.parts.0.text").String(), "You are Antigravity")
	}
}

func TestGroupSystemPromptAllOpenAIProvidersForward(t *testing.T) {
	for _, platform := range domain.PlatformIDsWhere(func(spec domain.PlatformSpec) bool { return spec.Gateway == domain.PlatformGatewayOpenAI }) {
		for _, mode := range []string{"prepend", "append", "replace"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%t", platform, mode, stream), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"model":"model-test","stream":%t,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, stream))
					ctx, c, _ := groupPromptForwardContext(platform, mode, "/v1/chat/completions", body)
					account := groupPromptOpenAIAccount("chat")
					account.Platform = platform
					upstream := &httpUpstreamRecorder{resp: groupPromptChatResponse(stream)}
					svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
					_, err := svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
					require.NoError(t, err)
					assertGroupPromptFinalBody(t, upstream.lastBody, SystemPromptProtocolChatCompletions, mode, true)
				})
			}
		}
	}
}

func TestGroupSystemPromptGeminiForwardAllIngress(t *testing.T) {
	for _, protocol := range []string{SystemPromptProtocolGemini, SystemPromptProtocolMessages, SystemPromptProtocolChatCompletions} {
		for _, kind := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
			for _, mode := range []string{"prepend", "append", "replace"} {
				for _, stream := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/stream=%t", protocol, kind, mode, stream), func(t *testing.T) {
						model := "gemini-2.5-flash"
						body := []byte(fmt.Sprintf(`{"model":"%s","stream":%t,"system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`, model, stream))
						switch protocol {
						case SystemPromptProtocolGemini:
							body = []byte(`{"systemInstruction":{"parts":[{"text":"client-system-marker"}]},"contents":[{"role":"user","parts":[{"text":"hello-user-marker"}]}]}`)
						case SystemPromptProtocolChatCompletions:
							body = []byte(fmt.Sprintf(`{"model":"%s","stream":%t,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, model, stream))
						}
						original := string(body)
						// 组合分组的原绑定策略应与具体 Gemini 分组一样生效。
						ctx, c, _ := groupPromptForwardContext(PlatformComposite, mode, "/v1/chat/completions", body)
						upstream := &httpUpstreamRecorder{resp: groupPromptGeminiResponse(stream || kind == AccountTypeOAuth, kind == AccountTypeOAuth)}
						svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream, tokenProvider: &GeminiTokenProvider{}}
						account := &Account{ID: 101, Platform: PlatformGemini, Type: kind, Concurrency: 1, Credentials: map[string]any{
							"api_key": "test-key", "access_token": "test-token", "project_id": "project-test",
						}}
						var err error
						switch protocol {
						case SystemPromptProtocolGemini:
							_, err = svc.ForwardNative(ctx, c, account, model, "generateContent", stream, body)
						case SystemPromptProtocolMessages:
							_, err = svc.Forward(ctx, c, account, body)
						default:
							_, err = svc.ForwardAsChatCompletions(ctx, c, account, body)
						}
						require.NoError(t, err)
						assertGroupPromptGeminiForward(t, upstream.lastBody, mode, false)
						require.Equal(t, original, string(body))
					})
				}
			}
		}
	}
}

func TestGroupSystemPromptAntigravityForwardAllIngress(t *testing.T) {
	for _, protocol := range []string{SystemPromptProtocolGemini, SystemPromptProtocolMessages, SystemPromptProtocolChatCompletions, SystemPromptProtocolResponses} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%t", protocol, mode, stream), func(t *testing.T) {
					model := "claude-sonnet-4-5"
					body := []byte(fmt.Sprintf(`{"model":"%s","stream":%t,"max_tokens":32,"system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`, model, stream))
					switch protocol {
					case SystemPromptProtocolGemini:
						model = "gemini-3.1-pro-high"
						body = []byte(`{"systemInstruction":{"parts":[{"text":"client-system-marker"}]},"contents":[{"role":"user","parts":[{"text":"hello-user-marker"}]}]}`)
					case SystemPromptProtocolChatCompletions:
						body = []byte(fmt.Sprintf(`{"model":"%s","stream":%t,"messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, model, stream))
					case SystemPromptProtocolResponses:
						body = []byte(fmt.Sprintf(`{"model":"%s","stream":%t,"instructions":"client-system-marker","input":[{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`, model, stream))
					}
					ctx, c, _ := groupPromptForwardContext(PlatformAntigravity, mode, "/v1/messages", body)
					upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{groupPromptGeminiResponse(true, true)}}
					svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
					account := newAntigravityCompatAccount(AccountTypeOAuth)
					var err error
					switch protocol {
					case SystemPromptProtocolGemini:
						_, err = svc.ForwardGemini(ctx, c, account, model, "generateContent", stream, body, false)
					case SystemPromptProtocolMessages:
						_, err = svc.Forward(ctx, c, account, body, false)
					case SystemPromptProtocolChatCompletions:
						_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, nil)
					default:
						_, err = svc.ForwardAsResponses(ctx, c, account, body, nil)
					}
					require.NoError(t, err)
					require.Len(t, upstream.requestBodies, 1)
					assertGroupPromptGeminiForward(t, upstream.requestBodies[0], mode, true)
				})
			}
		}
	}
}

func TestGroupSystemPromptOtherPlatformsCountTokens(t *testing.T) {
	body := []byte(`{"model":"gemini-2.5-flash","system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
	for _, platform := range []string{PlatformGemini, PlatformAntigravity, PlatformGrok} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(platform+"/"+mode, func(t *testing.T) {
				ctx, c, _ := groupPromptForwardContext(platform, mode, "/v1/messages/count_tokens", body)
				cfg := groupSystemPromptConfig(ctx, c)
				prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolMessages, cfg)
				require.NoError(t, err)
				if platform == PlatformGrok {
					expected, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolMessages, cfg)
					require.NoError(t, err)
					want, err := EstimateGrokCountTokens(expected)
					require.NoError(t, err)
					got, err := EstimateGrokCountTokensForGroup(ctx, c, body)
					require.NoError(t, err)
					require.Equal(t, want, got)
					return
				}
				svc := &GatewayService{}
				// OAuth 账号必须同样命中本地估算：凭据协议拦截不允许先于分组提示路径生效。
				for _, account := range []*Account{
					{Platform: platform, Type: AccountTypeAPIKey},
					{Platform: platform, Type: AccountTypeOAuth},
				} {
					ctx, c, rec := groupPromptForwardContext(platform, mode, "/v1/messages/count_tokens", body)
					parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					require.NoError(t, svc.ForwardCountTokens(ctx, c, account, parsed))
					require.Equal(t, 200, rec.Code)
					require.Greater(t, gjson.GetBytes(rec.Body.Bytes(), "input_tokens").Int(), int64(0))
					require.Equal(t, string(body), string(parsed.Body.Bytes()))
				}
			})
		}
	}
}

func TestGroupSystemPromptDeepSeekNativeProtocols(t *testing.T) {
	for _, protocol := range []string{SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions, SystemPromptProtocolMessages} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				body := []byte(`{"model":"deepseek-chat","instructions":"client-system-marker","input":[{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`)
				upstreamProtocol := APIProtocolResponses
				response := groupPromptResponsesResponse()
				switch protocol {
				case SystemPromptProtocolChatCompletions:
					body = []byte(`{"model":"deepseek-chat","messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`)
					upstreamProtocol = APIProtocolChatCompletions
					response = groupPromptChatResponse(false)
				case SystemPromptProtocolMessages:
					body = []byte(`{"model":"deepseek-chat","max_tokens":32,"system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
					upstreamProtocol = APIProtocolAnthropic
					response = nativeAnthropicBufferedResponse()
				}
				ctx, c, _ := groupPromptForwardContext(PlatformDeepseek, mode, "/v1/"+protocol, body)
				account := groupPromptOpenAIAccount("responses")
				account.Platform = PlatformDeepseek
				account.Credentials["api_protocol"] = upstreamProtocol
				account.Credentials["api_base_urls"] = map[string]any{upstreamProtocol: "http://upstream.example"}
				upstream := &httpUpstreamRecorder{resp: response}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				var err error
				switch protocol {
				case SystemPromptProtocolResponses:
					_, err = svc.Forward(ctx, c, account, body)
				case SystemPromptProtocolChatCompletions:
					_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
				default:
					_, err = svc.ForwardAsAnthropic(ctx, c, account, body, "", "")
				}
				require.NoError(t, err)
				assertGroupPromptFinalBody(t, upstream.lastBody, protocol, mode, true)
			})
		}
	}
}

func TestGroupSystemPromptGrokOAuthNativeAndBridge(t *testing.T) {
	for _, protocol := range []string{SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions, SystemPromptProtocolMessages, "chat_raw"} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				body := []byte(`{"model":"grok","prompt_cache_key":"group-session","instructions":"client-system-marker","input":[{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`)
				switch protocol {
				case SystemPromptProtocolChatCompletions, "chat_raw":
					body = []byte(`{"model":"grok","prompt_cache_key":"group-session","messages":[{"role":"system","content":"client-system-marker"},{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`)
				case SystemPromptProtocolMessages:
					body = []byte(`{"model":"grok","max_tokens":32,"system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
				}
				if protocol == SystemPromptProtocolChatCompletions {
					body = []byte(`{"model":"grok","prompt_cache_key":"group-session","messages":[{"role":"system","content":"client-system-marker"},{"role":"user","content":"hello-user-marker"}]}`)
				}
				ctx, c, _ := groupPromptForwardContext(PlatformGrok, mode, "/v1/"+protocol, body)
				account := grokChatBridgeTestAccount(79)
				repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{account.ID: account}}}
				response := grokChatBridgeCompletedResponse("resp-group", 0)
				expectedProtocol := SystemPromptProtocolResponses
				if protocol == "chat_raw" && mode != "replace" {
					response = groupPromptChatResponse(false)
					expectedProtocol = SystemPromptProtocolChatCompletions
				}
				upstream := &httpUpstreamRecorder{resp: response}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}
				var err error
				switch protocol {
				case SystemPromptProtocolResponses:
					_, err = svc.Forward(ctx, c, account, body)
				case SystemPromptProtocolChatCompletions, "chat_raw":
					_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
				default:
					_, err = svc.ForwardAsAnthropic(ctx, c, account, body, "", "")
				}
				require.NoError(t, err)
				assertGroupPromptFinalBody(t, upstream.lastBody, expectedProtocol, mode, true)
			})
		}
	}
}

func TestGroupSystemPromptAntigravitySignatureRetry(t *testing.T) {
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			body := []byte(`{"systemInstruction":{"parts":[{"text":"client-system-marker"}]},"contents":[{"role":"user","parts":[{"text":"hello-user-marker"}]},{"role":"model","parts":[{"functionCall":{"name":"tool","args":{}},"thoughtSignature":"bad"}]}]}`)
			ctx, c, _ := groupPromptForwardContext(PlatformAntigravity, mode, "/v1beta/models/gemini-3.1-pro-high:generateContent", body)
			first := groupPromptJSONResponse(`{"response":{"error":{"code":400,"message":"Corrupted thought signature.","status":"INVALID_ARGUMENT"}}}`)
			first.StatusCode = 400
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{first, groupPromptGeminiResponse(true, true)}}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			_, err := svc.ForwardGemini(ctx, c, newAntigravityCompatAccount(AccountTypeOAuth), "gemini-3.1-pro-high", "generateContent", false, body, false)
			require.NoError(t, err)
			require.Len(t, upstream.requestBodies, 2)
			for _, wire := range upstream.requestBodies {
				assertGroupPromptGeminiForward(t, wire, mode, true)
			}
		})
	}
}

func TestGroupSystemPromptAntigravityModelFallback(t *testing.T) {
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			body := []byte(`{"systemInstruction":{"parts":[{"text":"client-system-marker"}]},"contents":[{"role":"user","parts":[{"text":"hello-user-marker"}]}]}`)
			ctx, c, _ := groupPromptForwardContext(PlatformAntigravity, mode, "/v1beta/models/gemini-3.1-pro-high:generateContent", body)
			first := groupPromptJSONResponse(`{"error":{"code":404,"message":"model not found"}}`)
			first.StatusCode = 404
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{first, groupPromptGeminiResponse(true, true)}}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			svc.settingService = NewSettingService(&antigravitySettingRepoStub{values: map[string]string{
				SettingKeyEnableModelFallback: "true", SettingKeyFallbackModelAntigravity: "gemini-fallback",
			}}, &config.Config{})
			_, err := svc.ForwardGemini(ctx, c, newAntigravityCompatAccount(AccountTypeOAuth), "gemini-3.1-pro-high", "generateContent", false, body, false)
			require.NoError(t, err)
			require.Len(t, upstream.requestBodies, 2)
			for _, wire := range upstream.requestBodies {
				assertGroupPromptGeminiForward(t, wire, mode, true)
			}
			require.Equal(t, "gemini-fallback", gjson.GetBytes(upstream.requestBodies[1], "model").String())
		})
	}
}

func TestGroupSystemPromptGeminiSignatureRetry(t *testing.T) {
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			body := []byte(`{"model":"gemini-2.5-flash","system":"client-system-marker","messages":[{"role":"user","content":"hello-user-marker"}]}`)
			ctx, c, _ := groupPromptForwardContext(PlatformGemini, mode, "/v1/messages", body)
			first := groupPromptJSONResponse(`{"error":{"code":400,"message":"Corrupted thought signature.","status":"INVALID_ARGUMENT"}}`)
			first.StatusCode = 400
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{first, groupPromptGeminiResponse(false, false)}}
			svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
			_, err := svc.Forward(ctx, c, account, body)
			require.NoError(t, err)
			require.Len(t, upstream.requestBodies, 2)
			for _, wire := range upstream.requestBodies {
				assertGroupPromptGeminiForward(t, wire, mode, false)
			}
		})
	}
}

func TestGroupSystemPromptGrokWSHTTPBridgeTurns(t *testing.T) {
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			canonical := []byte(`{"type":"response.create","model":"grok","instructions":"client-system-marker","input":[{"role":"developer","content":"client-developer-marker"},{"role":"user","content":"hello-user-marker"}]}`)
			ctx, c, _ := groupPromptForwardContext(PlatformComposite, mode, "/v1/responses", canonical)
			account := grokChatBridgeTestAccount(79)
			repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{account.ID: account}}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{grokChatBridgeCompletedResponse("resp-1", 0), grokChatBridgeCompletedResponse("resp-2", 0)}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, accountRepo: repo}
			for turn := 1; turn <= 2; turn++ {
				payload, err := PrepareGroupSystemPromptBody(canonical, SystemPromptProtocolResponses, groupSystemPromptConfig(ctx, c))
				require.NoError(t, err)
				_, err = svc.proxyOpenAIWSHTTPBridgeTurn(ctx, c, account, "test-token", payload, len(payload), "grok", "", "", "", "group-session", turn, func([]byte) error { return nil })
				require.NoError(t, err)
			}
			require.Len(t, upstream.bodies, 2)
			for _, wire := range upstream.bodies {
				assertGroupPromptFinalBody(t, wire, SystemPromptProtocolResponses, mode, true)
			}
			require.NotContains(t, string(canonical), groupForwardPrompt)
		})
	}
}

func TestGroupSystemPromptGeminiNativeCountTokens(t *testing.T) {
	for _, platform := range []string{PlatformGemini, PlatformAntigravity} {
		for _, wrapper := range []string{"", "generateContentRequest", "generate_content_request"} {
			for _, mode := range []string{"prepend", "append", "replace"} {
				t.Run(platform+"/"+wrapper+"/"+mode, func(t *testing.T) {
					body := []byte(`{"systemInstruction":{"parts":[{"text":"client-system-marker"}]},"contents":[{"role":"user","parts":[{"text":"hello-user-marker"}]}]}`)
					if wrapper != "" {
						body = []byte(`{"` + wrapper + `":` + string(body) + `}`)
					}
					ctx, c, rec := groupPromptForwardContext(platform, mode, "/v1beta/models/gemini-2.5-flash:countTokens", body)
					cfg := groupSystemPromptConfig(ctx, c)
					prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolGemini, cfg)
					require.NoError(t, err)
					prepared = unwrapGroupSystemPromptGeminiBody(prepared)
					if platform == PlatformAntigravity {
						prepared, err = injectIdentityPatchToGeminiRequest(prepared)
						require.NoError(t, err)
					}
					wire, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolGemini, cfg)
					require.NoError(t, err)
					upstream := &httpUpstreamRecorder{err: io.ErrUnexpectedEOF}
					account := &Account{ID: 101, Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
					if platform == PlatformAntigravity {
						svc := newAntigravityCompatService(config.GatewayConfig{}, upstream)
						_, err = svc.ForwardGemini(ctx, c, account, "gemini-2.5-flash", "countTokens", false, body, false)
					} else {
						svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
						_, err = svc.ForwardNative(ctx, c, account, "gemini-2.5-flash", "countTokens", false, body)
					}
					require.NoError(t, err)
					require.Equal(t, int64(estimateGeminiCountTokens(wire)), gjson.GetBytes(rec.Body.Bytes(), "totalTokens").Int())
				})
			}
		}
	}
}
