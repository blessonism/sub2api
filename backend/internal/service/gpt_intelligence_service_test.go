//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type gptIntelligenceRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn gptIntelligenceRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestGptIntelligenceServiceFetchSnapshot_UsesPublicJSONEndpoint(t *testing.T) {
	var requestedURLs []string
	service := &GptIntelligenceService{
		client: &http.Client{Transport: gptIntelligenceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestedURLs = append(requestedURLs, req.URL.String())
			require.Equal(t, "application/json", req.Header.Get("Accept"))
			if req.URL.String() == gptIntelligenceEfficiencyURL {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
  "source_updated_at":"2026-07-11T16:00:00+08:00",
  "points":[
    {"model":"gpt-5.6-sol","effort":"max","iq":101.78,"passed":76,"valid_tasks":112},
    {"model":"deepseek-v4-flash","effort":"max","iq":87.05,"passed":65,"valid_tasks":112},
    {"model":"deepseek-v4-flash","effort":"high","iq":65.62,"passed":49,"valid_tasks":112}
  ],
  "history":[
    {"at":"2026-07-11T12:00:00+08:00","points":[
      {"model":"deepseek-v4-flash","effort":"max","iq":75.0,"passed":54,"valid_tasks":108}
    ]}
  ]
}`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
  "monitored_at":"2026-07-11T15:29:00+08:00",
  "timezone":"Asia/Shanghai",
  "model_iq":{"latest":{"date":"2026-07-11-pm","model":"gpt-5.6-sol","reasoning_effort":"max","score":135}}
}`)),
				Header: make(http.Header),
			}, nil
		})},
		now: func() time.Time { return time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC) },
	}

	snapshot, err := service.fetchSnapshot(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{gptIntelligenceDataURL, gptIntelligenceEfficiencyURL}, requestedURLs)
	require.Equal(t, "gpt-5.6-sol", snapshot.Latest.Model)
	require.Equal(t, "public_json_current+efficiency", snapshot.Metadata.Method)
	require.Len(t, snapshot.Comparisons, 2)
	require.Equal(t, "deepseek_v4_flash_high", snapshot.Comparisons[0].Key)
	require.Equal(t, 65.62, *snapshot.Comparisons[0].Latest.Score)
	require.Equal(t, 49.0, *snapshot.Comparisons[0].Latest.Passed)
	require.Equal(t, "deepseek_v4_flash_max", snapshot.Comparisons[1].Key)
	require.Equal(t, 87.05, *snapshot.Comparisons[1].Latest.Score)
	require.Equal(t, "deepseek-v4-flash", snapshot.Comparisons[1].Latest.Model)
	require.Equal(t, "max", snapshot.Comparisons[1].Latest.ReasoningEffort)
}

