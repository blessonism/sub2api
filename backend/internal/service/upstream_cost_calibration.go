package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	UpstreamCostCalibrationAdapterManual = "manual"

	UpstreamCostCalibrationRunStatusRunning = "running"
	UpstreamCostCalibrationRunStatusSuccess = "success"
	UpstreamCostCalibrationRunStatusFailed  = "failed"

	UpstreamCostCalibrationTestStatusSuccess = "success"
	UpstreamCostCalibrationTestStatusFailed  = "failed"
	UpstreamCostCalibrationTestStatusSkipped = "skipped"
)

type UpstreamCostCalibrationTask struct {
	ID              int64                            `json:"id"`
	Name            string                           `json:"name"`
	Enabled         bool                             `json:"enabled"`
	TargetGroupID   int64                            `json:"target_group_id"`
	TargetGroupName string                           `json:"target_group_name,omitempty"`
	Model           string                           `json:"model"`
	AdapterType     string                           `json:"adapter_type"`
	Unit            string                           `json:"unit"`
	TestPrompt      string                           `json:"test_prompt"`
	SampleCount     int                              `json:"sample_count"`
	PriorityStart   int                              `json:"priority_start"`
	PriorityStep    int                              `json:"priority_step"`
	Accounts        []UpstreamCostCalibrationAccount `json:"accounts,omitempty"`
	LatestRun       *UpstreamCostCalibrationRun      `json:"latest_run,omitempty"`
	LastRunID       *int64                           `json:"last_run_id,omitempty"`
	LastRunAt       *time.Time                       `json:"last_run_at,omitempty"`
	CreatedAt       time.Time                        `json:"created_at"`
	UpdatedAt       time.Time                        `json:"updated_at"`
}

type UpstreamCostCalibrationAccount struct {
	AccountID       int64          `json:"account_id"`
	AccountName     string         `json:"account_name,omitempty"`
	Platform        string         `json:"platform,omitempty"`
	CurrentPriority *int           `json:"current_priority,omitempty"`
	AdapterConfig   map[string]any `json:"adapter_config"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
}

type UpstreamCostCalibrationTaskInput struct {
	Name          string                           `json:"name"`
	Enabled       *bool                            `json:"enabled,omitempty"`
	TargetGroupID int64                            `json:"target_group_id"`
	Model         string                           `json:"model"`
	AdapterType   string                           `json:"adapter_type"`
	Unit          string                           `json:"unit"`
	TestPrompt    string                           `json:"test_prompt"`
	SampleCount   int                              `json:"sample_count"`
	PriorityStart int                              `json:"priority_start"`
	PriorityStep  int                              `json:"priority_step"`
	Accounts      []UpstreamCostCalibrationAccount `json:"accounts"`
}

type UpstreamCostCalibrationTaskListFilters struct {
	Enabled       *bool
	TargetGroupID int64
}

type UpstreamCostCalibrationRun struct {
	ID              int64                               `json:"id"`
	TaskID          int64                               `json:"task_id"`
	TargetGroupID   int64                               `json:"target_group_id"`
	Status          string                              `json:"status"`
	TotalAccounts   int                                 `json:"total_accounts"`
	ValidAccounts   int                                 `json:"valid_accounts"`
	InvalidAccounts int                                 `json:"invalid_accounts"`
	SuggestionCount int                                 `json:"suggestion_count"`
	Applied         bool                                `json:"applied"`
	AppliedBy       *int64                              `json:"applied_by,omitempty"`
	AppliedAt       *time.Time                          `json:"applied_at,omitempty"`
	ErrorMessage    string                              `json:"error_message,omitempty"`
	StartedAt       time.Time                           `json:"started_at"`
	FinishedAt      *time.Time                          `json:"finished_at,omitempty"`
	CreatedAt       time.Time                           `json:"created_at"`
	Results         []UpstreamCostCalibrationResult     `json:"results,omitempty"`
	Suggestions     []UpstreamCostCalibrationSuggestion `json:"suggestions,omitempty"`
}

type UpstreamCostCalibrationResult struct {
	ID                int64     `json:"id,omitempty"`
	RunID             int64     `json:"run_id,omitempty"`
	TaskID            int64     `json:"task_id,omitempty"`
	AccountID         int64     `json:"account_id"`
	AccountName       string    `json:"account_name,omitempty"`
	AccountPlatform   string    `json:"account_platform,omitempty"`
	CurrentPriority   *int      `json:"current_priority,omitempty"`
	BeforeBalance     *float64  `json:"before_balance,omitempty"`
	AfterBalance      *float64  `json:"after_balance,omitempty"`
	CostDelta         *float64  `json:"cost_delta,omitempty"`
	Unit              string    `json:"unit"`
	TestStatus        string    `json:"test_status"`
	LatencyMs         *int      `json:"latency_ms,omitempty"`
	Valid             bool      `json:"valid"`
	Rank              *int      `json:"rank,omitempty"`
	SuggestedPriority *int      `json:"suggested_priority,omitempty"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	CreatedAt         time.Time `json:"created_at,omitempty"`
}

