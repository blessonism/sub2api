package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGroupSystemPromptDisabledPreservesBytes(t *testing.T) {
	for _, body := range []string{" { \"instructions\" : \"old\" , \"input\" : [] }\n", "invalid json"} {
		for _, protocol := range []string{SystemPromptProtocolResponses, SystemPromptProtocolChatCompletions, SystemPromptProtocolMessages, "unknown"} {
			for _, apply := range []func([]byte, string, GroupSystemPromptInjection) ([]byte, error){PrepareGroupSystemPromptBody, InjectGroupSystemPromptBody} {
				got, err := apply([]byte(body), protocol, GroupSystemPromptInjection{Mode: "replace", Content: "new"})
				require.NoError(t, err)
				require.Equal(t, body, string(got))
			}
		}
	}
}

func TestGroupSystemPromptModes(t *testing.T) {
	tests := []struct {
		name, protocol, body, prepend, append, replace string
	}{
		{
			name: "responses with instructions", protocol: SystemPromptProtocolResponses,
			body:    `{"instructions":"old","input":"hello"}`,
			prepend: `{"instructions":"new\n\nold","input":"hello"}`,
			append:  `{"instructions":"old\n\nnew","input":"hello"}`,
			replace: `{"instructions":"new","input":"hello"}`,
		},
		{
			name: "responses without instructions", protocol: SystemPromptProtocolResponses,
			body:    `{"input":[{"role":"user","content":"hello"}]}`,
			prepend: `{"instructions":"new","input":[{"role":"user","content":"hello"}]}`,
			append:  `{"instructions":"new","input":[{"role":"user","content":"hello"}]}`,
			replace: `{"instructions":"new","input":[{"role":"user","content":"hello"}]}`,
		},
		{
			name: "chat with multiple directives", protocol: SystemPromptProtocolChatCompletions,
			body:    `{"messages":[{"role":"user","content":"first"},{"role":"system","content":"old"},{"role":"assistant","content":"answer"},{"role":"developer","content":[{"type":"text","text":"last"}]},{"role":"user","content":"hello"}]}`,
			prepend: `{"messages":[{"role":"user","content":"first"},{"role":"system","content":"new"},{"role":"system","content":"old"},{"role":"assistant","content":"answer"},{"role":"developer","content":[{"type":"text","text":"last"}]},{"role":"user","content":"hello"}]}`,
			append:  `{"messages":[{"role":"user","content":"first"},{"role":"system","content":"old"},{"role":"assistant","content":"answer"},{"role":"developer","content":[{"type":"text","text":"last"}]},{"role":"system","content":"new"},{"role":"user","content":"hello"}]}`,
			replace: `{"messages":[{"role":"system","content":"new"},{"role":"user","content":"first"},{"role":"assistant","content":"answer"},{"role":"user","content":"hello"}]}`,
		},
		{
			name: "chat without directives", protocol: SystemPromptProtocolChatCompletions,
			body:    `{"messages":[{"role":"user","content":"hello"}]}`,
			prepend: `{"messages":[{"role":"system","content":"new"},{"role":"user","content":"hello"}]}`,
			append:  `{"messages":[{"role":"system","content":"new"},{"role":"user","content":"hello"}]}`,
			replace: `{"messages":[{"role":"system","content":"new"},{"role":"user","content":"hello"}]}`,
		},
		{
			name: "messages string", protocol: SystemPromptProtocolMessages,
			body:    `{"system":"old","messages":[{"role":"user","content":"hello"}]}`,
			prepend: `{"system":"new\n\nold","messages":[{"role":"user","content":"hello"}]}`,
			append:  `{"system":"old\n\nnew","messages":[{"role":"user","content":"hello"}]}`,
			replace: `{"system":"new","messages":[{"role":"user","content":"hello"}]}`,
		},
		{
			name: "messages without directives", protocol: SystemPromptProtocolMessages,
			body:    `{"messages":[{"role":"user","content":"hello"}]}`,
			prepend: `{"system":"new","messages":[{"role":"user","content":"hello"}]}`,
			append:  `{"system":"new","messages":[{"role":"user","content":"hello"}]}`,
			replace: `{"system":"new","messages":[{"role":"user","content":"hello"}]}`,
		},
		{
			name: "messages blocks preserve cache", protocol: SystemPromptProtocolMessages,
			body:    `{"system":[{"type":"text","text":"old","cache_control":{"type":"ephemeral","ttl":"1h"},"id":"first"},{"type":"text","text":"last"}],"messages":[]}`,
			prepend: `{"system":[{"type":"text","text":"new"},{"type":"text","text":"old","cache_control":{"type":"ephemeral","ttl":"1h"},"id":"first"},{"type":"text","text":"last"}],"messages":[]}`,
			append:  `{"system":[{"type":"text","text":"old","cache_control":{"type":"ephemeral","ttl":"1h"},"id":"first"},{"type":"text","text":"last"},{"type":"text","text":"new"}],"messages":[]}`,
			replace: `{"system":"new","messages":[]}`,
		},
	}
	for _, tt := range tests {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "new"}
				prepared, err := PrepareGroupSystemPromptBody([]byte(tt.body), tt.protocol, cfg)
				require.NoError(t, err)
				got, err := InjectGroupSystemPromptBody(prepared, tt.protocol, cfg)
				require.NoError(t, err)
				want := map[string]string{"prepend": tt.prepend, "append": tt.append, "replace": tt.replace}[mode]
				require.JSONEq(t, want, string(got))
			})
		}
	}
}