func TestGptIntelligenceServiceFetchSnapshot_KeepsBaseSnapshotWhenEfficiencyUnavailable(t *testing.T) {
	service := &GptIntelligenceService{
		client: &http.Client{Transport: gptIntelligenceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == gptIntelligenceEfficiencyURL {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("boom")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
  "monitored_at":"2026-07-11T15:29:00+08:00",
  "timezone":"Asia/Shanghai",
  "model_iq":{"latest":{"date":"2026-07-11-pm","model":"gpt-5.6-sol","reasoning_effort":"max","score":135}}
}`)),
				Header: make(http.Header),
			}, nil
		})},
		now: func() time.Time { return time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC) },
	}

	snapshot, err := service.fetchSnapshot(context.Background())

	require.NoError(t, err)
	require.Equal(t, "gpt-5.6-sol", snapshot.Latest.Model)
	require.Empty(t, snapshot.Comparisons)
	require.Equal(t, "public_json_current", snapshot.Metadata.Method)
}

func TestParseGptIntelligenceJSON_ExtractsPublicModelIqSummary(t *testing.T) {
	collectedAt := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	raw := []byte(`{
  "monitored_at": "2026-07-11T15:29:00+08:00",
  "timezone": "Asia/Shanghai",
  "model_iq": {
    "latest": {"date":"2026-07-11-pm","score":135,"status":"green","passed":9,"tasks":10,"invalid":0,"total_tokens":88099695,"output_tokens":482854,"wall_seconds":2670,"wall_time_human":"44分钟","model":"gpt-5.6-sol","reasoning_effort":"max","cost_usd":68.646241},
    "recent_days": [
      {"date":"2026-07-10-n","score":120,"status":"green","passed":8,"tasks":10},
      {"date":"2026-07-11-pm","score":135,"status":"green","passed":9,"tasks":10}
    ],
    "comparisons": {
      "gpt_56_sol_xhigh": {
        "label":"GPT-5.6 Sol xhigh","model":"gpt-5.6-sol","reasoning_effort":"xhigh",
        "latest":{"date":"2026-07-11-pm","score":120,"status":"green","model":"gpt-5.6-sol","reasoning_effort":"xhigh"},
        "recent_days":[{"date":"2026-07-10-pm_2","score":105,"status":"green"},{"date":"2026-07-11-pm","score":120,"status":"green"}]
      },
      "gpt_56_sol_high": {
        "label":"GPT-5.6 Sol high","model":"gpt-5.6-sol","reasoning_effort":"high",
        "latest":{"date":"2026-07-11-pm","score":90,"status":"yellow"},
        "recent_days":[]
      }
    },
    "quota_radar":{"basis_window_label":"5h","cost_usd":101.725884,"rate":3.2815,"adjusted_delta":31,"updated_at":"2026-07-11T04:38:55Z"}
  }
}`)

	snapshot, err := ParseGptIntelligenceJSON(raw, collectedAt)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Latest)
	require.Equal(t, "2026-07-11T15:29:00+08:00", snapshot.MonitoredAt)
	require.Equal(t, "2026-07-11-pm", snapshot.Latest.Date)
	require.Equal(t, "gpt-5.6-sol", snapshot.Latest.Model)
	require.Equal(t, "max", snapshot.Latest.ReasoningEffort)
	require.NotNil(t, snapshot.Latest.Score)
	require.Equal(t, 135.0, *snapshot.Latest.Score)
	require.NotNil(t, snapshot.Latest.Passed)
	require.Equal(t, 9.0, *snapshot.Latest.Passed)
	require.NotNil(t, snapshot.Latest.Tasks)
	require.Equal(t, 10.0, *snapshot.Latest.Tasks)
	require.NotNil(t, snapshot.Latest.WallSeconds)
	require.Equal(t, 2670.0, *snapshot.Latest.WallSeconds)
	require.NotNil(t, snapshot.Latest.CostUSD)
	require.Equal(t, 68.646241, *snapshot.Latest.CostUSD)
	require.Len(t, snapshot.RecentDays, 2)
	require.Equal(t, "gpt-5.6-sol", snapshot.RecentDays[0].Model)
	require.Equal(t, "max", snapshot.RecentDays[0].ReasoningEffort)
	require.Len(t, snapshot.Comparisons, 2)
	require.Equal(t, "gpt_56_sol_high", snapshot.Comparisons[0].Key)
	require.Equal(t, "gpt-5.6-sol", snapshot.Comparisons[0].Latest.Model)
	require.Equal(t, "high", snapshot.Comparisons[0].Latest.ReasoningEffort)
	require.Equal(t, "xhigh", snapshot.Comparisons[1].RecentDays[0].ReasoningEffort)
	require.NotNil(t, snapshot.QuotaRadar)
	require.Equal(t, 3.2815, *snapshot.QuotaRadar.Rate)
	require.Equal(t, "public_json_current", snapshot.Metadata.Method)
	require.Equal(t, 5, snapshot.Metadata.RunCount)
	require.Equal(t, gptIntelligenceSourceURL, snapshot.Source.URL)
	require.Len(t, snapshot.Templates, 3)
}

func TestParseGptIntelligenceJSON_ReturnsUnavailableWhenModelIqMissing(t *testing.T) {
	_, err := ParseGptIntelligenceJSON([]byte(`{"monitored_at":"2026-07-11T15:29:00+08:00"}`), time.Now())

	require.Error(t, err)
	require.True(t, infraErrorIsServiceUnavailable(err))
}

func TestParseGptIntelligenceJSON_ReturnsUnavailableForMalformedJSON(t *testing.T) {
	_, err := ParseGptIntelligenceJSON([]byte(`{"model_iq":`), time.Now())

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

func TestMergeGptIntelligenceDeepSeekEfficiency_ExtractsDeepSeekSeries(t *testing.T) {
	raw := []byte(`{
  "source_updated_at":"2026-08-06T14:44:46+08:00",
  "points":[
    {"model":"gpt-5.6-sol","effort":"max","iq":101.78,"passed":76,"valid_tasks":112},
    {"model":"deepseek-v4-flash","effort":"max","iq":87.05,"passed":65,"valid_tasks":112},
    {"model":"deepseek-v4-flash","effort":"high","iq":65.62,"passed":49,"valid_tasks":112}
  ],
  "history":[
    {"at":"2026-08-02T02:44:46+08:00","points":[
      {"model":"deepseek-v4-flash","effort":"max","iq":75.0,"passed":54,"valid_tasks":108},
      {"model":"deepseek-v4-flash","effort":"high","iq":51.92,"passed":36,"valid_tasks":104}
    ]}
  ]
}`)
	var efficiency gptIntelligenceEfficiencyPayload
	require.NoError(t, json.Unmarshal(raw, &efficiency))
	snapshot := &GptIntelligenceSnapshot{
		Comparisons: []GptIntelligenceComparison{
			{Key: "gpt_56_sol_max", Model: "gpt-5.6-sol", ReasoningEffort: "max"},
		},
	}

	merged := mergeGptIntelligenceDeepSeekEfficiency(snapshot, &efficiency)

	require.True(t, merged)
	require.Len(t, snapshot.Comparisons, 3)
	require.Equal(t, "deepseek_v4_flash_high", snapshot.Comparisons[0].Key)
	require.Equal(t, "DeepSeek V4 Flash high", snapshot.Comparisons[0].Label)
	require.Equal(t, "deepseek-v4-flash", snapshot.Comparisons[0].Model)
	require.Equal(t, "high", snapshot.Comparisons[0].ReasoningEffort)
	require.Equal(t, 65.62, *snapshot.Comparisons[0].Latest.Score)
	require.Equal(t, 49.0, *snapshot.Comparisons[0].Latest.Passed)
	require.Equal(t, 112.0, *snapshot.Comparisons[0].Latest.Tasks)
	require.Len(t, snapshot.Comparisons[0].RecentDays, 2)
	require.Equal(t, "deepseek_v4_flash_max", snapshot.Comparisons[1].Key)
	require.Equal(t, 87.05, *snapshot.Comparisons[1].Latest.Score)
	require.Len(t, snapshot.Comparisons[1].RecentDays, 2)
	require.Equal(t, "gpt_56_sol_max", snapshot.Comparisons[2].Key)
}

func TestMergeGptIntelligenceDeepSeekEfficiency_SkipsExistingKeys(t *testing.T) {
	raw := []byte(`{
  "source_updated_at":"2026-08-06T14:44:46+08:00",
  "points":[
    {"model":"deepseek-v4-flash","effort":"max","iq":87.05,"passed":65,"valid_tasks":112},
    {"model":"deepseek-v4-flash","effort":"high","iq":65.62,"passed":49,"valid_tasks":112}
  ],
  "history":[]
}`)
	var efficiency gptIntelligenceEfficiencyPayload
	require.NoError(t, json.Unmarshal(raw, &efficiency))
	snapshot := &GptIntelligenceSnapshot{
		Comparisons: []GptIntelligenceComparison{
			{Key: "deepseek_v4_flash_max", Model: "deepseek-v4-flash", ReasoningEffort: "max"},
		},
	}

	merged := mergeGptIntelligenceDeepSeekEfficiency(snapshot, &efficiency)

	require.True(t, merged)
	require.Len(t, snapshot.Comparisons, 2)
	require.Equal(t, "deepseek_v4_flash_high", snapshot.Comparisons[0].Key)
	require.Equal(t, "deepseek_v4_flash_max", snapshot.Comparisons[1].Key)
}

func TestGptIntelligenceDeepSeekHelpers(t *testing.T) {
	require.Equal(t, "deepseek_v4_flash_max", gptIntelligenceComparisonKey("deepseek-v4-flash", "max"))
	require.Equal(t, "DeepSeek V4 Flash max", gptIntelligenceDeepSeekLabel("max"))
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
