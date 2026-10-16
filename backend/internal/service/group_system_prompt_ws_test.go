package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 复用真实 WS 测试的分阶段上游，额外记录 pool 使用的 WriteJSON。
type groupSystemPromptWSConn struct{ *stagedPassthroughConn }

func (c *groupSystemPromptWSConn) WriteJSON(ctx context.Context, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.WriteFrame(ctx, coderws.MessageText, body)
}

func TestGroupSystemPromptWSMultiTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, transport := range []string{OpenAIWSIngressModeCtxPool, OpenAIWSIngressModePassthrough} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(transport+"/"+mode, func(t *testing.T) {
				group := &Group{Platform: PlatformOpenAI, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "tenant prompt"}}
				ctx, cancel := context.WithCancel(WithGroupSystemPromptInjection(context.Background(), group))
				defer cancel()
				upstream := newStagedPassthroughConn()
				cfg := passthroughLifecycleConfig()
				cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
				cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 3
				cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
				cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
				svc := newPassthroughLifecycleService(cfg, upstream)
				account := passthroughLifecycleAccount()
				account.Extra["openai_apikey_responses_websockets_v2_mode"] = transport
				if transport == OpenAIWSIngressModeCtxPool {
					pool := newOpenAIWSConnPool(cfg)
					pool.setClientDialerForTest(&stagedPassthroughDialer{conn: &groupSystemPromptWSConn{upstream}})
					t.Cleanup(pool.Close)
					svc.openaiWSPool = pool
				}
				server, serverErr := startPassthroughLifecycleServer(t, ctx, svc, account)
				defer server.Close()
				client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-5.1","instructions":"client prompt","input":[{"type":"message","role":"developer","content":"client developer"},{"role":"user","content":"hello"}]}`)
				defer func() { _ = client.CloseNow() }()
				for turn := 1; turn <= 2; turn++ {
					outbound := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
					require.Equal(t, 1, strings.Count(string(outbound), "tenant prompt"), "每次 response.create 仅注入一次")
					if mode == "replace" {
						require.NotContains(t, string(outbound), "client prompt")
						require.NotContains(t, string(outbound), "client developer")
					} else {
						require.Contains(t, string(outbound), "client prompt")
						require.Contains(t, string(outbound), "client developer")
					}
					if mode == "prepend" {
						require.Equal(t, "tenant prompt\n\nclient prompt", gjson.GetBytes(outbound, "instructions").String())
					}
					upstream.Send(`{"type":"response.completed","response":{"id":"resp_prompt","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`)
					event, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
					require.NoError(t, err)
					require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
					if turn == 1 {
						if transport == OpenAIWSIngressModePassthrough {
							control := `{"type":"response.cancel","instructions":"control instruction"}`
							writeCtx, cancelWrite := context.WithTimeout(ctx, time.Second)
							require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(control)))
							cancelWrite()
							require.Equal(t, control, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
							// 上游会继承 session.instructions；replace 必须清理该来源，
							// 控制帧本身仍不能注入分组正文。
							sessionUpdate := `{"type":"session.update","session":{"model":"gpt-5.1","instructions":"session client rules","system":"session hidden rules","input":{"type":"message","role":"developer","content":"session client directive"},"tools":[{"type":"function","name":"keep_tool"}]}}`
							writeCtx, cancelWrite = context.WithTimeout(ctx, time.Second)
							require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(sessionUpdate)))
							cancelWrite()
							updated := requirePassthroughUpstreamWrite(t, upstream, time.Second)
							require.NotContains(t, string(updated), "tenant prompt")
							if mode == "replace" {
								require.NotContains(t, string(updated), "session client")
								require.NotContains(t, string(updated), "session hidden")
								require.Equal(t, "gpt-5.1", gjson.GetBytes(updated, "session.model").String())
								require.Equal(t, "keep_tool", gjson.GetBytes(updated, "session.tools.0.name").String())
							} else {
								require.JSONEq(t, sessionUpdate, string(updated))
							}
						}
						writeCtx, cancelWrite := context.WithTimeout(ctx, time.Second)
						err = client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","instructions":"client prompt","input":[{"role":"developer","content":"client developer"},{"role":"user","content":"next"}]}`))
						cancelWrite()
						require.NoError(t, err)
					}
				}
				_ = client.Close(coderws.StatusNormalClosure, "done")
				select {
				case err := <-serverErr:
					if err != nil {
						require.Contains(t, err.Error(), "StatusNormalClosure")
					}
				case <-time.After(4 * time.Second):
					t.Fatal("等待 websocket 会话结束超时")
				}
			})
		}
	}
}