func TestGroupSystemPromptResponsesSystemSequence(t *testing.T) {
	body := []byte(`{"instructions":"first","input":[{"role":"system","content":[{"type":"input_text","text":"second","cache_control":{"type":"ephemeral"}}]},{"type":"additional_tools","role":"developer","tools":[{"role":"system"}]},{"type":"message","role":"developer","content":"last"},{"type":"function_call_output","role":"developer","call_id":"x","output":"result"},{"role":"user","content":"hello"}],"tools":[{"type":"function","parameters":{"system":"schema","properties":{"developer":{"type":"string"}}}}]}`)
	for _, mode := range []string{"prepend", "append", "replace"} {
		t.Run(mode, func(t *testing.T) {
			cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "new"}
			prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolResponses, cfg)
			require.NoError(t, err)
			got, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolResponses, cfg)
			require.NoError(t, err)
			input := gjson.GetBytes(got, "input").Array()
			require.JSONEq(t, gjson.GetBytes(body, "tools").Raw, gjson.GetBytes(got, "tools").Raw)
			switch mode {
			case "prepend":
				require.Equal(t, "new\n\nfirst", gjson.GetBytes(got, "instructions").String())
				require.JSONEq(t, gjson.GetBytes(body, "input").Raw, gjson.GetBytes(got, "input").Raw)
			case "append":
				require.Equal(t, "first", gjson.GetBytes(got, "instructions").String())
				require.Len(t, input, 6)
				require.Equal(t, "new", input[3].Get("content.0.text").String())
				require.Equal(t, "function_call_output", input[4].Get("type").String())
			case "replace":
				require.Equal(t, "new", gjson.GetBytes(got, "instructions").String())
				require.Len(t, input, 3)
				require.Equal(t, "additional_tools", input[0].Get("type").String())
				require.Equal(t, "function_call_output", input[1].Get("type").String())
				require.Equal(t, "result", input[1].Get("output").String())
			}
		})
	}
}

func TestGroupSystemPromptFinalReplacePreservesServerInstructions(t *testing.T) {
	for _, tt := range []struct{ protocol, body, field, want string }{
		{SystemPromptProtocolResponses, `{"instructions":"server-global"}`, "instructions", "server-global\n\nnew"},
		{SystemPromptProtocolChatCompletions, `{"messages":[{"role":"system","content":"server-global"}]}`, "messages.0.content", "server-global"},
		{SystemPromptProtocolMessages, `{"system":"server-global"}`, "system", "server-global\n\nnew"},
	} {
		got, err := InjectGroupSystemPromptBody([]byte(tt.body), tt.protocol, GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "new"})
		require.NoError(t, err)
		require.Equal(t, tt.want, gjson.GetBytes(got, tt.field).String())
	}
}