type UpstreamCostCalibrationSuggestion struct {
	ID          int64      `json:"id,omitempty"`
	RunID       int64      `json:"run_id,omitempty"`
	TaskID      int64      `json:"task_id,omitempty"`
	AccountID   int64      `json:"account_id"`
	OldPriority *int       `json:"old_priority,omitempty"`
	NewPriority int        `json:"new_priority"`
	Reason      string     `json:"reason"`
	Applied     bool       `json:"applied"`
	AppliedBy   *int64     `json:"applied_by,omitempty"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at,omitempty"`
}

type UpstreamCostCalibrationRunStats struct {
	TotalAccounts   int
	ValidAccounts   int
	InvalidAccounts int
	SuggestionCount int
}

type UpstreamCostCalibrationRepository interface {
	ListTasks(ctx context.Context, params pagination.PaginationParams, filters UpstreamCostCalibrationTaskListFilters) ([]UpstreamCostCalibrationTask, *pagination.PaginationResult, error)
	GetTaskByID(ctx context.Context, id int64) (*UpstreamCostCalibrationTask, error)
	CreateTask(ctx context.Context, task *UpstreamCostCalibrationTask, accounts []UpstreamCostCalibrationAccount) (*UpstreamCostCalibrationTask, error)
	UpdateTask(ctx context.Context, task *UpstreamCostCalibrationTask, accounts []UpstreamCostCalibrationAccount) (*UpstreamCostCalibrationTask, error)
	DeleteTask(ctx context.Context, id int64) error
	BeginRun(ctx context.Context, taskID, targetGroupID int64) (*UpstreamCostCalibrationRun, error)
	FinishRun(ctx context.Context, task UpstreamCostCalibrationTask, runID int64, status string, stats UpstreamCostCalibrationRunStats, results []UpstreamCostCalibrationResult, suggestions []UpstreamCostCalibrationSuggestion, errMessage string) (*UpstreamCostCalibrationRun, error)
	GetRun(ctx context.Context, taskID, runID int64) (*UpstreamCostCalibrationRun, error)
	ListRuns(ctx context.Context, taskID int64, params pagination.PaginationParams) ([]UpstreamCostCalibrationRun, *pagination.PaginationResult, error)
	ApplyRunSuggestions(ctx context.Context, taskID, runID, operatorID int64) (*UpstreamCostCalibrationRun, error)
}

type BalanceSample struct {
	BeforeBalance *float64
	AfterBalance  *float64
	LatencyMs     *int
	ErrorMessage  string
}

type BalanceSampleSet struct {
	Samples      []BalanceSample
	ErrorMessage string
}

type BalanceAdapter interface {
	Type() string
	Sample(ctx context.Context, task UpstreamCostCalibrationTask, account UpstreamCostCalibrationAccount) BalanceSampleSet
}

type ManualBalanceAdapter struct{}

func (ManualBalanceAdapter) Type() string { return UpstreamCostCalibrationAdapterManual }

