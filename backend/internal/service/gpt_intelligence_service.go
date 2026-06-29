package service

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	gptIntelligenceSourceURL      = "https://codexradar.com/"
	gptIntelligenceSourceLabel    = "Codex 雷达"
	gptIntelligenceFetchTimeout   = 8 * time.Second
	gptIntelligenceCacheTTL       = time.Hour
	gptIntelligenceMaxSourceBytes = 512 * 1024
)

var (
	ErrGptIntelligenceUnavailable = infraerrors.ServiceUnavailable(
		"GPT_INTELLIGENCE_UNAVAILABLE",
		"GPT intelligence data is currently unavailable",
	)

	gptIntelligenceTitleRe = regexp.MustCompile(`<title>\s*([^<]*IQ指数[^<]*)\s*</title>`)
	gptIntelligenceRunRe   = regexp.MustCompile(`^([0-9]{1,2}\.[0-9]{1,2}(?:_(?:am|pm))?)\s+(.+?)\s+(xhigh|high|medium|low):\s*IQ指数\s*([0-9]+(?:\.[0-9]+)?),\s*([0-9]+)/([0-9]+),\s*费用\s*\$([0-9]+(?:\.[0-9]+)?),\s*耗时\s*([0-9]+)分钟`)
)

// GptIntelligenceService 从公开页面采集 GPT 智力检测数据，并归一化为前端快照结构。
type GptIntelligenceService struct {
	client *http.Client
	now    func() time.Time

	mu     sync.RWMutex
	cache  *GptIntelligenceSnapshot
	loaded time.Time
	sf     singleflight.Group
}

func NewGptIntelligenceService() *GptIntelligenceService {
	return &GptIntelligenceService{
		client: newGptIntelligenceHTTPClient(),
		now:    time.Now,
	}
}

func newGptIntelligenceHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Timeout:   gptIntelligenceFetchTimeout,
		Transport: transport,
	}
}

type GptIntelligenceSnapshot struct {
	MonitoredAt string                            `json:"monitored_at"`
	Timezone    string                            `json:"timezone"`
	Latest      *GptIntelligenceRun               `json:"latest"`
	RecentDays  []GptIntelligenceRun              `json:"recent_days"`
	Comparisons []GptIntelligenceComparison       `json:"comparisons"`
	QuotaRadar  *GptIntelligenceQuotaRadar        `json:"quota_radar"`
	Source      GptIntelligenceSource             `json:"source"`
	Metadata    GptIntelligenceCollectionMetadata `json:"metadata"`
}

type GptIntelligenceRun struct {
	Date            string   `json:"date"`
	Score           *float64 `json:"score"`
	Status          string   `json:"status"`
	Passed          *float64 `json:"passed"`
	Tasks           *float64 `json:"tasks"`
	Invalid         *float64 `json:"invalid"`
	TotalTokens     *float64 `json:"total_tokens"`
	OutputTokens    *float64 `json:"output_tokens"`
	WallSeconds     *float64 `json:"wall_seconds"`
	WallTimeHuman   string   `json:"wall_time_human"`
	Model           string   `json:"model"`
	ReasoningEffort string   `json:"reasoning_effort"`
	CostUSD         *float64 `json:"cost_usd"`
}

type GptIntelligenceComparison struct {
	Key             string               `json:"key"`
	Label           string               `json:"label"`
	Model           string               `json:"model"`
	ReasoningEffort string               `json:"reasoning_effort"`
	Latest          *GptIntelligenceRun  `json:"latest"`
	RecentDays      []GptIntelligenceRun `json:"recent_days"`
}

type GptIntelligenceQuotaRadar struct {
	BasisWindowLabel string   `json:"basis_window_label"`
	CostUSD          *float64 `json:"cost_usd"`
	Rate             *float64 `json:"rate"`
	AdjustedDelta    *float64 `json:"adjusted_delta"`
	UpdatedAt        string   `json:"updated_at"`
}

type GptIntelligenceSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GptIntelligenceCollectionMetadata struct {
	Method      string `json:"method"`
	CachedAt    string `json:"cached_at"`
	CacheTTL    int64  `json:"cache_ttl_seconds"`
	RunCount    int    `json:"run_count"`
	Series      int    `json:"series"`
	Attribution string `json:"attribution"`
}