func TestGroupSystemPromptClaudeIdentity(t *testing.T) {
	identityJSON, err := json.Marshal(claudeCodeSystemPrompt)
	require.NoError(t, err)
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_entrypoint=cli;"},{"type":"text","text":` + string(identityJSON) + `},{"type":"text","text":"ordinary","cache_control":{"type":"ephemeral"}}],"messages":[]}`)
	got, err := InjectGroupSystemPromptBody(body, SystemPromptProtocolMessages, GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "new"})
	require.NoError(t, err)
	require.Equal(t, "x-anthropic-billing-header: cc_entrypoint=cli;", gjson.GetBytes(got, "system.0.text").String())
	require.Equal(t, claudeCodeSystemPrompt, gjson.GetBytes(got, "system.1.text").String())
	require.Equal(t, "new", gjson.GetBytes(got, "system.2.text").String())
	require.Equal(t, "ordinary", gjson.GetBytes(got, "system.3.text").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(got, "system.3.cache_control.type").String())

	stringBody, _ := json.Marshal(map[string]string{"system": claudeCodeSystemPrompt + "\n\nordinary"})
	got, err = InjectGroupSystemPromptBody(stringBody, SystemPromptProtocolMessages, GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "new"})
	require.NoError(t, err)
	require.Equal(t, claudeCodeSystemPrompt+"\n\nnew\n\nordinary", gjson.GetBytes(got, "system").String())
}

func TestGroupSystemPromptReplaceKeepsOnlyClaudeProtocolIdentity(t *testing.T) {
	identity, _ := json.Marshal(claudeCodeSystemPrompt + "\n\nclient business rules")
	body := []byte(`{"instructions":"client","system":[{"type":"text","text":"x-anthropic-billing-header: cc_entrypoint=cli;"},{"type":"text","text":` + string(identity) + `,"cache_control":{"type":"ephemeral"}},{"type":"text","text":"client second block"}],"messages":[{"role":"system","content":"client hidden"},{"role":"user","content":"hi"}]}`)
	cfg := GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "new"}
	prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolMessages, cfg)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(prepared, "instructions").Exists())
	require.Len(t, gjson.GetBytes(prepared, "system").Array(), 2)
	require.Equal(t, claudeCodeSystemPrompt, gjson.GetBytes(prepared, "system.1.text").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(prepared, "system.1.cache_control.type").String())
	require.NotContains(t, string(prepared), "client")
	got, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolMessages, cfg)
	require.NoError(t, err)
	require.Equal(t, "new", gjson.GetBytes(got, "system.2.text").String())
}

func TestGroupSystemPromptPreservesContentAndNeverMatchesForDeduplication(t *testing.T) {
	content := " \nnew\n "
	for _, tt := range []struct{ protocol, body, field string }{
		{SystemPromptProtocolResponses, `{"instructions":"new"}`, "instructions"},
		{SystemPromptProtocolChatCompletions, `{"messages":[{"role":"system","content":"new"}]}`, "messages.1.content"},
		{SystemPromptProtocolMessages, `{"system":[{"type":"text","text":"new"}]}`, "system.1.text"},
	} {
		got, err := InjectGroupSystemPromptBody([]byte(tt.body), tt.protocol, GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: content})
		require.NoError(t, err)
		want := content
		if tt.protocol == SystemPromptProtocolResponses {
			want = "new\n\n" + content
		}
		require.Equal(t, want, gjson.GetBytes(got, tt.field).String())
	}
	got, err := InjectGroupSystemPromptBody([]byte(`{"instructions":"new"}`), SystemPromptProtocolResponses, GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "new"})
	require.NoError(t, err)
	require.Equal(t, "new\n\nnew", gjson.GetBytes(got, "instructions").String())
}

func TestGroupSystemPromptRejectsInvalidTargets(t *testing.T) {
	for _, tt := range []struct{ body, protocol string }{
		{`{`, SystemPromptProtocolResponses},
		{`null`, SystemPromptProtocolResponses},
		{`[]`, SystemPromptProtocolResponses},
		{`{"instructions":[]}`, SystemPromptProtocolResponses},
		{`{"input":123}`, SystemPromptProtocolResponses},
		{`{"system":{}}`, SystemPromptProtocolMessages},
		{`{"messages":"bad"}`, SystemPromptProtocolChatCompletions},
	} {
		for _, apply := range []func([]byte, string, GroupSystemPromptInjection) ([]byte, error){PrepareGroupSystemPromptBody, InjectGroupSystemPromptBody} {
			_, err := apply([]byte(tt.body), tt.protocol, GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "new"})
			require.Error(t, err, tt.body)
		}
	}
	for _, tt := range []struct{ protocol, mode string }{{"unknown", "append"}, {SystemPromptProtocolResponses, "unknown"}} {
		_, err := InjectGroupSystemPromptBody([]byte(`{}`), tt.protocol, GroupSystemPromptInjection{Enabled: true, Mode: tt.mode, Content: "new"})
		require.Error(t, err)
	}
}

