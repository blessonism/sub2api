package service

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// System One 没有顶层系统指令通道，每个问题的 instructions 是独立指令区。
// 只改该字段，不重写问题类型、criteria 或承载输入数据的 state。
func groupSystemPromptSystemOneQuestions(payload map[string]json.RawMessage) (map[string]map[string]json.RawMessage, error) {
	var questions map[string]map[string]json.RawMessage
	if err := json.Unmarshal(payload["questions"], &questions); err != nil || questions == nil {
		return nil, fmt.Errorf("invalid System One questions")
	}
	for id, question := range questions {
		if question == nil {
			return nil, fmt.Errorf("invalid System One question %q", id)
		}
	}
	return questions, nil
}

func validateGroupSystemPromptSystemOne(payload map[string]json.RawMessage) error {
	_, err := groupSystemPromptSystemOneQuestions(payload)
	return err
}

func prepareGroupSystemPromptSystemOne(payload map[string]json.RawMessage) ([]byte, error) {
	questions, err := groupSystemPromptSystemOneQuestions(payload)
	if err != nil {
		return nil, err
	}
	for _, question := range questions {
		delete(question, "instructions")
	}
	payload["questions"], _ = json.Marshal(questions)
	return json.Marshal(payload)
}

func injectGroupSystemPromptSystemOne(payload map[string]json.RawMessage, cfg GroupSystemPromptInjection) ([]byte, error) {
	questions, err := groupSystemPromptSystemOneQuestions(payload)
	if err != nil {
		return nil, err
	}
	for _, question := range questions {
		raw := bytes.TrimSpace(question["instructions"])
		var text string
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &text) == nil {
			question["instructions"], _ = json.Marshal(joinGroupSystemPrompt(text, cfg.Content, cfg.Mode == "prepend"))
			continue
		}
		var items []json.RawMessage
		switch raw[0] {
		case '[':
			_ = json.Unmarshal(raw, &items)
		case '{':
			items = []json.RawMessage{raw}
		default:
			return nil, fmt.Errorf("invalid System One instructions")
		}
		index := len(items)
		if cfg.Mode == "prepend" {
			index = 0
		}
		part, _ := json.Marshal(cfg.Content)
		question["instructions"], _ = json.Marshal(insertGroupSystemPromptItem(items, index, part))
	}
	payload["questions"], _ = json.Marshal(questions)
	return json.Marshal(payload)
}
