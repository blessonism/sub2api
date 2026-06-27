package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestManualBalanceAdapterSamplesUseTaskSampleCount(t *testing.T) {
	task := UpstreamCostCalibrationTask{
		ID:            1,
		Unit:          "credit",
		SampleCount:   3,
		PriorityStart: 10,
		PriorityStep:  10,
	}
	account := UpstreamCostCalibrationAccount{
		AccountID: 7,
		AdapterConfig: map[string]any{
			"samples": []map[string]any{
				{"before_balance": 10.0, "after_balance": 9.5, "latency_ms": 100}, // 0.5
				{"before_balance": 10.0, "after_balance": 9.8, "latency_ms": 80},  // 0.2
				{"before_balance": 10.0, "after_balance": 9.2, "latency_ms": 160}, // 0.8
				{"before_balance": 10.0, "after_balance": 1.0, "latency_ms": 10},  // 被 sample_count 截断
			},
		},
	}

	samples := ManualBalanceAdapter{}.Sample(context.Background(), task, account)
	result := buildCalibrationResult(task, account, samples)

	require.True(t, result.Valid)
	require.NotNil(t, result.CostDelta)
	require.InDelta(t, 0.5, *result.CostDelta, 0.0000001)
	require.NotNil(t, result.LatencyMs)
	require.Equal(t, 100, *result.LatencyMs)
}

func TestManualBalanceAdapterFallsBackToAccountExtra(t *testing.T) {
	task := UpstreamCostCalibrationTask{ID: 1, Unit: "credit", SampleCount: 1}
	account := UpstreamCostCalibrationAccount{
		AccountID:    7,
		AccountExtra: map[string]any{"before_balance": 12.0, "after_balance": 11.25, "latency_ms": 90},
	}

	samples := ManualBalanceAdapter{}.Sample(context.Background(), task, account)
	result := buildCalibrationResult(task, account, samples)

	require.True(t, result.Valid)
	require.NotNil(t, result.CostDelta)
	require.InDelta(t, 0.75, *result.CostDelta, 0.0000001)
	require.Equal(t, 90, *result.LatencyMs)
}

func TestManualBalanceAdapterConfigOverridesAccountExtra(t *testing.T) {
	task := UpstreamCostCalibrationTask{ID: 1, Unit: "credit", SampleCount: 1}
	account := UpstreamCostCalibrationAccount{
		AccountID:     7,
		AccountExtra:  map[string]any{"before_balance": 12.0, "after_balance": 11.25},
		AdapterConfig: map[string]any{"before_balance": 8.0, "after_balance": 7.5},
	}

	samples := ManualBalanceAdapter{}.Sample(context.Background(), task, account)
	result := buildCalibrationResult(task, account, samples)

	require.True(t, result.Valid)
	require.NotNil(t, result.CostDelta)
	require.InDelta(t, 0.5, *result.CostDelta, 0.0000001)
}

func TestScoreCalibrationResultsFiltersInvalidAndRanksCheapest(t *testing.T) {
	priority := 20
	cheap := 0.1
	expensive := 0.4
	invalidCost := -0.2
	results, suggestions := scoreCalibrationResults(
		UpstreamCostCalibrationTask{ID: 1, Unit: "credit", PriorityStart: 10, PriorityStep: 5},
		[]UpstreamCostCalibrationResult{
			{AccountID: 1, CurrentPriority: &priority, CostDelta: &expensive, Valid: true},
			{AccountID: 2, CostDelta: &invalidCost, Valid: false},
			{AccountID: 3, CostDelta: &cheap, Valid: true},
		},
	)

	require.Len(t, suggestions, 2)
	require.Equal(t, int64(3), suggestions[0].AccountID)
	require.Equal(t, 10, suggestions[0].NewPriority)
	require.Equal(t, int64(1), suggestions[1].AccountID)
	require.Equal(t, 15, suggestions[1].NewPriority)
	require.Equal(t, 2, *results[0].Rank)
	require.Nil(t, results[1].Rank)
	require.Equal(t, 1, *results[2].Rank)
}

func TestBuildCalibrationResultRejectsOnlyInvalidSamples(t *testing.T) {
	task := UpstreamCostCalibrationTask{ID: 1, Unit: "credit"}
	before := 9.0
	after := 10.0

	result := buildCalibrationResult(task, UpstreamCostCalibrationAccount{AccountID: 1}, BalanceSampleSet{
		Samples: []BalanceSample{{BeforeBalance: &before, AfterBalance: &after}},
	})

	require.False(t, result.Valid)
	require.Equal(t, UpstreamCostCalibrationTestStatusFailed, result.TestStatus)
	require.Contains(t, result.ErrorMessage, "after_balance is greater than before_balance")
}