func (ManualBalanceAdapter) Sample(_ context.Context, task UpstreamCostCalibrationTask, account UpstreamCostCalibrationAccount) BalanceSampleSet {
	if samples, ok := manualBalanceSamples(account.AdapterConfig); ok {
		if task.SampleCount > 0 && len(samples) > task.SampleCount {
			samples = samples[:task.SampleCount]
		}
		return BalanceSampleSet{Samples: samples}
	}
	before, okBefore := adapterConfigFloat(account.AdapterConfig, "before_balance")
	after, okAfter := adapterConfigFloat(account.AdapterConfig, "after_balance")
	latency, _ := adapterConfigInt(account.AdapterConfig, "latency_ms")
	if !okBefore || !okAfter {
		return BalanceSampleSet{ErrorMessage: "manual adapter requires before_balance and after_balance"}
	}
	return BalanceSampleSet{Samples: []BalanceSample{{BeforeBalance: &before, AfterBalance: &after, LatencyMs: latency}}}
}

type UpstreamCostCalibrationService struct {
	repo                 UpstreamCostCalibrationRepository
	adapters             map[string]BalanceAdapter
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewUpstreamCostCalibrationService(repo UpstreamCostCalibrationRepository, adapters []BalanceAdapter) *UpstreamCostCalibrationService {
	adapterMap := make(map[string]BalanceAdapter, len(adapters))
	for _, adapter := range adapters {
		if adapter != nil {
			adapterMap[adapter.Type()] = adapter
		}
	}
	return &UpstreamCostCalibrationService{repo: repo, adapters: adapterMap}
}

func ProvideUpstreamCostCalibrationService(repo UpstreamCostCalibrationRepository, authCacheInvalidator APIKeyAuthCacheInvalidator) *UpstreamCostCalibrationService {
	svc := NewUpstreamCostCalibrationService(repo, []BalanceAdapter{ManualBalanceAdapter{}})
	svc.authCacheInvalidator = authCacheInvalidator
	return svc
}

func (s *UpstreamCostCalibrationService) ListTasks(ctx context.Context, page, pageSize int, filters UpstreamCostCalibrationTaskListFilters) ([]UpstreamCostCalibrationTask, *pagination.PaginationResult, error) {
	return s.repo.ListTasks(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}, filters)
}

func (s *UpstreamCostCalibrationService) GetTask(ctx context.Context, id int64) (*UpstreamCostCalibrationTask, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CALIBRATION_TASK_ID", "invalid calibration task id")
	}
	return s.repo.GetTaskByID(ctx, id)
}

func (s *UpstreamCostCalibrationService) CreateTask(ctx context.Context, input UpstreamCostCalibrationTaskInput) (*UpstreamCostCalibrationTask, error) {
	task, accounts, err := normalizeUpstreamCostCalibrationInput(input, 0)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateTask(ctx, task, accounts)
}

func (s *UpstreamCostCalibrationService) UpdateTask(ctx context.Context, id int64, input UpstreamCostCalibrationTaskInput) (*UpstreamCostCalibrationTask, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CALIBRATION_TASK_ID", "invalid calibration task id")
	}
	task, accounts, err := normalizeUpstreamCostCalibrationInput(input, id)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateTask(ctx, task, accounts)
}

func (s *UpstreamCostCalibrationService) DeleteTask(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("INVALID_CALIBRATION_TASK_ID", "invalid calibration task id")
	}
	return s.repo.DeleteTask(ctx, id)
}

func (s *UpstreamCostCalibrationService) RunTask(ctx context.Context, id int64) (*UpstreamCostCalibrationRun, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(task.Accounts) == 0 {
		return nil, infraerrors.BadRequest("CALIBRATION_TASK_EMPTY_ACCOUNTS", "calibration task requires at least one account")
	}
	run, err := s.repo.BeginRun(ctx, task.ID, task.TargetGroupID)
	if err != nil {
		return nil, err
	}
	adapter := s.adapters[task.AdapterType]
	if adapter == nil {
		errMessage := fmt.Sprintf("unsupported balance adapter: %s", task.AdapterType)
		stats := UpstreamCostCalibrationRunStats{TotalAccounts: len(task.Accounts), InvalidAccounts: len(task.Accounts)}
		return s.repo.FinishRun(ctx, *task, run.ID, UpstreamCostCalibrationRunStatusFailed, stats, nil, nil, errMessage)
	}
	results, suggestions, stats := s.buildRunOutcome(ctx, *task, adapter)
	return s.repo.FinishRun(ctx, *task, run.ID, UpstreamCostCalibrationRunStatusSuccess, stats, results, suggestions, "")
}

