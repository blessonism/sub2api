//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func radarBenchTestBinding(selections ...radarBenchSelection) radarBenchBinding {
	return radarBenchBinding{Enabled: true, InterfaceConfirmed: true, ProductionReadVerified: true,
		Benchmark: "radar-bench-final64-20261004", CatalogVersion: "test-catalog", TaskSetSHA256: strings.Repeat("a", 64),
		ScoreVersion: radarBenchScoreVersion, ScoreMax: 150, TotalTasks: 64, ModelEfforts: selections}
}

func radarBenchTestSummary(b radarBenchBinding, selection radarBenchSelection, score *float64) radarBenchSummary {
	count := 54
	p := radarBenchSummary{Schema: radarBenchSummarySchema, Benchmark: b.Benchmark, CatalogVersion: b.CatalogVersion,
		TaskSetSHA256: b.TaskSetSHA256, ScoreVersion: b.ScoreVersion, ScoreMax: b.ScoreMax, radarBenchSelection: selection,
		ScoringPolicyVersion: radarBenchScoringPolicy,
		radarBenchGrade:      radarBenchGrade{Score: score, Coverage: 53, RequiredTasks: 64, ScoreStatus: "provisional"}, GradedRunsCount: &count, HistoryState: "available"}
	if score == nil {
		p.Coverage = 0
		p.ScoreStatus = "missing_current_result"
		*p.GradedRunsCount = 0
	}
	p.History = &radarBenchHistory{Schema: radarBenchHistorySchema, ScoreVersion: b.ScoreVersion, ScoreMax: b.ScoreMax,
		radarBenchSelection: selection, TimeZone: "UTC", Bucket: "day"}
	if score != nil {
		p.History.UpdatedBucket = "2026-10-13T00:00:00.000Z"
		p.History.Points = []radarBenchHistoryPoint{{At: p.History.UpdatedBucket, radarBenchGrade: p.radarBenchGrade}}
	}
	return p
}

func radarBenchTestService(t *testing.T, b radarBenchBinding, summaries map[radarBenchSelection]radarBenchSummary, now *time.Time) (*GptIntelligenceService, *[]string) {
	t.Helper()
	var mu sync.Mutex
	requests := []string{}
	s := &GptIntelligenceService{now: func() time.Time { return *now }}
	s.client = &http.Client{Transport: gptIntelligenceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests = append(requests, req.URL.String())
		mu.Unlock()
		var payload any
		code := http.StatusOK
		switch {
		case req.URL.String() == radarBenchBindingURL:
			payload = b
		case req.URL.Scheme+"://"+req.URL.Host+req.URL.Path == radarBenchSummaryURL:
			query := req.URL.Query()
			if query.Get("view") != "summary" || query.Get("selection") != "user-selected-v1" {
				t.Error("expected documented public summary query")
			}
			selection := radarBenchSelection{Model: query.Get("model"), Effort: query.Get("effort")}
			var ok bool
			payload, ok = summaries[selection]
			if !ok {
				code = http.StatusServiceUnavailable
			}
		default:
			t.Errorf("unexpected source: %s", req.URL.String())
			code = http.StatusNotFound
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}
	return s, &requests
}

func TestGptIntelligenceRadarBench_DiscoversProvidedModelsAndPreservesZero(t *testing.T) {
	sol := radarBenchSelection{Model: "gpt-6.1-sol", Effort: "max"}
	old := radarBenchSelection{Model: "gpt-5.5", Effort: "high"}
	future := radarBenchSelection{Model: "future-model", Effort: "new-effort"}
	missing := radarBenchSelection{Model: "gpt-6-astra", Effort: "ultra"}
	b := radarBenchTestBinding(sol, old, future, missing, sol)
	b.ActiveSelection = sol
	score, zero := 106.72, 0.0
	summaries := map[radarBenchSelection]radarBenchSummary{sol: radarBenchTestSummary(b, sol, &score), old: radarBenchTestSummary(b, old, &zero), future: radarBenchTestSummary(b, future, &score), missing: radarBenchTestSummary(b, missing, nil)}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s, requests := radarBenchTestService(t, b, summaries, &now)
	snap, err := s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, *requests, 5) // 目录一次，去重后的实际配置四次；没有旧静态源。
	require.Equal(t, "gpt-6.1-sol", snap.Latest.Model)
	require.Equal(t, "2026-10-13", snap.Latest.Date)
	require.NotEqual(t, snap.MonitoredAt, snap.Latest.Date)
	require.Equal(t, "provisional", snap.Latest.Status)
	require.Equal(t, 53, snap.Latest.Coverage)
	require.Equal(t, 64, snap.Latest.RequiredTasks)
	require.Equal(t, 150, snap.Latest.ScoreMax)
	require.Nil(t, snap.Latest.Passed) // 覆盖数不是通过数，旧成本/Token 不挪用。
	require.Nil(t, snap.Latest.CostUSD)
	require.Equal(t, 4, snap.Metadata.RequestedSeries)
	require.Equal(t, 3, snap.Metadata.Series)
	require.Equal(t, "gpt-5.5", snap.Comparisons[0].Model)
	require.Equal(t, zero, *snap.Comparisons[0].Latest.Score)
	require.Equal(t, "future-model", snap.Comparisons[1].Model)
	*snap.Latest.Score = -1
	*snap.Latest.GradedRunsCount = -1
	cached, err := s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, score, *cached.Latest.Score)
	require.Equal(t, 54, *cached.Latest.GradedRunsCount)
	require.Len(t, *requests, 5)
}

