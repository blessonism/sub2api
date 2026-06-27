package service

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	ConversationQualityErrorParseFailed             = "parse_failed"
	ConversationQualityErrorTruncatedPayload        = "truncated_payload"
	ConversationQualityErrorClientDisconnect        = "client_disconnect"
	ConversationQualityErrorMissingRequestMessages  = "missing_request_messages"
	ConversationQualityErrorMissingResponseMessages = "missing_response_messages"
	ConversationQualityErrorHeuristicSession        = "heuristic_session"
	ConversationQualityErrorIncompleteToolCallChain = "incomplete_tool_call_chain"
	ConversationQualityErrorOrphanToolResult        = "orphan_tool_result"
	ConversationQualityErrorEmptyAssistantOutput    = "empty_assistant_output"
)

type ConversationQualityAssessment struct {
	QualityStatus string
	QualityErrors []QualityError
	Exportable    bool
}

func AssessConversationTurnQuality(record ConversationTurnRecord) ConversationQualityAssessment {
	input := conversationQualityInput{
		SessionSource:    record.SessionSource,
		ParseStatus:      record.ParseStatus,
		RequestMessages:  record.RequestMessages,
		ResponseMessages: record.ResponseMessages,
		Tools:            record.Tools,
		ClientDisconnect: record.ClientDisconnect,
		Truncated:        record.Truncated,
	}
	return assessConversationQuality(input)
}

func CanExportConversationTurn(turn ConversationTurn, req ConversationExportMessagesJSONLRequest) bool {
	if !turn.Exportable ||
		turn.ParseStatus != ConversationParseStatusSuccess ||
		turn.Truncated ||
		turn.ClientDisconnect ||
		turn.QualityStatus != ConversationQualityStatusClean {
		return false
	}
	sessionSource, _ := turn.Meta["session_source"].(string)
	if strings.TrimSpace(sessionSource) == ConversationSessionSourceHeuristic && !req.IncludeHeuristic {
		return false
	}
	recheckSessionSource := sessionSource
	if req.IncludeHeuristic && strings.TrimSpace(sessionSource) == ConversationSessionSourceHeuristic {
		recheckSessionSource = ""
	}
	return assessConversationQuality(conversationQualityInput{
		SessionSource:    recheckSessionSource,
		ParseStatus:      turn.ParseStatus,
		RequestMessages:  turn.RequestMessages,
		ResponseMessages: turn.ResponseMessages,
		Tools:            turn.Tools,
		ClientDisconnect: turn.ClientDisconnect,
		Truncated:        turn.Truncated,
	}).QualityStatus == ConversationQualityStatusClean
}

func FilterConversationExportableTurns(turns []ConversationTurn, req ConversationExportMessagesJSONLRequest) []ConversationTurn {
	out := make([]ConversationTurn, 0, len(turns))
	for _, turn := range turns {
		if CanExportConversationTurn(turn, req) {
			out = append(out, turn)
		}
	}
	return out
}

type conversationQualityInput struct {
	SessionSource    string
	ParseStatus      string
	RequestMessages  []json.RawMessage
	ResponseMessages []json.RawMessage
	Tools            []json.RawMessage
	ClientDisconnect bool
	Truncated        bool
}

func assessConversationQuality(input conversationQualityInput) ConversationQualityAssessment {
	status := ConversationQualityStatusClean
	errors := []QualityError{}
	addRejected := func(code, message string) {
		status = ConversationQualityStatusRejected
		errors = append(errors, QualityError{Code: code, Message: message, Source: "auto"})
	}
	addReview := func(code, message string) {
		if status != ConversationQualityStatusRejected {
			status = ConversationQualityStatusNeedsReview
		}
		errors = append(errors, QualityError{Code: code, Message: message, Source: "auto"})
	}

	if strings.TrimSpace(input.ParseStatus) != ConversationParseStatusSuccess {
		addRejected(ConversationQualityErrorParseFailed, "conversation parse failed")
	}
	if input.Truncated {
		addRejected(ConversationQualityErrorTruncatedPayload, "payload was truncated")
	}
	if input.ClientDisconnect {
		addRejected(ConversationQualityErrorClientDisconnect, "client disconnected before response completed")
	}
	if len(input.RequestMessages) == 0 {
		addRejected(ConversationQualityErrorMissingRequestMessages, "request messages are missing")
	}
	if len(input.ResponseMessages) == 0 {
		addRejected(ConversationQualityErrorMissingResponseMessages, "response messages are missing")
	}
	if strings.TrimSpace(input.SessionSource) == ConversationSessionSourceHeuristic {
		addReview(ConversationQualityErrorHeuristicSession, "session was inferred heuristically")
	}

	toolLinks := collectConversationToolLinks(input.RequestMessages, input.ResponseMessages, input.Tools)
	if len(toolLinks.calls) > 0 {
		for id := range toolLinks.calls {
			if _, ok := toolLinks.validResults[id]; !ok {
				addReview(ConversationQualityErrorIncompleteToolCallChain, "tool call is missing a matching tool result")
				break
			}
		}
	}
	if len(toolLinks.results) > 0 {
		for id := range toolLinks.results {
			if _, ok := toolLinks.calls[id]; !ok {
				addReview(ConversationQualityErrorOrphanToolResult, "tool result is missing a matching tool call")
				break
			}
		}
	}
	if len(input.ResponseMessages) > 0 && !conversationMessagesHaveAssistantOutput(input.ResponseMessages) && !toolLinks.hasCompleteValidChain() {
		addReview(ConversationQualityErrorEmptyAssistantOutput, "assistant output is empty")
	}

	return ConversationQualityAssessment{
		QualityStatus: status,
		QualityErrors: normalizeConversationQualityErrors(errors),
		Exportable:    status == ConversationQualityStatusClean,
	}
}