func (s *UpstreamCostCalibrationService) GetRun(ctx context.Context, taskID, runID int64) (*UpstreamCostCalibrationRun, error) {
	if taskID <= 0 || runID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CALIBRATION_RUN_ID", "invalid calibration run id")
	}
	return s.repo.GetRun(ctx, taskID, runID)
}

func (s *UpstreamCostCalibrationService) ListRuns(ctx context.Context, taskID int64, page, pageSize int) ([]UpstreamCostCalibrationRun, *pagination.PaginationResult, error) {
	if taskID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_CALIBRATION_TASK_ID", "invalid calibration task id")
	}
	return s.repo.ListRuns(ctx, taskID, pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"})
}

func (s *UpstreamCostCalibrationService) ApplyRunSuggestions(ctx context.Context, taskID, runID, operatorID int64) (*UpstreamCostCalibrationRun, error) {
	if taskID <= 0 || runID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CALIBRATION_RUN_ID", "invalid calibration run id")
	}
	run, err := s.repo.ApplyRunSuggestions(ctx, taskID, runID, operatorID)
	if err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, run.TargetGroupID)
	}
	return run, nil
}

func (s *UpstreamCostCalibrationService) buildRunOutcome(ctx context.Context, task UpstreamCostCalibrationTask, adapter BalanceAdapter) ([]UpstreamCostCalibrationResult, []UpstreamCostCalibrationSuggestion, UpstreamCostCalibrationRunStats) {
	results := make([]UpstreamCostCalibrationResult, 0, len(task.Accounts))
	for _, account := range task.Accounts {
		samples := adapter.Sample(ctx, task, account)
		result := buildCalibrationResult(task, account, samples)
		results = append(results, result)
	}
	results, suggestions := scoreCalibrationResults(task, results)
	stats := UpstreamCostCalibrationRunStats{
		TotalAccounts:   len(results),
		SuggestionCount: len(suggestions),
	}
	for _, result := range results {
		if result.Valid {
			stats.ValidAccounts++
		} else {
			stats.InvalidAccounts++
		}
	}
	return results, suggestions, stats
}

func buildCalibrationResult(task UpstreamCostCalibrationTask, account UpstreamCostCalibrationAccount, sampleSet BalanceSampleSet) UpstreamCostCalibrationResult {
	result := UpstreamCostCalibrationResult{
		TaskID:          task.ID,
		AccountID:       account.AccountID,
		AccountName:     account.AccountName,
		AccountPlatform: account.Platform,
		CurrentPriority: account.CurrentPriority,
		Unit:            task.Unit,
		TestStatus:      UpstreamCostCalibrationTestStatusSuccess,
		ErrorMessage:    strings.TrimSpace(sampleSet.ErrorMessage),
	}
	if result.ErrorMessage != "" {
		result.TestStatus = UpstreamCostCalibrationTestStatusFailed
		return result
	}
	if len(sampleSet.Samples) == 0 {
		result.TestStatus = UpstreamCostCalibrationTestStatusFailed
		result.ErrorMessage = "balance sample is incomplete"
		return result
	}
	validDeltas := make([]float64, 0, len(sampleSet.Samples))
	validLatencies := make([]int, 0, len(sampleSet.Samples))
	errorMessages := []string{}
	for _, sample := range sampleSet.Samples {
		if result.BeforeBalance == nil && sample.BeforeBalance != nil {
			result.BeforeBalance = sample.BeforeBalance
		}
		if result.AfterBalance == nil && sample.AfterBalance != nil {
			result.AfterBalance = sample.AfterBalance
		}
		if result.LatencyMs == nil && sample.LatencyMs != nil {
			result.LatencyMs = sample.LatencyMs
		}
		if sample.ErrorMessage != "" {
			errorMessages = append(errorMessages, strings.TrimSpace(sample.ErrorMessage))
			continue
		}
		if sample.BeforeBalance == nil || sample.AfterBalance == nil {
			errorMessages = append(errorMessages, "balance sample is incomplete")
			continue
		}
		delta := *sample.BeforeBalance - *sample.AfterBalance
		if math.IsNaN(delta) || math.IsInf(delta, 0) {
			errorMessages = append(errorMessages, "cost delta is invalid")
			continue
		}
		if delta < 0 {
			errorMessages = append(errorMessages, "after_balance is greater than before_balance")
			continue
		}
		validDeltas = append(validDeltas, delta)
		if sample.LatencyMs != nil {
			validLatencies = append(validLatencies, *sample.LatencyMs)
		}
	}
	if len(validDeltas) == 0 {
		result.TestStatus = UpstreamCostCalibrationTestStatusFailed
		result.ErrorMessage = strings.Join(uniqueNonEmptyStrings(errorMessages), "; ")
		if result.ErrorMessage == "" {
			result.ErrorMessage = "balance sample is incomplete"
		}
		return result
	}
	if len(errorMessages) > 0 {
		result.TestStatus = UpstreamCostCalibrationTestStatusFailed
		result.ErrorMessage = strings.Join(uniqueNonEmptyStrings(errorMessages), "; ")
		return result
	}
	delta := medianFloat64(validDeltas)
	result.CostDelta = &delta
	if len(validLatencies) > 0 {
		latency := medianInt(validLatencies)
		result.LatencyMs = &latency
	}
	result.Valid = true
	return result
}

