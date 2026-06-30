//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseGptIntelligenceHTML_ExtractsPublicModelIqTitles(t *testing.T) {
	collectedAt := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	raw := `
<svg>
  <title>6.29_am GPT-5.5 xhigh: IQ指数 87.5, 7/12, 费用 $42.61, 耗时 156分钟, cache命中率 93.4%</title>
  <title>6.29_pm GPT-5.5 xhigh: IQ指数 75.0, 6/12, 费用 $42.00, 耗时 204分钟, cache命中率 95.2%</title>
  <title>6.29_pm GPT-5.5 high: IQ指数 87.5, 7/12, 费用 $26.31, 耗时 109分钟, cache命中率 93.8%</title>
  <title>6.29_pm GPT-5.4 xhigh: IQ指数 87.5, 7/12, 费用 $21.13, 耗时 258分钟, cache命中率 95.1%</title>
</svg>`

	snapshot, err := ParseGptIntelligenceHTML(raw, collectedAt)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Latest)
	require.Equal(t, "2026-06-29-pm", snapshot.Latest.Date)
	require.Equal(t, "GPT-5.5", snapshot.Latest.Model)
	require.Equal(t, "xhigh", snapshot.Latest.ReasoningEffort)
	require.NotNil(t, snapshot.Latest.Score)
	require.Equal(t, 75.0, *snapshot.Latest.Score)
	require.NotNil(t, snapshot.Latest.Passed)
	require.Equal(t, 6.0, *snapshot.Latest.Passed)
	require.NotNil(t, snapshot.Latest.Tasks)
	require.Equal(t, 12.0, *snapshot.Latest.Tasks)
	require.NotNil(t, snapshot.Latest.WallSeconds)
	require.Equal(t, 12240.0, *snapshot.Latest.WallSeconds)
	require.NotNil(t, snapshot.Latest.CostUSD)
	require.Equal(t, 42.0, *snapshot.Latest.CostUSD)
	require.Len(t, snapshot.RecentDays, 2)
	require.Len(t, snapshot.Comparisons, 2)
	require.Equal(t, "gpt_55_high", snapshot.Comparisons[0].Key)
	require.Equal(t, "public_html_title", snapshot.Metadata.Method)
	require.Equal(t, 4, snapshot.Metadata.RunCount)
	require.Equal(t, gptIntelligenceSourceURL, snapshot.Source.URL)
	require.Len(t, snapshot.Templates, 3)
}

func TestParseGptIntelligenceHTML_ReturnsUnavailableWhenTitlesMissing(t *testing.T) {
	_, err := ParseGptIntelligenceHTML("<html></html>", time.Now())

	require.Error(t, err)
	require.True(t, infraErrorIsServiceUnavailable(err))
}

func TestNewGptIntelligenceHTTPClient_DisablesEnvironmentProxy(t *testing.T) {
	client := newGptIntelligenceHTTPClient()
	transport, ok := client.Transport.(*http.Transport)

	require.True(t, ok)
	require.Nil(t, transport.Proxy)
	require.Equal(t, gptIntelligenceFetchTimeout, client.Timeout)
}

func TestCloneGptIntelligenceSnapshot_DeepCopiesRunPointers(t *testing.T) {
	score := 80.0
	original := &GptIntelligenceSnapshot{
		Latest: &GptIntelligenceRun{Score: &score},
		Templates: []GptIntelligencePromptTemplate{
			{ID: "logic", Title: "原始标题", Prompt: "原始题目", Expected: "原始期望", Threshold: "原始阈值"},
		},
		RecentDays: []GptIntelligenceRun{
			{Score: &score},
		},
		Comparisons: []GptIntelligenceComparison{
			{
				Latest: &GptIntelligenceRun{Score: &score},
				RecentDays: []GptIntelligenceRun{
					{Score: &score},
				},
			},
		},
	}

	cloned := cloneGptIntelligenceSnapshot(original)
	require.NotNil(t, cloned)

	*cloned.Latest.Score = 70
	*cloned.RecentDays[0].Score = 60
	*cloned.Comparisons[0].Latest.Score = 50
	*cloned.Comparisons[0].RecentDays[0].Score = 40
	cloned.Templates[0].Title = "修改标题"

	require.Equal(t, 80.0, *original.Latest.Score)
	require.Equal(t, 80.0, *original.RecentDays[0].Score)
	require.Equal(t, 80.0, *original.Comparisons[0].Latest.Score)
	require.Equal(t, 80.0, *original.Comparisons[0].RecentDays[0].Score)
	require.Equal(t, "原始标题", original.Templates[0].Title)
}

func TestEncodeGptIntelligencePromptTemplates_NormalizesKnownTemplates(t *testing.T) {
	defaults := DefaultGptIntelligencePromptTemplates()
	defaults[0].Title = "  新逻辑题  "

	raw, normalized, err := EncodeGptIntelligencePromptTemplates(defaults)

	require.NoError(t, err)
	require.Contains(t, raw, `"id":"logic"`)
	require.Len(t, normalized, 3)
	require.Equal(t, "新逻辑题", normalized[0].Title)
}

func TestEncodeGptIntelligencePromptTemplates_PreservesCustomTemplates(t *testing.T) {
	templates := DefaultGptIntelligencePromptTemplates()
	templates = append(templates, GptIntelligencePromptTemplate{
		ID:          "custom-1",
		Title:       "自定义题",
		Description: "管理员新增",
		Prompt:      "自定义 Prompt",
		Expected:    "自定义期望",
		Threshold:   "自定义阈值",
	})

	_, normalized, err := EncodeGptIntelligencePromptTemplates(templates)

	require.NoError(t, err)
	require.Len(t, normalized, 4)
	require.Equal(t, "custom-1", normalized[3].ID)
	require.Equal(t, "自定义题", normalized[3].Title)
}

func TestEncodeGptIntelligencePromptTemplates_DoesNotRestoreDeletedDefaultTemplates(t *testing.T) {
	_, normalized, err := EncodeGptIntelligencePromptTemplates([]GptIntelligencePromptTemplate{
		{
			ID:        "custom-1",
			Title:     "自定义题",
			Prompt:    "自定义 Prompt",
			Expected:  "自定义期望",
			Threshold: "自定义阈值",
		},
	})

	require.NoError(t, err)
	require.Len(t, normalized, 1)
	require.Equal(t, "custom-1", normalized[0].ID)
}

func TestEncodeGptIntelligencePromptTemplates_AllowsEmptyTemplates(t *testing.T) {
	raw, normalized, err := EncodeGptIntelligencePromptTemplates(nil)

	require.NoError(t, err)
	require.Equal(t, "[]", raw)
	require.Empty(t, normalized)
}

func TestEncodeGptIntelligencePromptTemplates_RejectsEmptyPrompt(t *testing.T) {
	defaults := DefaultGptIntelligencePromptTemplates()
	defaults[0].Prompt = " "

	_, _, err := EncodeGptIntelligencePromptTemplates(defaults)

	require.Error(t, err)
	require.True(t, ErrGptIntelligenceTemplateInvalid.Is(err))
}

func infraErrorIsServiceUnavailable(err error) bool {
	return err != nil && ErrGptIntelligenceUnavailable.Is(err)
}