func TestGptIntelligenceRadarBench_PartialFailureRetriesAndNoOldFallback(t *testing.T) {
	a := radarBenchSelection{Model: "gpt-6-astra", Effort: "max"}
	b := radarBenchSelection{Model: "gpt-6.1-sol", Effort: "max"}
	binding := radarBenchTestBinding(a, b)
	score := 113.67
	now := time.Now()
	s, requests := radarBenchTestService(t, binding, map[radarBenchSelection]radarBenchSummary{a: radarBenchTestSummary(binding, a, &score)}, &now)
	snap, err := s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, snap.Metadata.FailedSeries)
	require.Equal(t, int64(60), snap.Metadata.CacheTTL)
	require.Equal(t, a.Model, snap.Latest.Model)
	now = now.Add(59 * time.Second)
	_, err = s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, *requests, 3)
	now = now.Add(2 * time.Second)
	_, err = s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, *requests, 6)
	failed, _ := radarBenchTestService(t, binding, nil, &now)
	_, err = failed.GetSnapshot(context.Background())
	require.Error(t, err)
	require.True(t, ErrGptIntelligenceUnavailable.Is(err))
}

func TestGptIntelligenceRadarBench_NoPublishedGradesIsEmpty(t *testing.T) {
	selection := radarBenchSelection{Model: "gpt-6-astra", Effort: "ultra"}
	b := radarBenchTestBinding(selection)
	now := time.Now()
	s, _ := radarBenchTestService(t, b, map[radarBenchSelection]radarBenchSummary{selection: radarBenchTestSummary(b, selection, nil)}, &now)
	snap, err := s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Nil(t, snap.Latest)
	require.Empty(t, snap.Comparisons)
	require.Zero(t, snap.Metadata.FailedSeries)
}

