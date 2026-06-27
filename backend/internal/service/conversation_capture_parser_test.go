package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOpenAIChatCompletionsTurnNonStream(t *testing.T) {
	req := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	resp := []byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`)

	got := parseOpenAIChatCompletionsTurn(req, resp, false)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Len(t, got.RequestMessages, 1)
	require.Len(t, got.ResponseMessages, 1)
}

func TestParseOpenAIChatCompletionsTurnStream(t *testing.T) {
	req := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	resp := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"llo\"}}]}\n\ndata: [DONE]\n\n")

	got := parseOpenAIChatCompletionsTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusSuccess, got.ParseStatus)
	require.Len(t, got.ResponseMessages, 1)
	require.Contains(t, string(got.ResponseMessages[0]), "hello")
}

func TestParseOpenAIChatCompletionsTurnFailedKeepsStatus(t *testing.T) {
	got := parseOpenAIChatCompletionsTurn([]byte(`{"messages":[]}`), []byte(`bad`), false)

	require.Equal(t, ConversationParseStatusFailed, got.ParseStatus)
	require.NotEmpty(t, got.ParseError)
}

func TestParseOpenAIChatCompletionsTurnStreamRejectsInvalidJSON(t *testing.T) {
	req := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	resp := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\ndata: {invalid json}\n\ndata: [DONE]\n\n")

	got := parseOpenAIChatCompletionsTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusFailed, got.ParseStatus)
	require.Contains(t, got.ParseError, "JSON parse failed")
}

func TestParseOpenAIChatCompletionsTurnStreamRejectsMissingDone(t *testing.T) {
	req := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	resp := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")

	got := parseOpenAIChatCompletionsTurn(req, resp, true)

	require.Equal(t, ConversationParseStatusFailed, got.ParseStatus)
	require.Contains(t, got.ParseError, "done marker missing")
}
