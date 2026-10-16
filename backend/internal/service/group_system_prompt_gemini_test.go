package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGroupSystemPromptGeminiPolicy(t *testing.T) {
	for _, field := range []string{"systemInstruction", "system_instruction"} {
		for _, wrapper := range []string{"", "request", "generateContentRequest"} {
			for _, mode := range []string{"prepend", "append", "replace"} {
				t.Run(fmt.Sprintf("%s/%s/%s", field, wrapper, mode), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"%s":{"role":"user","cache_control":{"type":"ephemeral"},"parts":[{"text":"client-one","thoughtSignature":"sig"},{"text":"client-two"}]},"contents":[{"role":"user","parts":[{"functionResponse":{"name":"tool","response":{"role":"developer","data":"keep"}}}]}],"tools":[{"functionDeclarations":[{"name":"tool"}]}],"cachedContent":"cachedContents/existing"}`, field))
					if wrapper != "" {
						body = []byte(fmt.Sprintf(`{"%s":%s,"model":"gemini-test"}`, wrapper, body))
					}
					original := string(body)
					cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "group-policy"}
					prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolGemini, cfg)
					require.NoError(t, err)
					wire, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolGemini, cfg)
					require.NoError(t, err)
					root := gjson.ParseBytes(wire)
					if wrapper != "" {
						root = root.Get(wrapper)
					}
					require.Equal(t, 1, strings.Count(string(wire), "group-policy"))
					require.Equal(t, "keep", root.Get("contents.0.parts.0.functionResponse.response.data").String())
					require.Equal(t, "developer", root.Get("contents.0.parts.0.functionResponse.response.role").String())
					require.Equal(t, "tool", root.Get("tools.0.functionDeclarations.0.name").String())
					require.Equal(t, "cachedContents/existing", root.Get("cachedContent").String())
					if mode == "replace" {
						require.NotContains(t, string(wire), "client-one")
						require.NotContains(t, string(wire), "client-two")
					} else {
						require.Equal(t, "ephemeral", root.Get("systemInstruction.cache_control.type").String())
						parts := root.Get("systemInstruction.parts").Array()
						index := 0
						if mode == "prepend" {
							require.Equal(t, "group-policy", parts[0].Get("text").String())
							index = 1
						} else {
							require.Equal(t, "group-policy", parts[len(parts)-1].Get("text").String())
						}
						require.Equal(t, "sig", parts[index].Get("thoughtSignature").String())
					}
					require.Equal(t, original, string(body), "基础输入不可变")
				})
			}
		}
	}
}

func TestGroupSystemPromptGeminiKeepsRequiredIdentity(t *testing.T) {
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "group-policy"}
			body := []byte(`{"systemInstruction":{"parts":[{"text":"client"}]},"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
			prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolGemini, cfg)
			require.NoError(t, err)
			withIdentity, err := injectIdentityPatchToGeminiRequest(prepared)
			require.NoError(t, err)
			wire, err := InjectGroupSystemPromptBody(withIdentity, SystemPromptProtocolGemini, cfg)
			require.NoError(t, err)
			parts := gjson.GetBytes(wire, "systemInstruction.parts").Array()
			require.Equal(t, strings.TrimSpace(antigravity.GetDefaultIdentityPatch()), strings.TrimSpace(parts[0].Get("text").String()))
			require.Equal(t, 1, strings.Count(string(wire), "group-policy"))
			if mode == "prepend" {
				require.Equal(t, "group-policy", parts[1].Get("text").String())
				require.Equal(t, "client", parts[2].Get("text").String())
			}
			if mode == "replace" {
				require.NotContains(t, string(wire), `"text":"client"`)
			}
		})
	}
}

func TestGroupSystemPromptGeminiValidationAndDisabled(t *testing.T) {
	for _, body := range []string{`{"systemInstruction":"bad"}`, `{"systemInstruction":{"parts":{}}}`, `{"request":null}`, `{"generateContentRequest":[]}`} {
		cfg := GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "group"}
		_, err := PrepareGroupSystemPromptBody([]byte(body), SystemPromptProtocolGemini, cfg)
		require.Error(t, err, body)
		cfg.Enabled = false
		got, err := PrepareGroupSystemPromptBody([]byte(body), SystemPromptProtocolGemini, cfg)
		require.NoError(t, err)
		require.Equal(t, body, string(got))
	}
}

func TestGroupSystemPromptGeminiPreservesOtherRequestFields(t *testing.T) {
	cfg := GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "new"}
	body := []byte(`{"generateContentRequest":{"systemInstruction":{"parts":[{"text":"old"}]},"contents":[{"role":"model","parts":[{"functionCall":{"name":"developer","args":{"system":"tool data"}}}]}],"generationConfig":{"temperature":0.5}}}`)
	prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolGemini, cfg)
	require.NoError(t, err)
	wire, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolGemini, cfg)
	require.NoError(t, err)
	require.Equal(t, "tool data", gjson.GetBytes(wire, "generateContentRequest.contents.0.parts.0.functionCall.args.system").String())
	require.Equal(t, 0.5, gjson.GetBytes(wire, "generateContentRequest.generationConfig.temperature").Float())
	require.Greater(t, estimateGeminiCountTokens(unwrapGroupSystemPromptGeminiBody(wire)), 0)
}