func TestGptIntelligenceRadarBench_RejectsMismatchedSourceAndHistory(t *testing.T) {
	selection := radarBenchSelection{Model: "gpt-6.1-sol", Effort: "max"}
	b := radarBenchTestBinding(selection)
	score := 106.72
	for _, tc := range []struct {
		name   string
		change func(*radarBenchSummary)
	}{
		{"model", func(p *radarBenchSummary) { p.Model = "other-model" }},
		{"catalog", func(p *radarBenchSummary) { p.CatalogVersion = "other-catalog" }},
		{"digest", func(p *radarBenchSummary) { p.TaskSetSHA256 = strings.Repeat("b", 64) }},
		{"coverage", func(p *radarBenchSummary) { p.Coverage = 65 }},
		{"status", func(p *radarBenchSummary) { p.ScoreStatus = "complete" }},
		{"last grade", func(p *radarBenchSummary) { p.History.Points[0].Coverage = 52 }},
		{"non day timestamp", func(p *radarBenchSummary) { p.History.Points[0].At = "2026-10-13T12:00:00Z" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := radarBenchTestSummary(b, selection, &score)
			tc.change(&p)
			require.Error(t, p.validate(&b, selection))
		})
	}
	p := radarBenchTestSummary(b, selection, &score)
	p.HistoryState = "unavailable"
	p.History = nil
	require.NoError(t, p.validate(&b, selection))
	snap := snapshotFromRadarBench(&b, []*radarBenchSummary{&p}, 0, time.Now())
	require.Empty(t, snap.Latest.Date) // 无历史时不以采集时间伪造判分时间。
	require.Empty(t, snap.RecentDays)
}

func TestGptIntelligenceRadarBench_ResponseLimitAndInvalidBinding(t *testing.T) {
	for _, extra := range []int{0, 1} {
		body := `{}` + strings.Repeat(" ", gptIntelligenceMaxSourceBytes-2+extra)
		s := &GptIntelligenceService{client: &http.Client{Transport: gptIntelligenceRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
		var value radarBenchBinding
		err := s.fetchRadarBenchJSON(context.Background(), radarBenchBindingURL, &value)
		if extra == 0 {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, "response exceeds")
		}
	}
	for _, b := range []radarBenchBinding{{}, radarBenchTestBinding(), radarBenchTestBinding(radarBenchSelection{Model: "x"})} {
		_, err := b.selections()
		require.Error(t, err)
	}
}

func TestGptIntelligenceRadarBench_LimitsConcurrentReadsAndHonorsCancellation(t *testing.T) {
	b := radarBenchTestBinding()
	summaries := map[radarBenchSelection]radarBenchSummary{}
	score := 90.0
	for _, model := range []string{"one", "two", "three", "four", "five", "six", "seven"} {
		selection := radarBenchSelection{Model: model, Effort: "max"}
		b.ModelEfforts = append(b.ModelEfforts, selection)
		summaries[selection] = radarBenchTestSummary(b, selection, &score)
	}
	now := time.Now()
	s, requests := radarBenchTestService(t, b, summaries, &now)
	original := s.client.Transport
	var active, peak atomic.Int32
	s.client.Transport = gptIntelligenceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/api/radar-bench-score" {
			count := active.Add(1)
			defer active.Add(-1)
			for current := peak.Load(); count > current && !peak.CompareAndSwap(current, count); current = peak.Load() {
			}
			time.Sleep(3 * time.Millisecond)
		}
		return original.RoundTrip(req)
	})
	snap, err := s.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 7, snap.Metadata.Series)
	require.Equal(t, int32(3), peak.Load())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.cache = nil
	_, err = s.GetSnapshot(ctx)
	require.Error(t, err)
	require.Len(t, *requests, 8)
}

func TestGptIntelligenceRadarBench_HonorsSourceRetryAfter(t *testing.T) {
	now := time.Now()
	selection := radarBenchSelection{Model: "gpt-6.1-sol", Effort: "max"}
	b := radarBenchTestBinding(selection)
	var calls atomic.Int32
	s := &GptIntelligenceService{now: func() time.Time { return now }}
	s.client = &http.Client{Transport: gptIntelligenceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.URL.String() == radarBenchBindingURL {
			raw, err := json.Marshal(b)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
		}
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"600"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	_, err := s.GetSnapshot(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(2), calls.Load())
	now = now.Add(599 * time.Second)
	_, err = s.GetSnapshot(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(2), calls.Load())
	now = now.Add(2 * time.Second)
	_, err = s.GetSnapshot(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(4), calls.Load())
}
