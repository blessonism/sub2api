package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 判分"出现即通过"语义与 challenge 判分一致：
// 响应中出现独立整数 token 即通过；糖果问题标准答案为 21。
func TestValidateModelIQAnswer(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		expected string
		want     bool
	}{
		{"正确答案在句尾", "推理……所以最少取出 21 个糖果。", "21", true},
		{"正确答案在句中", "答案是 21 个。", "21", true},
		{"错误答案 20", "答案是 20 个。", "21", false},
		{"错误答案 23", "最少需要 23 个。", "21", false},
		{"错误答案 29", "最坏情况取 29 个。", "21", false},
		{"出现即通过（选定语义）", "不是 21 而是 20。", "21", true},
		{"多位数不误匹配", "答案是 121 个。", "21", false},
		{"多位数不误匹配2", "答案是 210 个。", "21", false},
		{"空响应", "", "21", false},
		{"空期望", "答案是 21 个。", "", false},
		{"非数字期望-包含匹配", "答案是 三十五 个。", "三十五", true},
		{"非数字期望-不包含", "答案是 21 个。", "二十二", false},
		{"非数字期望-大小写不敏感", "Answer: PASS", "pass", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidateModelIQAnswer(tc.text, tc.expected))
		})
	}
}

func TestModelIQProbeConfigDefaults(t *testing.T) {
	cfg := DefaultModelIQProbeConfig()
	require.Equal(t, "21", cfg.ExpectedAnswer)
	require.True(t, strings.Contains(cfg.Question, "苹果味"))
	require.True(t, strings.Contains(cfg.Question, "五角星形"))
	require.Equal(t, 360, cfg.IntervalMinutes)
	require.Equal(t, ModelIQProbeTargetModeAccountModel, cfg.TargetMode)
	require.False(t, cfg.Enabled)
	require.NoError(t, cfg.Validate())
}

func TestModelIQProbeConfigDecode(t *testing.T) {
	// 空/非法 JSON 回落默认值。
	require.Equal(t, DefaultModelIQProbeConfig().ExpectedAnswer, DecodeModelIQProbeConfig("").ExpectedAnswer)
	require.Equal(t, DefaultModelIQProbeConfig().IntervalMinutes, DecodeModelIQProbeConfig("not-json").IntervalMinutes)

	// 部分字段覆盖，缺省字段回落默认值。
	cfg := DecodeModelIQProbeConfig(`{"enabled":true,"expected_answer":"42"}`)
	require.True(t, cfg.Enabled)
	require.Equal(t, "42", cfg.ExpectedAnswer)
	require.Equal(t, DefaultModelIQProbeQuestion, cfg.Question)
	require.Equal(t, 360, cfg.IntervalMinutes)
}

func TestModelIQProbeConfigValidate(t *testing.T) {
	base := DefaultModelIQProbeConfig()

	empty := *base
	empty.ExpectedAnswer = " "
	require.Error(t, empty.Validate())

	badInterval := *base
	badInterval.IntervalMinutes = 4
	require.Error(t, badInterval.Validate())
	badInterval.IntervalMinutes = 10081
	require.Error(t, badInterval.Validate())

	badMode := *base
	badMode.TargetMode = "weird"
	require.Error(t, badMode.Validate())

	longAnswer := *base
	longAnswer.ExpectedAnswer = strings.Repeat("x", 201)
	require.Error(t, longAnswer.Validate())
}

func TestModelIQProbeResultEffectiveStatus(t *testing.T) {
	override := "pass"
	row := &ModelIQProbeResult{Status: ModelIQProbeStatusWrongAnswer, OverrideStatus: &override}
	require.Equal(t, ModelIQProbeStatusPass, row.ResolveEffectiveStatus())

	row.OverrideStatus = nil
	require.Equal(t, ModelIQProbeStatusWrongAnswer, row.ResolveEffectiveStatus())
}

func TestTruncateStringBytesUTF8Safe(t *testing.T) {
	text := strings.Repeat("糖", 3000) // 每字符 3 字节
	truncated := truncateStringBytes(text, 4096)
	require.LessOrEqual(t, len(truncated), 4096)
	require.True(t, strings.HasSuffix(truncated, "糖"))
}

func TestUserSummaryFromRows(t *testing.T) {
	now := time.Now()
	rows := []*ModelIQProbeResult{
		{Model: "gpt-5.2", Status: ModelIQProbeStatusPass, ActualAnswer: "最少 21 个", CreatedAt: now.Add(-1 * time.Hour)},
		{Model: "gpt-5.2", Status: ModelIQProbeStatusWrongAnswer, ActualAnswer: "23 个", CreatedAt: now.Add(-2 * time.Hour)},
		{Model: "gpt-5.2", Status: ModelIQProbeStatusError, ErrorMessage: "timeout", CreatedAt: now.Add(-3 * time.Hour)},
		{Model: "gpt-5.1", Status: ModelIQProbeStatusWrongAnswer, ActualAnswer: strings.Repeat("长", 200), CreatedAt: now.Add(-30 * time.Minute)},
	}

	items := userSummaryFromRows(rows)
	require.Len(t, items, 2)
	// 按模型名排序。
	require.Equal(t, "gpt-5.1", items[0].Model)
	require.Equal(t, "gpt-5.2", items[1].Model)

	latest := items[1].Latest
	require.NotNil(t, latest)
	require.Equal(t, ModelIQProbeStatusPass, latest.Status)
	require.Equal(t, "最少 21 个", latest.AnswerExcerpt)
	// 完整回答随 latest 下发，供前端悬浮展示。
	require.Equal(t, "最少 21 个", latest.Answer)

	// Recent 转为旧→新。
	require.Len(t, items[1].Recent, 3)
	require.Equal(t, ModelIQProbeStatusError, items[1].Recent[0].Status)
	require.Equal(t, ModelIQProbeStatusPass, items[1].Recent[2].Status)

	// 时间线点位带悬浮摘要：回答原文优先，error 行回退错误信息。
	require.Equal(t, "timeout", items[1].Recent[0].AnswerExcerpt)
	require.Equal(t, "23 个", items[1].Recent[1].AnswerExcerpt)
	require.Equal(t, "最少 21 个", items[1].Recent[2].AnswerExcerpt)

	// 摘要按字符截断。
	require.LessOrEqual(t, len([]rune(items[0].Latest.AnswerExcerpt)), modelIQProbeAnswerExcerptMaxRunes)
	require.LessOrEqual(t, len([]rune(items[0].Recent[0].AnswerExcerpt)), modelIQProbeAnswerExcerptMaxRunes)
	// 不含账号信息。
	require.NotContains(t, items[0].Model, "account")
}
