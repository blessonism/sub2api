package domain

// GroupSystemPromptInjection 是分组级自定义系统提示词配置。
type GroupSystemPromptInjection struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
}

// WithDefaults 补齐省略的模式；正文和开关保持原值。
func (c GroupSystemPromptInjection) WithDefaults() GroupSystemPromptInjection {
	if c.Mode == "" {
		c.Mode = "append"
	}
	return c
}