// GetSnapshot 返回缓存快照，缓存过期时重新拉取公开页面。
func (s *GptIntelligenceService) GetSnapshot(ctx context.Context) (*GptIntelligenceSnapshot, error) {
	if s == nil {
		return nil, ErrGptIntelligenceUnavailable
	}
	if snap := s.cachedSnapshot(); snap != nil {
		return snap, nil
	}

	value, err, _ := s.sf.Do("snapshot", func() (any, error) {
		if snap := s.cachedSnapshot(); snap != nil {
			return snap, nil
		}
		return s.fetchSnapshot(ctx)
	})
	if err != nil {
		return nil, err
	}
	snap, ok := value.(*GptIntelligenceSnapshot)
	if !ok || snap == nil {
		return nil, ErrGptIntelligenceUnavailable
	}
	return snap, nil
}

func (s *GptIntelligenceService) cachedSnapshot() *GptIntelligenceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cache == nil || s.now().Sub(s.loaded) >= gptIntelligenceCacheTTL {
		return nil
	}
	return cloneGptIntelligenceSnapshot(s.cache)
}

func (s *GptIntelligenceService) fetchSnapshot(ctx context.Context) (*GptIntelligenceSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gptIntelligenceSourceURL, nil)
	if err != nil {
		return nil, ErrGptIntelligenceUnavailable.WithCause(err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "sub2api-gpt-intelligence/1.0")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, ErrGptIntelligenceUnavailable.WithCause(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrGptIntelligenceUnavailable.WithCause(fmt.Errorf("source returned HTTP %d", res.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, gptIntelligenceMaxSourceBytes))
	if err != nil {
		return nil, ErrGptIntelligenceUnavailable.WithCause(err)
	}

	snap, err := ParseGptIntelligenceHTML(string(body), s.now())
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache = cloneGptIntelligenceSnapshot(snap)
	s.loaded = s.now()
	s.mu.Unlock()

	return cloneGptIntelligenceSnapshot(snap), nil
}

// ParseGptIntelligenceHTML 从公开首页 HTML 的 IQ 图表 title 文本中提取快照。
func ParseGptIntelligenceHTML(rawHTML string, collectedAt time.Time) (*GptIntelligenceSnapshot, error) {
	matches := gptIntelligenceTitleRe.FindAllStringSubmatch(rawHTML, -1)
	if len(matches) == 0 {
		return nil, ErrGptIntelligenceUnavailable.WithCause(fmt.Errorf("model IQ titles not found"))
	}

	seriesByKey := make(map[string][]GptIntelligenceRun)
	seriesOrder := make([]string, 0)
	seen := make(map[string]struct{})
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		run, ok := parseGptIntelligenceTitle(match[1], collectedAt)
		if !ok {
			continue
		}
		key := gptIntelligenceSeriesKey(run.Model, run.ReasoningEffort)
		if _, exists := seriesByKey[key]; !exists {
			seriesOrder = append(seriesOrder, key)
		}
		dedupeKey := key + "|" + run.Date
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}
		seriesByKey[key] = append(seriesByKey[key], run)
	}
	if len(seriesByKey) == 0 {
		return nil, ErrGptIntelligenceUnavailable.WithCause(fmt.Errorf("model IQ titles could not be parsed"))
	}

	keys := make([]string, 0, len(seriesByKey))
	for key := range seriesByKey {
		sortGptIntelligenceRuns(seriesByKey[key])
	}
	keys = stableGptIntelligenceSeriesOrder(seriesOrder, seriesByKey)

	primaryKey := choosePrimaryGptIntelligenceSeries(keys, seriesByKey)
	primaryRuns := seriesByKey[primaryKey]
	latest := latestGptIntelligenceRun(primaryRuns)

	comparisons := make([]GptIntelligenceComparison, 0, len(keys)-1)
	for _, key := range keys {
		if key == primaryKey {
			continue
		}
		runs := seriesByKey[key]
		if len(runs) == 0 {
			continue
		}
		latestRun := latestGptIntelligenceRun(runs)
		comparisons = append(comparisons, GptIntelligenceComparison{
			Key:             key,
			Label:           buildGptIntelligenceSeriesLabel(latestRun.Model, latestRun.ReasoningEffort),
			Model:           latestRun.Model,
			ReasoningEffort: latestRun.ReasoningEffort,
			Latest:          cloneGptIntelligenceRunPtr(&latestRun),
			RecentDays:      cloneGptIntelligenceRuns(runs),
		})
	}

	now := collectedAt.UTC()
	snap := &GptIntelligenceSnapshot{
		MonitoredAt: now.Format(time.RFC3339),
		Timezone:    "Asia/Shanghai",
		Latest:      cloneGptIntelligenceRunPtr(&latest),
		RecentDays:  cloneGptIntelligenceRuns(primaryRuns),
		Comparisons: comparisons,
		QuotaRadar:  nil,
		Source: GptIntelligenceSource{
			Name: gptIntelligenceSourceLabel,
			URL:  gptIntelligenceSourceURL,
		},
		Metadata: GptIntelligenceCollectionMetadata{
			Method:      "public_html_title",
			CachedAt:    now.Format(time.RFC3339),
			CacheTTL:    int64(gptIntelligenceCacheTTL / time.Second),
			RunCount:    len(seen),
			Series:      len(seriesByKey),
			Attribution: "数据来自 Codex 雷达 codexradar.com 公开页面",
		},
	}
	return snap, nil
}