func TestGroupSystemPromptHTTPToWSAttemptsKeepOriginalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := passthroughLifecycleConfig()
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	upstream := &openAIWSCaptureConn{events: [][]byte{
		[]byte(`{"type":"response.completed","response":{"id":"resp_prompt_1","model":"gpt-5.1","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_prompt_2","model":"gpt-5.1","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`),
	}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&stagedPassthroughDialer{conn: upstream})
	svc := &OpenAIGatewayService{cfg: cfg, openaiWSPool: pool, cache: &stubGatewayCache{}}
	t.Cleanup(pool.Close)
	account := passthroughLifecycleAccount()
	group := &Group{Platform: PlatformOpenAI, SystemPromptInjection: GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "tenant prompt"}}
	ctx := WithGroupSystemPromptInjection(context.Background(), group)
	body := map[string]any{"model": "gpt-5.1", "instructions": "server global", "input": "hello"}
	for attempt := 0; attempt < 2; attempt++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		_, err := svc.forwardOpenAIWSV2(ctx, c, account, body, "", "", "sk-test", OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}, false, false, "gpt-5.1", "gpt-5.1", time.Now(), attempt, "", nil)
		require.NoError(t, err)
		require.Equal(t, "server global", body["instructions"], "下一次尝试的基础请求不能带已注入的正文")
	}
	require.Len(t, upstream.writes, 2)
	for _, write := range upstream.writes {
		require.Equal(t, "server global\n\ntenant prompt", write["instructions"])
	}
}

func TestGroupSystemPromptWSSessionUpdate(t *testing.T) {
	body := []byte(`{"type":"session.update","instructions":"frame rules","session":{"instructions":"client default","system":"hidden","messages":[{"role":"system","content":"client message"},{"role":"user","content":"user"}],"input":[{"type":"message","role":"developer","content":"client input"},{"type":"additional_tools","role":"developer","tools":[{"name":"keep"}]}],"tools":[{"name":"tool"}],"model":"gpt-5.1"},"event_id":"e1"}`)
	for _, mode := range []string{"prepend", "append", "replace"} {
		cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "tenant"}
		got, err := prepareGroupSystemPromptWSSessionUpdate(body, cfg)
		require.NoError(t, err)
		require.NotContains(t, string(got), "tenant")
		if mode != "replace" {
			require.Equal(t, string(body), string(got))
			continue
		}
		require.False(t, gjson.GetBytes(got, "instructions").Exists())
		require.False(t, gjson.GetBytes(got, "session.instructions").Exists())
		require.False(t, gjson.GetBytes(got, "session.system").Exists())
		require.Equal(t, "user", gjson.GetBytes(got, "session.messages.0.role").String())
		require.Len(t, gjson.GetBytes(got, "session.messages").Array(), 1)
		require.Equal(t, "additional_tools", gjson.GetBytes(got, "session.input.0.type").String())
		require.Len(t, gjson.GetBytes(got, "session.input").Array(), 1)
		require.JSONEq(t, gjson.GetBytes(body, "session.tools").Raw, gjson.GetBytes(got, "session.tools").Raw)
		require.Equal(t, "e1", gjson.GetBytes(got, "event_id").String())
	}
	got, err := prepareGroupSystemPromptWSSessionUpdate(body, GroupSystemPromptInjection{})
	require.NoError(t, err)
	require.Equal(t, body, got)
}