func TestBuildCalibrationResultRejectsMixedInvalidSamples(t *testing.T) {
	task := UpstreamCostCalibrationTask{ID: 1, Unit: "credit"}
	beforeOK := 10.0
	afterOK := 9.8
	beforeBad := 10.0
	afterBad := 10.2

	result := buildCalibrationResult(task, UpstreamCostCalibrationAccount{AccountID: 1}, BalanceSampleSet{
		Samples: []BalanceSample{
			{BeforeBalance: &beforeOK, AfterBalance: &afterOK},
			{BeforeBalance: &beforeBad, AfterBalance: &afterBad},
		},
	})

	require.False(t, result.Valid)
	require.Nil(t, result.CostDelta)
	require.Nil(t, result.Rank)
	require.Equal(t, UpstreamCostCalibrationTestStatusFailed, result.TestStatus)
	require.Contains(t, result.ErrorMessage, "after_balance is greater than before_balance")
}

func TestApplyRunSuggestionsInvalidatesRunSnapshotGroup(t *testing.T) {
	repo := &upstreamCostCalibrationFakeRepo{
		task: UpstreamCostCalibrationTask{ID: 9, TargetGroupID: 8},
		run:  UpstreamCostCalibrationRun{ID: 12, TaskID: 9, TargetGroupID: 7, Status: UpstreamCostCalibrationRunStatusSuccess},
	}
	invalidator := &upstreamCostCalibrationCacheInvalidator{}
	svc := NewUpstreamCostCalibrationService(repo, nil)
	svc.authCacheInvalidator = invalidator

	run, err := svc.ApplyRunSuggestions(context.Background(), 9, 12, 66)

	require.NoError(t, err)
	require.Equal(t, int64(7), run.TargetGroupID)
	require.Equal(t, int64(66), repo.operatorID)
	require.Equal(t, []int64{7}, invalidator.groupIDs)
}

type upstreamCostCalibrationFakeRepo struct {
	task       UpstreamCostCalibrationTask
	run        UpstreamCostCalibrationRun
	operatorID int64
}

func (r *upstreamCostCalibrationFakeRepo) ListTasks(context.Context, pagination.PaginationParams, UpstreamCostCalibrationTaskListFilters) ([]UpstreamCostCalibrationTask, *pagination.PaginationResult, error) {
	return []UpstreamCostCalibrationTask{r.task}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamCostCalibrationFakeRepo) GetTaskByID(context.Context, int64) (*UpstreamCostCalibrationTask, error) {
	task := r.task
	return &task, nil
}

func (r *upstreamCostCalibrationFakeRepo) CreateTask(context.Context, *UpstreamCostCalibrationTask, []UpstreamCostCalibrationAccount) (*UpstreamCostCalibrationTask, error) {
	return nil, nil
}

func (r *upstreamCostCalibrationFakeRepo) UpdateTask(context.Context, *UpstreamCostCalibrationTask, []UpstreamCostCalibrationAccount) (*UpstreamCostCalibrationTask, error) {
	return nil, nil
}

func (r *upstreamCostCalibrationFakeRepo) DeleteTask(context.Context, int64) error {
	return nil
}

func (r *upstreamCostCalibrationFakeRepo) BeginRun(context.Context, int64, int64) (*UpstreamCostCalibrationRun, error) {
	run := r.run
	return &run, nil
}

func (r *upstreamCostCalibrationFakeRepo) FinishRun(context.Context, UpstreamCostCalibrationTask, int64, string, UpstreamCostCalibrationRunStats, []UpstreamCostCalibrationResult, []UpstreamCostCalibrationSuggestion, string) (*UpstreamCostCalibrationRun, error) {
	run := r.run
	return &run, nil
}

func (r *upstreamCostCalibrationFakeRepo) GetRun(context.Context, int64, int64) (*UpstreamCostCalibrationRun, error) {
	run := r.run
	return &run, nil
}

func (r *upstreamCostCalibrationFakeRepo) ListRuns(context.Context, int64, pagination.PaginationParams) ([]UpstreamCostCalibrationRun, *pagination.PaginationResult, error) {
	return []UpstreamCostCalibrationRun{r.run}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamCostCalibrationFakeRepo) ApplyRunSuggestions(_ context.Context, _, _ int64, operatorID int64) (*UpstreamCostCalibrationRun, error) {
	r.operatorID = operatorID
	run := r.run
	return &run, nil
}

type upstreamCostCalibrationCacheInvalidator struct {
	groupIDs []int64
}

func (i *upstreamCostCalibrationCacheInvalidator) InvalidateAuthCacheByKey(context.Context, string) {}

func (i *upstreamCostCalibrationCacheInvalidator) InvalidateAuthCacheByUserID(context.Context, int64) {
}

func (i *upstreamCostCalibrationCacheInvalidator) InvalidateAuthCacheByGroupID(_ context.Context, groupID int64) {
	i.groupIDs = append(i.groupIDs, groupID)
}