func parseGptIntelligenceTitle(raw string, collectedAt time.Time) (GptIntelligenceRun, bool) {
	title := strings.TrimSpace(html.UnescapeString(raw))
	match := gptIntelligenceRunRe.FindStringSubmatch(title)
	if len(match) != 9 {
		return GptIntelligenceRun{}, false
	}

	score, err := strconv.ParseFloat(match[4], 64)
	if err != nil {
		return GptIntelligenceRun{}, false
	}
	passed, err := strconv.ParseFloat(match[5], 64)
	if err != nil {
		return GptIntelligenceRun{}, false
	}
	tasks, err := strconv.ParseFloat(match[6], 64)
	if err != nil {
		return GptIntelligenceRun{}, false
	}
	cost, err := strconv.ParseFloat(match[7], 64)
	if err != nil {
		return GptIntelligenceRun{}, false
	}
	minutes, err := strconv.ParseFloat(match[8], 64)
	if err != nil {
		return GptIntelligenceRun{}, false
	}

	date := normalizeGptIntelligenceDate(match[1], collectedAt)
	model := strings.TrimSpace(match[2])
	effort := strings.TrimSpace(match[3])
	wallSeconds := minutes * 60
	return GptIntelligenceRun{
		Date:            date,
		Score:           float64Ptr(score),
		Status:          statusFromGptIntelligenceScore(score),
		Passed:          float64Ptr(passed),
		Tasks:           float64Ptr(tasks),
		Invalid:         nil,
		TotalTokens:     nil,
		OutputTokens:    nil,
		WallSeconds:     float64Ptr(wallSeconds),
		WallTimeHuman:   fmt.Sprintf("%.0f分钟", minutes),
		Model:           model,
		ReasoningEffort: effort,
		CostUSD:         float64Ptr(cost),
	}, true
}

func normalizeGptIntelligenceDate(raw string, now time.Time) string {
	parts := strings.Split(raw, "_")
	md := strings.Split(parts[0], ".")
	if len(md) != 2 {
		return raw
	}
	month, errM := strconv.Atoi(md[0])
	day, errD := strconv.Atoi(md[1])
	if errM != nil || errD != nil {
		return raw
	}
	year := now.In(time.FixedZone("CST", 8*60*60)).Year()
	date := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if len(parts) > 1 && parts[1] != "" {
		date += "-" + parts[1]
	}
	return date
}

func statusFromGptIntelligenceScore(score float64) string {
	switch {
	case score >= 100:
		return "green"
	case score >= 75:
		return "yellow"
	default:
		return "red"
	}
}

func choosePrimaryGptIntelligenceSeries(keys []string, seriesByKey map[string][]GptIntelligenceRun) string {
	for _, preferred := range []string{"gpt_55_xhigh", "gpt_5_5_xhigh"} {
		for _, key := range keys {
			if key == preferred {
				return key
			}
		}
	}
	bestKey := keys[0]
	bestRuns := len(seriesByKey[bestKey])
	for _, key := range keys[1:] {
		if count := len(seriesByKey[key]); count > bestRuns {
			bestKey = key
			bestRuns = count
		}
	}
	return bestKey
}

