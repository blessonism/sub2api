package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"golang.org/x/sync/errgroup"
)

const (
	radarBenchBindingURL        = "https://codexradar.com/data/radar-bench-binding.json"
	radarBenchSummaryURL        = "https://codexradar.com/api/radar-bench-score"
	radarBenchScoreVersion      = "radar-bench-v2-150"
	radarBenchSummarySchema     = "radar-bench-public-summary-v6-150-details"
	radarBenchSummarySchemaV5   = "radar-bench-public-summary-v5-150-daily-graded-count"
	radarBenchHistorySchema     = "radar-bench-public-history-v1-150-daily"
	radarBenchScoringPolicy     = "radar-bench-v2-last3-per-task-equal-weight-150"
	radarBenchCollectionTimeout = 20 * time.Second
	radarBenchMaxSelections     = 256
)

type radarBenchSelection struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type radarBenchBinding struct {
	Enabled                bool                  `json:"enabled"`
	InterfaceConfirmed     bool                  `json:"interface_confirmed"`
	ProductionReadVerified bool                  `json:"production_read_verified"`
	Benchmark              string                `json:"benchmark"`
	CatalogVersion         string                `json:"catalog_version"`
	TaskSetSHA256          string                `json:"task_set_sha256"`
	ScoreVersion           string                `json:"score_version"`
	ScoreMax               int                   `json:"score_max"`
	TotalTasks             int                   `json:"total_tasks"`
	ModelEfforts           []radarBenchSelection `json:"model_efforts"`
	ActiveSelection        radarBenchSelection   `json:"active_display_selection"`
}

type radarBenchGrade struct {
	Score         *float64 `json:"score"`
	Coverage      int      `json:"coverage"`
	RequiredTasks int      `json:"required_tasks"`
	ScoreStatus   string   `json:"score_status"`
}

type radarBenchHistoryPoint struct {
	At string `json:"at"`
	radarBenchGrade
}

type radarBenchHistory struct {
	Schema       string `json:"schema"`
	ScoreVersion string `json:"score_version"`
	ScoreMax     int    `json:"score_max"`
	radarBenchSelection
	TimeZone      string                   `json:"time_zone"`
	Bucket        string                   `json:"bucket"`
	UpdatedBucket string                   `json:"updated_bucket"`
	Points        []radarBenchHistoryPoint `json:"points"`
}

type radarBenchSummary struct {
	Schema               string `json:"schema"`
	Benchmark            string `json:"benchmark"`
	CatalogVersion       string `json:"catalog_version"`
	TaskSetSHA256        string `json:"task_set_sha256"`
	ScoreVersion         string `json:"score_version"`
	ScoreMax             int    `json:"score_max"`
	ScoringPolicyVersion string `json:"scoring_policy_version"`
	radarBenchSelection
	radarBenchGrade
	GradedRunsCount *int               `json:"graded_runs_count"`
	HistoryState    string             `json:"history_state"`
	History         *radarBenchHistory `json:"history"`
}

// fetchSnapshot 只采集现行公开成绩，不用旧 IQ 快照冒充最新 RadarBench。
func (s *GptIntelligenceService) fetchSnapshot(ctx context.Context) (*GptIntelligenceSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, radarBenchCollectionTimeout)
	defer cancel()
	var binding radarBenchBinding
	if err := s.fetchRadarBenchJSON(ctx, radarBenchBindingURL, &binding); err != nil {
		return nil, ErrGptIntelligenceUnavailable.WithCause(err)
	}
	selections, err := binding.selections()
	if err != nil {
		return nil, ErrGptIntelligenceUnavailable.WithCause(err)
	}

	results := make([]*radarBenchSummary, len(selections))
	errors := make([]error, len(selections))
	var group errgroup.Group
	group.SetLimit(3)
	for i, selection := range selections {
		group.Go(func() error {
			query := url.Values{"model": {selection.Model}, "effort": {selection.Effort}, "view": {"summary"}, "selection": {"user-selected-v1"}}
			var summary radarBenchSummary
			err := s.fetchRadarBenchJSON(ctx, radarBenchSummaryURL+"?"+query.Encode(), &summary)
			if err == nil {
				err = summary.validate(&binding, selection)
			}
			if err != nil {
				errors[i] = err
				if _, limited := err.(*radarBenchRateLimitError); limited {
					cancel()
				}
				return nil
			}
			results[i] = &summary
			return nil
		})
	}
	_ = group.Wait() // 单档失败写入对应槽位，不取消其他可用成绩。
	failed := 0
	var firstError error
	for _, err := range errors {
		if err != nil {
			failed++
			if firstError == nil {
				firstError = err
			}
		}
	}
	if failed == len(selections) {
		return nil, ErrGptIntelligenceUnavailable.WithCause(firstError)
	}
	if failed > 0 {
		logger.LegacyPrintf("service.gpt_intelligence", "radar_bench_partial: failed=%d total=%d first_error=%v", failed, len(selections), firstError)
	}
	snap := snapshotFromRadarBench(&binding, results, failed, s.now())
	s.mu.Lock()
	if delay := s.retryUntil.Sub(s.now()); failed > 0 && delay > time.Minute {
		snap.Metadata.CacheTTL = int64(delay/time.Second) + 1
	}
	s.cache = cloneGptIntelligenceSnapshot(snap)
	s.loaded = s.now()
	s.mu.Unlock()
	return cloneGptIntelligenceSnapshot(snap), nil
}

