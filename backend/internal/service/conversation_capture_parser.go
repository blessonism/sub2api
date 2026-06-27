package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/tidwall/gjson"
)

type conversationParseResult struct {
	RequestMessages    []json.RawMessage
	ResponseMessages   []json.RawMessage
	Tools              []json.RawMessage
	ParseStatus        string
	ParseError         string
	ResponseID         string
	PreviousResponseID string
}

func parseOpenAIChatCompletionsTurn(requestBody, responseRaw []byte, stream bool) conversationParseResult {
	result := conversationParseResult{ParseStatus: ConversationParseStatusSuccess}
	requestMessages, err := extractChatRequestMessages(requestBody)
	if err != nil {
		result.ParseStatus = ConversationParseStatusFailed
		result.ParseError = err.Error()
		return result
	}
	result.RequestMessages = requestMessages

	var assistant json.RawMessage
	if stream {
		assistant, err = parseChatCompletionsStreamAssistant(responseRaw)
	} else {
		assistant, err = parseChatCompletionsNonStreamAssistant(responseRaw)
	}
	if err != nil {
		result.ParseStatus = ConversationParseStatusFailed
		result.ParseError = err.Error()
		return result
	}
	result.ResponseMessages = []json.RawMessage{assistant}
	return result
}

func parseOpenAIResponsesTurn(requestBody, responseRaw []byte, stream bool) conversationParseResult {
	result := conversationParseResult{ParseStatus: ConversationParseStatusSuccess}
	requestMessages, previousResponseID, err := extractResponsesRequestMessages(requestBody)
	if err != nil {
		result.ParseStatus = ConversationParseStatusFailed
		result.ParseError = err.Error()
		return result
	}
	result.RequestMessages = requestMessages
	result.PreviousResponseID = previousResponseID

	var responseMessages, tools []json.RawMessage
	var responseID string
	if stream {
		responseMessages, tools, responseID, err = parseResponsesStream(responseRaw)
	} else {
		responseMessages, tools, responseID, err = parseResponsesNonStream(responseRaw)
	}
	if err != nil {
		result.ParseStatus = ConversationParseStatusFailed
		result.ParseError = err.Error()
		return result
	}
	result.ResponseMessages = responseMessages
	result.Tools = tools
	result.ResponseID = responseID
	return result
}

func extractChatRequestMessages(body []byte) ([]json.RawMessage, error) {
	var payload struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("request JSON parse failed")
	}
	if len(payload.Messages) == 0 {
		return nil, errors.New("request messages missing")
	}
	return payload.Messages, nil
}