func stableGptIntelligenceSeriesOrder(order []string, seriesByKey map[string][]GptIntelligenceRun) []string {
	keys := make([]string, 0, len(seriesByKey))
	seen := make(map[string]struct{}, len(seriesByKey))
	for _, key := range order {
		if _, ok := seriesByKey[key]; !ok {
			continue
		}
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	var missing []string
	for key := range seriesByKey {
		if _, ok := seen[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return append(keys, missing...)
}

func gptIntelligenceSeriesKey(model, effort string) string {
	key := strings.ToLower(model)
	key = strings.ReplaceAll(key, ".", "")
	key = strings.NewReplacer("-", "_", " ", "_", "/", "_").Replace(key)
	key = strings.Trim(key, "_")
	effort = strings.ToLower(strings.TrimSpace(effort))
	if key == "" {
		key = "model"
	}
	if effort == "" {
		return key
	}
	return key + "_" + effort
}

func buildGptIntelligenceSeriesLabel(model, effort string) string {
	if strings.TrimSpace(effort) == "" {
		return strings.TrimSpace(model)
	}
	return strings.TrimSpace(model) + " " + strings.TrimSpace(effort)
}

func sortGptIntelligenceRuns(runs []GptIntelligenceRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		return gptIntelligenceDateOrder(runs[i].Date) < gptIntelligenceDateOrder(runs[j].Date)
	})
}

func gptIntelligenceDateOrder(value string) int {
	matched := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})(?:-(am|pm))?$`).FindStringSubmatch(value)
	if len(matched) == 0 {
		return int(^uint(0) >> 1)
	}
	year, _ := strconv.Atoi(matched[1])
	month, _ := strconv.Atoi(matched[2])
	day, _ := strconv.Atoi(matched[3])
	halfOrder := 0
	if len(matched) > 4 && matched[4] == "pm" {
		halfOrder = 1
	}
	return year*100000 + month*1000 + day*10 + halfOrder
}

func latestGptIntelligenceRun(runs []GptIntelligenceRun) GptIntelligenceRun {
	if len(runs) == 0 {
		return GptIntelligenceRun{}
	}
	return runs[len(runs)-1]
}

func cloneGptIntelligenceSnapshot(snap *GptIntelligenceSnapshot) *GptIntelligenceSnapshot {
	if snap == nil {
		return nil
	}
	out := *snap
	out.Latest = cloneGptIntelligenceRunPtr(snap.Latest)
	out.RecentDays = cloneGptIntelligenceRuns(snap.RecentDays)
	out.Comparisons = make([]GptIntelligenceComparison, len(snap.Comparisons))
	for i, comparison := range snap.Comparisons {
		out.Comparisons[i] = comparison
		out.Comparisons[i].Latest = cloneGptIntelligenceRunPtr(comparison.Latest)
		out.Comparisons[i].RecentDays = cloneGptIntelligenceRuns(comparison.RecentDays)
	}
	if snap.QuotaRadar != nil {
		q := *snap.QuotaRadar
		out.QuotaRadar = &q
	}
	return &out
}

func cloneGptIntelligenceRuns(runs []GptIntelligenceRun) []GptIntelligenceRun {
	if runs == nil {
		return nil
	}
	out := make([]GptIntelligenceRun, len(runs))
	for i := range runs {
		out[i] = *cloneGptIntelligenceRunPtr(&runs[i])
	}
	return out
}

func cloneGptIntelligenceRunPtr(run *GptIntelligenceRun) *GptIntelligenceRun {
	if run == nil {
		return nil
	}
	out := *run
	out.Score = cloneFloat64Ptr(run.Score)
	out.Passed = cloneFloat64Ptr(run.Passed)
	out.Tasks = cloneFloat64Ptr(run.Tasks)
	out.Invalid = cloneFloat64Ptr(run.Invalid)
	out.TotalTokens = cloneFloat64Ptr(run.TotalTokens)
	out.OutputTokens = cloneFloat64Ptr(run.OutputTokens)
	out.WallSeconds = cloneFloat64Ptr(run.WallSeconds)
	out.CostUSD = cloneFloat64Ptr(run.CostUSD)
	return &out
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