func TestGroupSystemPromptResponsesObjectInput(t *testing.T) {
	for _, item := range []string{
		`{"role":"user","content":"hello","cache_control":{"type":"ephemeral"}}`,
		`{"type":"message","role":"system","content":[{"type":"input_text","text":"client"}]}`,
		`{"role":"developer","content":"client"}`,
		`{"type":"additional_tools","role":"developer","tools":[{"role":"system"}]}`,
		`{"type":"function_call_output","role":"developer","call_id":"call_1","output":"keep"}`,
	} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(item+"/"+mode, func(t *testing.T) {
				cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "tenant"}
				body := []byte(`{"instructions":"original","input":` + item + `}`)
				prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolResponses, cfg)
				require.NoError(t, err)
				got, err := InjectGroupSystemPromptBody(prepared, SystemPromptProtocolResponses, cfg)
				require.NoError(t, err)
				input := gjson.GetBytes(got, "input")
				isDirective := isGroupSystemPromptMessage(json.RawMessage(item))
				switch {
				case mode == "replace" && isDirective:
					require.JSONEq(t, `[]`, input.Raw)
					require.Equal(t, "tenant", gjson.GetBytes(got, "instructions").String())
				case mode == "append" && isDirective:
					require.Len(t, input.Array(), 2)
					require.JSONEq(t, item, input.Array()[0].Raw)
					require.Equal(t, "tenant", input.Array()[1].Get("content.0.text").String())
					require.Equal(t, "original", gjson.GetBytes(got, "instructions").String())
				default:
					require.JSONEq(t, item, input.Raw)
				}
			})
		}
	}
}

func TestGroupSystemPromptPreservesUnrelatedProtocolFields(t *testing.T) {
	for _, tt := range []struct {
		protocol, body string
		fields         []string
	}{
		{SystemPromptProtocolResponses, `{"input":[null,5,"opaque",{"type":"additional_tools","role":"developer"}],"system":{"extension":true},"messages":"legacy alias"}`, []string{"input", "system", "messages"}},
		{SystemPromptProtocolChatCompletions, `{"messages":[{"role":"user","content":"hi"}],"instructions":{"extension":true},"system":123,"input":false}`, []string{"instructions", "system", "input"}},
		{SystemPromptProtocolMessages, `{"system":[{"type":"extension","data":{"value":3}},{"type":"text","text":"client","cache_control":{"type":"ephemeral"}}],"messages":"upstream-validated","input":123,"instructions":false}`, []string{"messages", "input", "instructions"}},
	} {
		for _, mode := range []string{"prepend", "append"} {
			cfg := GroupSystemPromptInjection{Enabled: true, Mode: mode, Content: "tenant"}
			prepared, err := PrepareGroupSystemPromptBody([]byte(tt.body), tt.protocol, cfg)
			require.NoError(t, err)
			require.Equal(t, tt.body, string(prepared))
			got, err := InjectGroupSystemPromptBody(prepared, tt.protocol, cfg)
			require.NoError(t, err)
			for _, field := range tt.fields {
				require.JSONEq(t, gjson.Get(tt.body, field).Raw, gjson.GetBytes(got, field).Raw)
			}
		}
	}
}

func TestGroupSystemPromptReplaceStripsBillingBusinessSuffix(t *testing.T) {
	for _, text := range []string{
		"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=d8726;\n\nclient business rules",
		"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=claude-vscode; business=client rules;",
		"x-anthropic-billing-header: cc_entrypoint=cli; client business rules; cc_version=9.9.9;",
	} {
		body, err := json.Marshal(map[string]any{"system": []map[string]any{{"type": "text", "text": text, "cache_control": map[string]string{"type": "ephemeral"}}}})
		require.NoError(t, err)
		prepared, err := PrepareGroupSystemPromptBody(body, SystemPromptProtocolMessages, GroupSystemPromptInjection{Enabled: true, Mode: "replace"})
		require.NoError(t, err)
		require.NotContains(t, string(prepared), "business")
		require.NotContains(t, string(prepared), "client")
		require.NotContains(t, string(prepared), "9.9.9")
		require.Contains(t, gjson.GetBytes(prepared, "system.0.text").String(), "cc_entrypoint=")
		require.Equal(t, "ephemeral", gjson.GetBytes(prepared, "system.0.cache_control.type").String())
	}
}