func extractResponsesRequestMessages(body []byte) ([]json.RawMessage, string, error) {
	var payload struct {
		Input              json.RawMessage `json:"input"`
		PreviousResponseID string          `json:"previous_response_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", errors.New("responses request JSON parse failed")
	}
	messages := responsesInputToMessages(payload.Input)
	if len(messages) == 0 {
		return nil, "", errors.New("responses request input missing")
	}
	return messages, strings.TrimSpace(payload.PreviousResponseID), nil
}

func parseChatCompletionsNonStreamAssistant(body []byte) (json.RawMessage, error) {
	var payload struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("response JSON parse failed")
	}
	for _, choice := range payload.Choices {
		if len(choice.Message) == 0 {
			continue
		}
		if strings.EqualFold(gjson.GetBytes(choice.Message, "role").String(), "assistant") {
			return choice.Message, nil
		}
	}
	return nil, errors.New("assistant message missing")
}

func parseChatCompletionsStreamAssistant(body []byte) (json.RawMessage, error) {
	var content strings.Builder
	seenDone := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			seenDone = true
			continue
		}
		if !gjson.Valid(data) {
			return nil, errors.New("stream response JSON parse failed")
		}
		choices := gjson.Get(data, "choices")
		choices.ForEach(func(_, choice gjson.Result) bool {
			delta := choice.Get("delta")
			if part := delta.Get("content"); part.Exists() && part.Type == gjson.String {
				content.WriteString(part.String())
			}
			return true
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("stream response scan failed")
	}
	if !seenDone {
		return nil, errors.New("assistant stream done marker missing")
	}
	if content.Len() == 0 {
		return nil, errors.New("assistant stream content missing")
	}
	msg := map[string]any{
		"role":    "assistant",
		"content": content.String(),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return nil, errors.New("assistant stream marshal failed")
	}
	return raw, nil
}

func parseResponsesNonStream(body []byte) ([]json.RawMessage, []json.RawMessage, string, error) {
	if !gjson.ValidBytes(body) {
		return nil, nil, "", errors.New("responses response JSON parse failed")
	}
	responseID := strings.TrimSpace(gjson.GetBytes(body, "id").String())
	output := gjson.GetBytes(body, "output")
	if !output.Exists() || !output.IsArray() {
		return nil, nil, responseID, errors.New("responses output missing")
	}
	var messages []json.RawMessage
	var tools []json.RawMessage
	output.ForEach(func(_, item gjson.Result) bool {
		consumeResponsesOutputItem(item, &messages, &tools)
		return true
	})
	if len(messages) == 0 && len(tools) == 0 {
		return nil, nil, responseID, errors.New("responses output items missing")
	}
	return messages, tools, responseID, nil
}

func parseResponsesStream(body []byte) ([]json.RawMessage, []json.RawMessage, string, error) {
	var text strings.Builder
	var messages []json.RawMessage
	var tools []json.RawMessage
	var responseID string
	seenTerminal := false
	hasCompletedOutput := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			seenTerminal = true
			continue
		}
		if !gjson.Valid(data) {
			return nil, nil, responseID, errors.New("responses stream JSON parse failed")
		}
		event := gjson.Parse(data)
		if id := strings.TrimSpace(event.Get("response.id").String()); id != "" {
			responseID = id
		}
		if id := strings.TrimSpace(event.Get("id").String()); responseID == "" && strings.HasPrefix(id, "resp_") {
			responseID = id
		}
		eventType := event.Get("type").String()
		switch {
		case eventType == "response.output_text.delta":
			if delta := event.Get("delta"); delta.Exists() && delta.Type == gjson.String {
				text.WriteString(delta.String())
			}
		case eventType == "response.output_text.done":
			if value := event.Get("text"); value.Exists() && value.Type == gjson.String && text.Len() == 0 {
				text.WriteString(value.String())
			}
		case eventType == "response.output_item.done":
			item := event.Get("item")
			if item.Exists() {
				consumeResponsesOutputItem(item, &messages, &tools)
			}
		case eventType == "response.completed":
			seenTerminal = true
			output := event.Get("response.output")
			output.ForEach(func(_, item gjson.Result) bool {
				consumeResponsesOutputItem(item, &messages, &tools)
				hasCompletedOutput = true
				return true
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, responseID, errors.New("responses stream scan failed")
	}
	if text.Len() > 0 && !hasCompletedOutput {
		messages = append(messages, mustMarshalConversationRaw(map[string]any{"role": "assistant", "content": text.String()}))
	}
	if !seenTerminal {
		return nil, nil, responseID, errors.New("responses stream terminal event missing")
	}
	if len(messages) == 0 && len(tools) == 0 {
		return nil, nil, responseID, errors.New("responses stream output missing")
	}
	return messages, tools, responseID, nil
}

func responsesInputToMessages(input json.RawMessage) []json.RawMessage {
	if len(input) == 0 || string(input) == "null" {
		return nil
	}
	if gjson.ValidBytes(input) {
		value := gjson.ParseBytes(input)
		if value.Type == gjson.String {
			return []json.RawMessage{mustMarshalConversationRaw(map[string]any{"role": "user", "content": value.String()})}
		}
		if value.IsArray() {
			var out []json.RawMessage
			value.ForEach(func(_, item gjson.Result) bool {
				raw := item.Raw
				itemType := item.Get("type").String()
				role := strings.TrimSpace(item.Get("role").String())
				switch {
				case role != "":
					out = append(out, json.RawMessage(raw))
				case itemType == "input_text":
					out = append(out, mustMarshalConversationRaw(map[string]any{"role": "user", "content": item.Get("text").String()}))
				case itemType == "message":
					out = append(out, normalizeResponsesMessageItem(item))
				default:
					out = append(out, json.RawMessage(raw))
				}
				return true
			})
			return out
		}
		if value.IsObject() {
			return []json.RawMessage{json.RawMessage(value.Raw)}
		}
	}
	return []json.RawMessage{mustMarshalConversationRaw(map[string]any{"role": "user", "content": string(input)})}
}

func consumeResponsesOutputItem(item gjson.Result, messages *[]json.RawMessage, tools *[]json.RawMessage) {
	itemType := item.Get("type").String()
	switch itemType {
	case "message":
		*messages = append(*messages, normalizeResponsesMessageItem(item))
	case "function_call", "tool_call", "web_search_call", "computer_call", "file_search_call", "image_generation_call", "code_interpreter_call":
		*tools = append(*tools, json.RawMessage(item.Raw))
	case "function_call_output", "tool_result":
		*tools = append(*tools, json.RawMessage(item.Raw))
	default:
		if strings.Contains(itemType, "call") || strings.Contains(itemType, "tool") {
			*tools = append(*tools, json.RawMessage(item.Raw))
			return
		}
		if content := item.Get("content"); content.Exists() {
			*messages = append(*messages, normalizeResponsesMessageItem(item))
		}
	}
}

func normalizeResponsesMessageItem(item gjson.Result) json.RawMessage {
	role := strings.TrimSpace(item.Get("role").String())
	if role == "" {
		role = "assistant"
	}
	contentText := responsesContentText(item.Get("content"))
	if contentText != "" {
		return mustMarshalConversationRaw(map[string]any{"role": role, "content": contentText})
	}
	return json.RawMessage(item.Raw)
}

func responsesContentText(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}
	var b strings.Builder
	content.ForEach(func(_, part gjson.Result) bool {
		if text := part.Get("text"); text.Exists() && text.Type == gjson.String {
			b.WriteString(text.String())
			return true
		}
		if text := part.Get("output_text"); text.Exists() && text.Type == gjson.String {
			b.WriteString(text.String())
			return true
		}
		return true
	})
	return b.String()
}

func mustMarshalConversationRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