func scoreCalibrationResults(task UpstreamCostCalibrationTask, results []UpstreamCostCalibrationResult) ([]UpstreamCostCalibrationResult, []UpstreamCostCalibrationSuggestion) {
	validIndexes := make([]int, 0, len(results))
	for i := range results {
		if results[i].Valid && results[i].CostDelta != nil {
			validIndexes = append(validIndexes, i)
		}
	}
	sort.SliceStable(validIndexes, func(i, j int) bool {
		left := results[validIndexes[i]]
		right := results[validIndexes[j]]
		if *left.CostDelta != *right.CostDelta {
			return *left.CostDelta < *right.CostDelta
		}
		if left.LatencyMs != nil && right.LatencyMs != nil && *left.LatencyMs != *right.LatencyMs {
			return *left.LatencyMs < *right.LatencyMs
		}
		return left.AccountID < right.AccountID
	})
	suggestions := make([]UpstreamCostCalibrationSuggestion, 0, len(validIndexes))
	for order, resultIndex := range validIndexes {
		rank := order + 1
		newPriority := task.PriorityStart + order*task.PriorityStep
		results[resultIndex].Rank = &rank
		results[resultIndex].SuggestedPriority = &newPriority
		if results[resultIndex].CurrentPriority == nil || *results[resultIndex].CurrentPriority != newPriority {
			reason := fmt.Sprintf("rank %d by cost %.8f %s", rank, *results[resultIndex].CostDelta, task.Unit)
			suggestions = append(suggestions, UpstreamCostCalibrationSuggestion{
				TaskID:      task.ID,
				AccountID:   results[resultIndex].AccountID,
				OldPriority: results[resultIndex].CurrentPriority,
				NewPriority: newPriority,
				Reason:      reason,
			})
		}
	}
	return results, suggestions
}