func (s *GptIntelligenceService) fetchRadarBenchJSON(ctx context.Context, sourceURL string, target any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-gpt-intelligence/1.0")
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusTooManyRequests {
		delay := time.Minute
		if seconds, err := strconv.ParseInt(res.Header.Get("Retry-After"), 10, 32); err == nil && seconds > 60 {
			delay = time.Duration(seconds) * time.Second
		} else if until, err := http.ParseTime(res.Header.Get("Retry-After")); err == nil && until.Sub(s.now()) > delay {
			delay = until.Sub(s.now())
		}
		s.mu.Lock()
		if until := s.now().Add(delay); until.After(s.retryUntil) {
			s.retryUntil = until
		}
		s.mu.Unlock()
		return &radarBenchRateLimitError{}
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("radar bench source returned HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, gptIntelligenceMaxSourceBytes+1))
	if err != nil {
		return err
	}
	if len(body) > gptIntelligenceMaxSourceBytes {
		return fmt.Errorf("radar bench response exceeds %d bytes", gptIntelligenceMaxSourceBytes)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode radar bench: %w", err)
	}
	return nil
}

type radarBenchRateLimitError struct{}

func (*radarBenchRateLimitError) Error() string { return "radar bench source returned HTTP 429" }

func (b *radarBenchBinding) selections() ([]radarBenchSelection, error) {
	digest, err := hex.DecodeString(b.TaskSetSHA256)
	if !b.Enabled || !b.InterfaceConfirmed || !b.ProductionReadVerified || b.Benchmark == "" || b.CatalogVersion == "" || err != nil || len(digest) != 32 || b.ScoreVersion != radarBenchScoreVersion || b.ScoreMax != 150 || b.TotalTasks != 64 || len(b.ModelEfforts) == 0 || len(b.ModelEfforts) > radarBenchMaxSelections {
		return nil, fmt.Errorf("invalid radar bench binding")
	}
	selections := make([]radarBenchSelection, 0, len(b.ModelEfforts))
	seen := make(map[radarBenchSelection]bool, len(b.ModelEfforts))
	for _, selection := range b.ModelEfforts {
		if strings.TrimSpace(selection.Model) == "" || strings.TrimSpace(selection.Effort) == "" {
			return nil, fmt.Errorf("invalid radar bench selection")
		}
		if !seen[selection] {
			seen[selection] = true
			selections = append(selections, selection)
		}
	}
	return selections, nil
}

func (g radarBenchGrade) valid(maxScore float64, tasks int) bool {
	if g.RequiredTasks != tasks || g.Coverage < 0 || g.Coverage > tasks {
		return false
	}
	if g.Coverage == 0 {
		return g.Score == nil && g.ScoreStatus == "missing_current_result"
	}
	status := "provisional"
	if g.Coverage == tasks {
		status = "complete"
	}
	return g.Score != nil && *g.Score >= 0 && *g.Score <= maxScore && g.ScoreStatus == status
}

func (p *radarBenchSummary) validate(b *radarBenchBinding, requested radarBenchSelection) error {
	switch p.Schema {
	case radarBenchSummarySchema, radarBenchSummarySchemaV5:
		// v5/v6 共用评分和 UTC 日桶契约；新增详情指标不进入现有快照。
	default:
		return fmt.Errorf("unsupported radar bench summary schema %q for %s/%s", p.Schema, requested.Model, requested.Effort)
	}
	if p.radarBenchSelection != requested || p.Benchmark != b.Benchmark || p.CatalogVersion != b.CatalogVersion || p.TaskSetSHA256 != b.TaskSetSHA256 || p.ScoreVersion != b.ScoreVersion || p.ScoreMax != b.ScoreMax || p.ScoringPolicyVersion != radarBenchScoringPolicy || !p.valid(float64(b.ScoreMax), b.TotalTasks) || p.GradedRunsCount == nil || *p.GradedRunsCount < 0 {
		return fmt.Errorf("invalid radar bench summary for %s/%s", requested.Model, requested.Effort)
	}
	if p.HistoryState != "available" {
		return nil
	}
	h := p.History
	if h == nil || h.Schema != radarBenchHistorySchema || h.ScoreVersion != b.ScoreVersion || h.ScoreMax != b.ScoreMax || h.radarBenchSelection != requested || h.TimeZone != "UTC" || h.Bucket != "day" {
		return fmt.Errorf("invalid radar bench history")
	}
	if len(h.Points) == 0 {
		if p.Score == nil && h.UpdatedBucket == "" {
			return nil
		}
		return fmt.Errorf("radar bench grade has no matching history")
	}
	var previous time.Time
	for _, point := range h.Points {
		at, err := time.Parse(time.RFC3339, point.At)
		if err != nil || !at.After(previous) || !at.Equal(at.UTC().Truncate(24*time.Hour)) || !point.valid(float64(b.ScoreMax), b.TotalTasks) || point.Score == nil {
			return fmt.Errorf("invalid radar bench grading day")
		}
		previous = at
	}
	last := h.Points[len(h.Points)-1]
	updated, err := time.Parse(time.RFC3339, h.UpdatedBucket)
	if err != nil || !updated.Equal(previous) || p.Score == nil || *last.Score != *p.Score || last.Coverage != p.Coverage || last.ScoreStatus != p.ScoreStatus {
		return fmt.Errorf("radar bench history differs from current grade")
	}
	return nil
}

func snapshotFromRadarBench(binding *radarBenchBinding, summaries []*radarBenchSummary, failed int, collectedAt time.Time) *GptIntelligenceSnapshot {
	ttl := gptIntelligenceCacheTTL
	if failed > 0 {
		ttl = time.Minute
	}
	snap := &GptIntelligenceSnapshot{
		Benchmark: binding.Benchmark, MonitoredAt: collectedAt.UTC().Format(time.RFC3339), Timezone: "UTC",
		Comparisons: []GptIntelligenceComparison{}, RecentDays: []GptIntelligenceRun{}, Templates: DefaultGptIntelligencePromptTemplates(),
		Source:   GptIntelligenceSource{Name: "Codex 雷达 · 雷达Bench", URL: gptIntelligenceSourceURL},
		Metadata: GptIntelligenceCollectionMetadata{Method: "public_radar_bench", CachedAt: collectedAt.UTC().Format(time.RFC3339), CacheTTL: int64(ttl / time.Second), RequestedSeries: len(summaries), FailedSeries: failed, Attribution: "数据来自 Codex 雷达 codexradar.com；雷达Bench 满分 150，日期为 UTC 判分日"},
	}
	series := make([]GptIntelligenceComparison, 0, len(summaries))
	primaryIndex := 0
	for _, summary := range summaries {
		if summary == nil || summary.Score == nil {
			continue
		}
		runs := []GptIntelligenceRun{}
		if summary.HistoryState == "available" && summary.History != nil {
			for _, point := range summary.History.Points {
				at, _ := time.Parse(time.RFC3339, point.At) // validate 已检查 UTC 日桶。
				runs = append(runs, radarBenchRun(summary.radarBenchSelection, point.radarBenchGrade, summary.ScoreMax, at.UTC().Format(time.DateOnly)))
			}
		}
		day := ""
		if len(runs) > 0 {
			day = runs[len(runs)-1].Date
		}
		latest := radarBenchRun(summary.radarBenchSelection, summary.radarBenchGrade, summary.ScoreMax, day)
		latest.GradedRunsCount = summary.GradedRunsCount
		key, _ := json.Marshal(summary.radarBenchSelection)
		if summary.radarBenchSelection == binding.ActiveSelection {
			primaryIndex = len(series)
		}
		series = append(series, GptIntelligenceComparison{Key: string(key), Label: gptIntelligenceModelLabel(summary.Model, summary.Effort), Model: summary.Model, ReasoningEffort: summary.Effort, Latest: &latest, RecentDays: runs})
		snap.Metadata.RunCount += len(runs)
	}
	snap.Metadata.Series = len(series)
	if len(series) > 0 {
		snap.Latest = series[primaryIndex].Latest
		snap.RecentDays = series[primaryIndex].RecentDays
		snap.Comparisons = append(series[:primaryIndex], series[primaryIndex+1:]...)
	}
	return snap
}

func radarBenchRun(selection radarBenchSelection, grade radarBenchGrade, maxScore int, day string) GptIntelligenceRun {
	return GptIntelligenceRun{Date: day, Score: grade.Score, Status: grade.ScoreStatus, Model: selection.Model, ReasoningEffort: selection.Effort, ScoreMax: maxScore, Coverage: grade.Coverage, RequiredTasks: grade.RequiredTasks}
}
