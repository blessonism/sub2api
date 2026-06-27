package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssessConversationTurnQualityCleanOrdinaryConversation(t *testing.T) {
	got := AssessConversationTurnQuality(qualityTestRecord())

	require.Equal(t, ConversationQualityStatusClean, got.QualityStatus)
	require.True(t, got.Exportable)
	require.Empty(t, got.QualityErrors)
}

func TestAssessConversationTurnQualityRejectsHardFailures(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ConversationTurnRecord)
		code string
	}{
		{
			name: "parse failed",
			mut: func(record *ConversationTurnRecord) {
				record.ParseStatus = ConversationParseStatusFailed
			},
			code: ConversationQualityErrorParseFailed,
		},
		{
			name: "truncated",
			mut: func(record *ConversationTurnRecord) {
				record.Truncated = true
			},
			code: ConversationQualityErrorTruncatedPayload,
		},
		{
			name: "client disconnect",
			mut: func(record *ConversationTurnRecord) {
				record.ClientDisconnect = true
			},
			code: ConversationQualityErrorClientDisconnect,
		},
		{
			name: "missing request messages",
			mut: func(record *ConversationTurnRecord) {
				record.RequestMessages = nil
			},
			code: ConversationQualityErrorMissingRequestMessages,
		},
		{
			name: "missing response messages",
			mut: func(record *ConversationTurnRecord) {
				record.ResponseMessages = nil
			},
			code: ConversationQualityErrorMissingResponseMessages,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := qualityTestRecord()
			tc.mut(&record)

			got := AssessConversationTurnQuality(record)

			require.Equal(t, ConversationQualityStatusRejected, got.QualityStatus)
			require.False(t, got.Exportable)
			requireQualityErrorCode(t, got.QualityErrors, tc.code)
		})
	}
}

func TestAssessConversationTurnQualityHeuristicNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.SessionSource = ConversationSessionSourceHeuristic

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorHeuristicSession)
}

func TestAssessConversationTurnQualityToolCallChainNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}`)}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorIncompleteToolCallChain)
}

func TestAssessConversationTurnQualityOrphanToolResultNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.RequestMessages = append(record.RequestMessages, json.RawMessage(`{"role":"tool","tool_call_id":"call_missing","content":"ok"}`))

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorOrphanToolResult)
}

func TestAssessConversationTurnQualityEmptyAssistantOutputNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"role":"assistant","content":""}`)}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorEmptyAssistantOutput)
}

func TestAssessConversationTurnQualityResponsesToolOnlyWithoutResultNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[]}`)}
	record.Tools = []json.RawMessage{json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}`)}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorIncompleteToolCallChain)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorEmptyAssistantOutput)
}

func TestAssessConversationTurnQualityResponsesToolCallOutputWithoutCallNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[]}`)}
	record.Tools = []json.RawMessage{json.RawMessage(`{"type":"function_call_output","call_id":"call_1","output":"ok"}`)}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorOrphanToolResult)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorEmptyAssistantOutput)
}

func TestAssessConversationTurnQualityResponsesToolCallWithResultAllowsEmptyAssistantText(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[]}`)}
	record.Tools = []json.RawMessage{
		json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}`),
		json.RawMessage(`{"type":"function_call_output","call_id":"call_1","output":"ok"}`),
	}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusClean, got.QualityStatus)
	require.True(t, got.Exportable)
	require.Empty(t, got.QualityErrors)
}

func TestAssessConversationTurnQualityResponsesToolCallWithEmptyResultNeedsReview(t *testing.T) {
	record := qualityTestRecord()
	record.ResponseMessages = []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[]}`)}
	record.Tools = []json.RawMessage{
		json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}`),
		json.RawMessage(`{"type":"function_call_output","call_id":"call_1","output":""}`),
	}

	got := AssessConversationTurnQuality(record)

	require.Equal(t, ConversationQualityStatusNeedsReview, got.QualityStatus)
	require.False(t, got.Exportable)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorIncompleteToolCallChain)
	requireQualityErrorCode(t, got.QualityErrors, ConversationQualityErrorEmptyAssistantOutput)
}

func qualityTestRecord() ConversationTurnRecord {
	return ConversationTurnRecord{
		SessionSource:    ConversationSessionSourceExplicit,
		ParseStatus:      ConversationParseStatusSuccess,
		RequestMessages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)},
		ResponseMessages: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"hi"}`)},
	}
}

func requireQualityErrorCode(t *testing.T, errors []QualityError, code string) {
	t.Helper()
	for _, err := range errors {
		if err.Code == code {
			return
		}
	}
	require.Failf(t, "quality error code not found", "code=%s errors=%v", code, errors)
}