func normalizeUpstreamCostCalibrationInput(input UpstreamCostCalibrationTaskInput, id int64) (*UpstreamCostCalibrationTask, []UpstreamCostCalibrationAccount, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, infraerrors.BadRequest("CALIBRATION_TASK_NAME_REQUIRED", "name is required")
	}
	if input.TargetGroupID <= 0 {
		return nil, nil, infraerrors.BadRequest("CALIBRATION_TARGET_GROUP_REQUIRED", "target_group_id is required")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, nil, infraerrors.BadRequest("CALIBRATION_MODEL_REQUIRED", "model is required")
	}
	adapterType := strings.TrimSpace(input.AdapterType)
	if adapterType == "" {
		adapterType = UpstreamCostCalibrationAdapterManual
	}
	if adapterType != UpstreamCostCalibrationAdapterManual {
		return nil, nil, infraerrors.BadRequest("UNSUPPORTED_CALIBRATION_ADAPTER", "unsupported balance adapter")
	}
	unit := strings.TrimSpace(input.Unit)
	if unit == "" {
		unit = "credit"
	}
	testPrompt := strings.TrimSpace(input.TestPrompt)
	if testPrompt == "" {
		testPrompt = "ping"
	}
	sampleCount := input.SampleCount
	if sampleCount <= 0 {
		sampleCount = 1
	}
	if sampleCount > 10 {
		return nil, nil, infraerrors.BadRequest("INVALID_CALIBRATION_SAMPLE_COUNT", "sample_count must be between 1 and 10")
	}
	priorityStart := input.PriorityStart
	if priorityStart <= 0 {
		priorityStart = 10
	}
	if priorityStart > 10000 {
		return nil, nil, infraerrors.BadRequest("INVALID_CALIBRATION_PRIORITY", "priority_start must be between 1 and 10000")
	}
	priorityStep := input.PriorityStep
	if priorityStep <= 0 {
		priorityStep = 10
	}
	if priorityStep > 10000 {
		return nil, nil, infraerrors.BadRequest("INVALID_CALIBRATION_PRIORITY", "priority_step must be between 1 and 10000")
	}
	if len(input.Accounts) == 0 {
		return nil, nil, infraerrors.BadRequest("CALIBRATION_TASK_EMPTY_ACCOUNTS", "calibration task requires at least one account")
	}
	seen := map[int64]struct{}{}
	accounts := make([]UpstreamCostCalibrationAccount, 0, len(input.Accounts))
	for _, account := range input.Accounts {
		if account.AccountID <= 0 {
			return nil, nil, infraerrors.BadRequest("INVALID_CALIBRATION_ACCOUNT_ID", "account_id must be positive")
		}
		if _, ok := seen[account.AccountID]; ok {
			return nil, nil, infraerrors.BadRequest("DUPLICATE_CALIBRATION_ACCOUNT", "duplicate account in calibration task")
		}
		seen[account.AccountID] = struct{}{}
		if account.AdapterConfig == nil {
			account.AdapterConfig = map[string]any{}
		}
		accounts = append(accounts, account)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return &UpstreamCostCalibrationTask{
		ID:            id,
		Name:          name,
		Enabled:       enabled,
		TargetGroupID: input.TargetGroupID,
		Model:         model,
		AdapterType:   adapterType,
		Unit:          unit,
		TestPrompt:    testPrompt,
		SampleCount:   sampleCount,
		PriorityStart: priorityStart,
		PriorityStep:  priorityStep,
	}, accounts, nil
}

func adapterConfigFloat(config map[string]any, key string) (float64, bool) {
	v, ok := config[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func adapterConfigInt(config map[string]any, key string) (*int, bool) {
	v, ok := adapterConfigFloat(config, key)
	if !ok {
		return nil, false
	}
	i := int(v)
	return &i, true
}

func manualBalanceSamples(config map[string]any) ([]BalanceSample, bool) {
	raw, ok := config["samples"]
	if !ok || raw == nil {
		return nil, false
	}
	var items []map[string]any
	switch val := raw.(type) {
	case []any:
		items = make([]map[string]any, 0, len(val))
		for _, item := range val {
			sampleConfig, ok := item.(map[string]any)
			if !ok {
				continue
			}
			items = append(items, sampleConfig)
		}
	case []map[string]any:
		items = val
	default:
		return nil, false
	}
	samples := make([]BalanceSample, 0, len(items))
	for _, sampleConfig := range items {
		before, okBefore := adapterConfigFloat(sampleConfig, "before_balance")
		after, okAfter := adapterConfigFloat(sampleConfig, "after_balance")
		latency, _ := adapterConfigInt(sampleConfig, "latency_ms")
		if !okBefore || !okAfter {
			samples = append(samples, BalanceSample{LatencyMs: latency, ErrorMessage: "manual adapter requires before_balance and after_balance"})
			continue
		}
		samples = append(samples, BalanceSample{BeforeBalance: &before, AfterBalance: &after, LatencyMs: latency})
	}
	return samples, true
}

func medianFloat64(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func medianInt(values []int) int {
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