type conversationToolLinks struct {
	calls        map[string]struct{}
	results      map[string]struct{}
	validResults map[string]struct{}
}

func (links conversationToolLinks) hasCompleteValidChain() bool {
	if len(links.calls) == 0 || len(links.validResults) == 0 {
		return false
	}
	for id := range links.calls {
		if _, ok := links.validResults[id]; !ok {
			return false
		}
	}
	for id := range links.validResults {
		if _, ok := links.calls[id]; !ok {
			return false
		}
	}
	return true
}

func collectConversationToolLinks(groups ...[]json.RawMessage) conversationToolLinks {
	links := conversationToolLinks{
		calls:        map[string]struct{}{},
		results:      map[string]struct{}{},
		validResults: map[string]struct{}{},
	}
	for _, group := range groups {
		for _, raw := range group {
			value := gjson.ParseBytes(raw)
			collectConversationToolCalls(value, links.calls)
			collectConversationToolResults(value, links.results, links.validResults)
		}
	}
	return links
}

func collectConversationToolCalls(value gjson.Result, out map[string]struct{}) {
	for _, path := range []string{"tool_calls", "function_call", "function_calls"} {
		item := value.Get(path)
		if !item.Exists() {
			continue
		}
		if item.IsArray() {
			item.ForEach(func(_, child gjson.Result) bool {
				addConversationPreferredToolID(child, out)
				return true
			})
			continue
		}
		addConversationPreferredToolID(item, out)
	}
	if conversationToolCallItemType(strings.TrimSpace(value.Get("type").String())) {
		addConversationPreferredToolID(value, out)
	}
}

func collectConversationToolResults(value gjson.Result, out map[string]struct{}, validOut map[string]struct{}) {
	role := strings.TrimSpace(value.Get("role").String())
	itemType := strings.TrimSpace(value.Get("type").String())
	if role == "tool" || role == "function" || itemType == "function_call_output" || itemType == "tool_result" {
		id := conversationPreferredToolID(value)
		addConversationToolID(id, out)
		if conversationToolResultHasOutput(value) {
			addConversationToolID(id, validOut)
		}
	}
}

func conversationToolCallItemType(itemType string) bool {
	switch itemType {
	case "function_call", "tool_call", "web_search_call", "computer_call", "file_search_call", "image_generation_call", "code_interpreter_call":
		return true
	default:
		return false
	}
}

func addConversationPreferredToolID(value gjson.Result, out map[string]struct{}) {
	addConversationToolID(conversationPreferredToolID(value), out)
}

func conversationPreferredToolID(value gjson.Result) string {
	for _, path := range []string{"tool_call_id", "call_id", "tool_use_id", "id"} {
		if id := strings.TrimSpace(value.Get(path).String()); id != "" {
			return id
		}
	}
	return ""
}

func addConversationToolID(id string, out map[string]struct{}) {
	id = strings.TrimSpace(id)
	if id != "" {
		out[id] = struct{}{}
	}
}

func conversationToolResultHasOutput(value gjson.Result) bool {
	for _, path := range []string{"content", "output", "text", "result"} {
		item := value.Get(path)
		if conversationJSONValueHasContent(item) {
			return true
		}
	}
	return false
}

func conversationJSONValueHasContent(value gjson.Result) bool {
	if !value.Exists() {
		return false
	}
	switch value.Type {
	case gjson.String:
		return strings.TrimSpace(value.String()) != ""
	case gjson.Number, gjson.True, gjson.False:
		return true
	case gjson.JSON:
		if value.IsArray() {
			hasContent := false
			value.ForEach(func(_, child gjson.Result) bool {
				if conversationJSONValueHasContent(child.Get("text")) ||
					conversationJSONValueHasContent(child.Get("content")) ||
					conversationJSONValueHasContent(child.Get("output_text")) ||
					conversationJSONValueHasContent(child.Get("output")) ||
					conversationJSONValueHasContent(child.Get("result")) {
					hasContent = true
					return false
				}
				return true
			})
			return hasContent
		}
		return conversationJSONValueHasContent(value.Get("text")) ||
			conversationJSONValueHasContent(value.Get("content")) ||
			conversationJSONValueHasContent(value.Get("output_text")) ||
			conversationJSONValueHasContent(value.Get("output")) ||
			conversationJSONValueHasContent(value.Get("result"))
	default:
		return false
	}
}

func conversationMessagesHaveAssistantOutput(messages []json.RawMessage) bool {
	for _, raw := range messages {
		value := gjson.ParseBytes(raw)
		role := strings.TrimSpace(value.Get("role").String())
		itemType := strings.TrimSpace(value.Get("type").String())
		if role != "" && role != "assistant" && itemType != "message" {
			continue
		}
		content := value.Get("content")
		if (content.Type == gjson.String && strings.TrimSpace(content.String()) != "") ||
			(value.Get("text").Type == gjson.String && strings.TrimSpace(value.Get("text").String()) != "") ||
			(value.Get("output_text").Type == gjson.String && strings.TrimSpace(value.Get("output_text").String()) != "") {
			return true
		}
		if content.IsArray() {
			hasText := false
			content.ForEach(func(_, part gjson.Result) bool {
				if strings.TrimSpace(part.Get("text").String()) != "" ||
					strings.TrimSpace(part.Get("content").String()) != "" {
					hasText = true
					return false
				}
				return true
			})
			if hasText {
				return true
			}
		}
	}
	return false
}
