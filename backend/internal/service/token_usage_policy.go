package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	TokenUsagePolicyActionRateOnly          = "rate_only"
	TokenUsagePolicyActionGrantGroupAndRate = "grant_group_and_rate"

	TokenUsagePolicyConflictManualPriority = "manual_priority"
	TokenUsagePolicyConflictAutoPriority   = "auto_priority"

	TokenUsagePolicyFrequencyEvery6h = "every_6h"
	TokenUsagePolicyFrequencyDaily   = "daily"
	TokenUsagePolicyFrequencyWeekly  = "weekly"

	TokenUsagePolicyRunTypePreview   = "preview"
	TokenUsagePolicyRunTypeManual    = "manual"
	TokenUsagePolicyRunTypeScheduled = "scheduled"
	TokenUsagePolicyRunTypeClear     = "clear"

	TokenUsagePolicyRunStatusRunning = "running"
	TokenUsagePolicyRunStatusSuccess = "success"
	TokenUsagePolicyRunStatusFailed  = "failed"

	TokenUsagePolicyChangeCreate     = "create"
	TokenUsagePolicyChangeUpdate     = "update"
	TokenUsagePolicyChangeDowngrade  = "downgrade"
	TokenUsagePolicyChangeClear      = "clear"
	TokenUsagePolicyChangeSkipManual = "skip_manual"

	TokenUsagePolicyManualGroupRemovedReason      = "policy-granted group access was removed manually"
	TokenUsagePolicyManualTakeoverPreservedReason = "manual takeover preserved; policy ownership cleared"

	TokenUsagePolicyConditionToken      = "token"
	TokenUsagePolicyConditionActualCost = "actual_cost"
	TokenUsagePolicyConditionBoth       = "both"
)

type TokenUsageAutoPolicy struct {
	ID                int64                       `json:"id"`
	Name              string                      `json:"name"`
	Enabled           bool                        `json:"enabled"`
	WindowDays        int                         `json:"window_days"`
	TargetGroupID     int64                       `json:"target_group_id"`
	TargetGroupName   string                      `json:"target_group_name,omitempty"`
	ActionMode        string                      `json:"action_mode"`
	ConflictMode      string                      `json:"conflict_mode"`
	ScheduleFrequency string                      `json:"schedule_frequency"`
	Filters           TokenUsageAutoPolicyFilters `json:"filters"`
	Tiers             []TokenUsageAutoPolicyTier  `json:"tiers,omitempty"`
	LastRunAt         *time.Time                  `json:"last_run_at,omitempty"`
	NextRunAt         *time.Time                  `json:"next_run_at,omitempty"`
	LatestRun         *TokenUsageAutoPolicyRun    `json:"latest_run,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

type TokenUsageAutoPolicyFilters struct {
	GroupID     *int64  `json:"group_id,omitempty"`
	Model       *string `json:"model,omitempty"`
	RequestType *int16  `json:"request_type,omitempty"`
	BillingType *int8   `json:"billing_type,omitempty"`
}

type TokenUsageAutoPolicyTier struct {
	ID             int64     `json:"id"`
	PolicyID       int64     `json:"policy_id,omitempty"`
	ConditionMode  string    `json:"condition_mode"`
	MinTokens      int64     `json:"min_tokens"`
	MinActualCost  float64   `json:"min_actual_cost"`
	RateMultiplier float64   `json:"rate_multiplier"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type TokenUsageAutoAssignment struct {
	ID                     int64      `json:"id"`
	PolicyID               int64      `json:"policy_id"`
	UserID                 int64      `json:"user_id"`
	UserName               string     `json:"user_name,omitempty"`
	UserEmail              string     `json:"user_email,omitempty"`
	TargetGroupID          int64      `json:"target_group_id"`
	TierID                 *int64     `json:"tier_id,omitempty"`
	LastTokenUsage         int64      `json:"last_token_usage"`
	LastActualCost         float64    `json:"last_actual_cost"`
	LastRateMultiplier     *float64   `json:"last_rate_multiplier,omitempty"`
	GroupGrantedByPolicy   bool       `json:"group_granted_by_policy"`
	PreviousRateMultiplier *float64   `json:"previous_rate_multiplier,omitempty"`
	ManualTakeover         bool       `json:"manual_takeover"`
	ManualTakeoverReason   string     `json:"manual_takeover_reason,omitempty"`
	ManualTakeoverAt       *time.Time `json:"manual_takeover_at,omitempty"`
	LastAppliedAt          *time.Time `json:"last_applied_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type TokenUsageAutoPolicyRun struct {
	ID             int64      `json:"id"`
	PolicyID       int64      `json:"policy_id"`
	RunType        string     `json:"run_type"`
	Status         string     `json:"status"`
	TotalUsers     int        `json:"total_users"`
	CreateCount    int        `json:"create_count"`
	UpdateCount    int        `json:"update_count"`
	DowngradeCount int        `json:"downgrade_count"`
	ClearCount     int        `json:"clear_count"`
	SkipCount      int        `json:"skip_count"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type TokenUsageAutoPolicyRunStats struct {
	TotalUsers     int `json:"total_users"`
	CreateCount    int `json:"create_count"`
	UpdateCount    int `json:"update_count"`
	DowngradeCount int `json:"downgrade_count"`
	ClearCount     int `json:"clear_count"`
	SkipCount      int `json:"skip_count"`
}

type TokenUsageAutoPolicyChange struct {
	ChangeType        string   `json:"change_type"`
	UserID            int64    `json:"user_id"`
	UserName          string   `json:"user_name,omitempty"`
	UserEmail         string   `json:"user_email,omitempty"`
	TokenUsage        int64    `json:"token_usage"`
	ActualCost        float64  `json:"actual_cost"`
	TargetGroupID     int64    `json:"target_group_id"`
	TierID            *int64   `json:"tier_id,omitempty"`
	TierMinTokens     *int64   `json:"tier_min_tokens,omitempty"`
	TierConditionMode string   `json:"tier_condition_mode,omitempty"`
	TierMinActualCost *float64 `json:"tier_min_actual_cost,omitempty"`
	OldRateMultiplier *float64 `json:"old_rate_multiplier,omitempty"`
	NewRateMultiplier *float64 `json:"new_rate_multiplier,omitempty"`
	Reason            string   `json:"reason,omitempty"`
	GroupGranted      bool     `json:"group_granted"`
	ManualTakeover    bool     `json:"manual_takeover"`
}

type TokenUsageAutoPolicyPreview struct {
	PolicyID int64                        `json:"policy_id"`
	Stats    TokenUsageAutoPolicyRunStats `json:"stats"`
	Changes  []TokenUsageAutoPolicyChange `json:"changes"`
}

type TokenUsageAutoPolicyInput struct {
	Name              string                      `json:"name"`
	Enabled           *bool                       `json:"enabled,omitempty"`
	WindowDays        int                         `json:"window_days"`
	TargetGroupID     int64                       `json:"target_group_id"`
	ActionMode        string                      `json:"action_mode"`
	ConflictMode      string                      `json:"conflict_mode"`
	ScheduleFrequency string                      `json:"schedule_frequency"`
	Filters           TokenUsageAutoPolicyFilters `json:"filters"`
	Tiers             []TokenUsageAutoPolicyTier  `json:"tiers"`
}

type TokenUsageAutoPolicyListFilters struct {
	Enabled       *bool
	TargetGroupID int64
}

type TokenUsageAutoPolicyUsageRow struct {
	UserID     int64
	UserName   string
	UserEmail  string
	TokenUsage int64
	ActualCost float64
}

type TokenUsageAutoPolicyState struct {
	UserID          int64
	UserName        string
	UserEmail       string
	CurrentRate     *float64
	HasAllowedGroup bool
	Assignment      *TokenUsageAutoAssignment
}

type TokenUsageAutoPolicyRepository interface {
	ListPolicies(ctx context.Context, params pagination.PaginationParams, filters TokenUsageAutoPolicyListFilters) ([]TokenUsageAutoPolicy, *pagination.PaginationResult, error)
	GetPolicyByID(ctx context.Context, id int64) (*TokenUsageAutoPolicy, error)
	CreatePolicy(ctx context.Context, policy *TokenUsageAutoPolicy, tiers []TokenUsageAutoPolicyTier) (*TokenUsageAutoPolicy, error)
	UpdatePolicy(ctx context.Context, policy *TokenUsageAutoPolicy, tiers []TokenUsageAutoPolicyTier) (*TokenUsageAutoPolicy, error)
	DeletePolicy(ctx context.Context, id int64) error
	CountAssignments(ctx context.Context, policyID int64) (int64, error)
	ListDuePolicies(ctx context.Context, now time.Time, limit int) ([]TokenUsageAutoPolicy, error)
	TryLockPolicy(ctx context.Context, policyID int64) (bool, error)
	AggregatePolicyUsage(ctx context.Context, policy TokenUsageAutoPolicy, since time.Time) ([]TokenUsageAutoPolicyUsageRow, error)
	ListPolicyStates(ctx context.Context, policyID, targetGroupID int64, userIDs []int64) (map[int64]TokenUsageAutoPolicyState, error)
	ListPolicyAssignmentStates(ctx context.Context, policyID int64) (map[int64]TokenUsageAutoPolicyState, error)
	ApplyPolicyChanges(ctx context.Context, policy TokenUsageAutoPolicy, changes []TokenUsageAutoPolicyChange, nextRunAt *time.Time) error
	ApplyPolicyChangesAndFinishRun(ctx context.Context, runID int64, policy TokenUsageAutoPolicy, changes []TokenUsageAutoPolicyChange, stats TokenUsageAutoPolicyRunStats, nextRunAt *time.Time, disablePolicy bool) error
	BeginPolicyRun(ctx context.Context, policyID int64, runType string) (*TokenUsageAutoPolicyRun, error)
	FinishPolicyRun(ctx context.Context, runID int64, status string, stats TokenUsageAutoPolicyRunStats, changes []TokenUsageAutoPolicyChange, errMessage string) error
	ListPolicyRuns(ctx context.Context, policyID int64, params pagination.PaginationParams) ([]TokenUsageAutoPolicyRun, *pagination.PaginationResult, error)
	ListPolicyRunChanges(ctx context.Context, policyID, runID int64, params pagination.PaginationParams) ([]TokenUsageAutoPolicyChange, *pagination.PaginationResult, error)
}

type TokenUsageAutoPolicyService struct {
	repo                 TokenUsageAutoPolicyRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewTokenUsageAutoPolicyService(repo TokenUsageAutoPolicyRepository) *TokenUsageAutoPolicyService {
	return &TokenUsageAutoPolicyService{repo: repo}
}

func ProvideTokenUsageAutoPolicyService(
	repo TokenUsageAutoPolicyRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *TokenUsageAutoPolicyService {
	svc := NewTokenUsageAutoPolicyService(repo)
	svc.authCacheInvalidator = authCacheInvalidator
	return svc
}

func (s *TokenUsageAutoPolicyService) ListPolicies(ctx context.Context, page, pageSize int, filters TokenUsageAutoPolicyListFilters) ([]TokenUsageAutoPolicy, *pagination.PaginationResult, error) {
	return s.repo.ListPolicies(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}, filters)
}

func (s *TokenUsageAutoPolicyService) GetPolicy(ctx context.Context, id int64) (*TokenUsageAutoPolicy, error) {
	return s.repo.GetPolicyByID(ctx, id)
}

func (s *TokenUsageAutoPolicyService) CreatePolicy(ctx context.Context, input TokenUsageAutoPolicyInput) (*TokenUsageAutoPolicy, error) {
	policy, tiers, err := normalizeTokenUsagePolicyInput(input, 0)
	if err != nil {
		return nil, err
	}
	if policy.Enabled {
		policy.NextRunAt = nextTokenUsagePolicyRunAt(policy.ScheduleFrequency, time.Now())
	}
	return s.repo.CreatePolicy(ctx, policy, tiers)
}

func (s *TokenUsageAutoPolicyService) UpdatePolicy(ctx context.Context, id int64, input TokenUsageAutoPolicyInput) (*TokenUsageAutoPolicy, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	policy, tiers, err := normalizeTokenUsagePolicyInput(input, id)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetPolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.TargetGroupID != policy.TargetGroupID {
		count, err := s.repo.CountAssignments(ctx, id)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, infraerrors.BadRequest("POLICY_TARGET_GROUP_LOCKED", "policy target group cannot be changed after automatic assignments exist")
		}
	}
	if policy.Enabled {
		policy.NextRunAt = nextTokenUsagePolicyRunAt(policy.ScheduleFrequency, time.Now())
	}
	updated, err := s.repo.UpdatePolicy(ctx, policy, tiers)
	if err != nil {
		return nil, err
	}
	invalidateUserGroupRateCacheForPolicyUpdate(*existing, *policy)
	return updated, nil
}

func (s *TokenUsageAutoPolicyService) DeletePolicy(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	if _, err := s.repo.GetPolicyByID(ctx, id); err != nil {
		return err
	}
	count, err := s.repo.CountAssignments(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return infraerrors.BadRequest("POLICY_HAS_ASSIGNMENTS", "policy has automatic assignments; disable and clear it before deletion")
	}
	return s.repo.DeletePolicy(ctx, id)
}

func (s *TokenUsageAutoPolicyService) PreviewPolicy(ctx context.Context, id int64) (*TokenUsageAutoPolicyPreview, error) {
	policy, err := s.repo.GetPolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.previewPolicy(ctx, *policy)
}

func (s *TokenUsageAutoPolicyService) RunPolicy(ctx context.Context, id int64, runType string) (*TokenUsageAutoPolicyRun, error) {
	if runType == "" {
		runType = TokenUsagePolicyRunTypeManual
	}
	if runType != TokenUsagePolicyRunTypeManual && runType != TokenUsagePolicyRunTypeScheduled {
		return nil, infraerrors.BadRequest("INVALID_RUN_TYPE", "run_type must be manual or scheduled")
	}
	policy, err := s.repo.GetPolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.runPolicy(ctx, *policy, runType)
}

func (s *TokenUsageAutoPolicyService) ClearPolicy(ctx context.Context, id int64) (*TokenUsageAutoPolicyRun, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	policy, err := s.repo.GetPolicyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.clearPolicy(ctx, *policy)
}

func (s *TokenUsageAutoPolicyService) ListRuns(ctx context.Context, policyID int64, page, pageSize int) ([]TokenUsageAutoPolicyRun, *pagination.PaginationResult, error) {
	if policyID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	if _, err := s.repo.GetPolicyByID(ctx, policyID); err != nil {
		return nil, nil, err
	}
	return s.repo.ListPolicyRuns(ctx, policyID, pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"})
}

func (s *TokenUsageAutoPolicyService) ListRunChanges(ctx context.Context, policyID, runID int64, page, pageSize int) ([]TokenUsageAutoPolicyChange, *pagination.PaginationResult, error) {
	if policyID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	if runID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_POLICY_RUN_ID", "invalid policy run id")
	}
	if _, err := s.repo.GetPolicyByID(ctx, policyID); err != nil {
		return nil, nil, err
	}
	return s.repo.ListPolicyRunChanges(ctx, policyID, runID, pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "id", SortOrder: "asc"})
}

func (s *TokenUsageAutoPolicyService) previewPolicy(ctx context.Context, policy TokenUsageAutoPolicy) (*TokenUsageAutoPolicyPreview, error) {
	changes, stats, err := s.buildPolicyChanges(ctx, policy)
	if err != nil {
		return nil, err
	}
	return &TokenUsageAutoPolicyPreview{
		PolicyID: policy.ID,
		Stats:    stats,
		Changes:  changes,
	}, nil
}

func (s *TokenUsageAutoPolicyService) runPolicy(ctx context.Context, policy TokenUsageAutoPolicy, runType string) (*TokenUsageAutoPolicyRun, error) {
	locked, err := s.repo.TryLockPolicy(ctx, policy.ID)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, infraerrors.Conflict("POLICY_ALREADY_RUNNING", "policy is already running")
	}

	run, err := s.repo.BeginPolicyRun(ctx, policy.ID, runType)
	if err != nil {
		return nil, err
	}

	changes, stats, buildErr := s.buildPolicyChanges(ctx, policy)
	if buildErr != nil {
		_ = s.repo.FinishPolicyRun(ctx, run.ID, TokenUsagePolicyRunStatusFailed, stats, nil, buildErr.Error())
		return nil, buildErr
	}

	nextRun := nextTokenUsagePolicyRunAt(policy.ScheduleFrequency, time.Now())
	if !policy.Enabled {
		nextRun = nil
	}
	if err := s.repo.ApplyPolicyChangesAndFinishRun(ctx, run.ID, policy, changes, stats, nextRun, false); err != nil {
		_ = s.repo.FinishPolicyRun(ctx, run.ID, TokenUsagePolicyRunStatusFailed, stats, nil, err.Error())
		return nil, err
	}
	s.invalidateAuthCacheForPolicyChanges(ctx, policy, changes)

	finished, _, err := s.repo.ListPolicyRuns(ctx, policy.ID, pagination.PaginationParams{Page: 1, PageSize: 1, SortBy: "created_at", SortOrder: "desc"})
	if err == nil && len(finished) > 0 {
		return &finished[0], nil
	}
	run.Status = TokenUsagePolicyRunStatusSuccess
	run.TotalUsers = stats.TotalUsers
	run.CreateCount = stats.CreateCount
	run.UpdateCount = stats.UpdateCount
	run.DowngradeCount = stats.DowngradeCount
	run.ClearCount = stats.ClearCount
	run.SkipCount = stats.SkipCount
	now := time.Now().UTC()
	run.FinishedAt = &now
	return run, nil
}

func (s *TokenUsageAutoPolicyService) clearPolicy(ctx context.Context, policy TokenUsageAutoPolicy) (*TokenUsageAutoPolicyRun, error) {
	locked, err := s.repo.TryLockPolicy(ctx, policy.ID)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, infraerrors.Conflict("POLICY_ALREADY_RUNNING", "policy is already running")
	}

	run, err := s.repo.BeginPolicyRun(ctx, policy.ID, TokenUsagePolicyRunTypeClear)
	if err != nil {
		return nil, err
	}

	changes, stats, buildErr := s.buildPolicyClearChanges(ctx, policy)
	if buildErr != nil {
		_ = s.repo.FinishPolicyRun(ctx, run.ID, TokenUsagePolicyRunStatusFailed, stats, nil, buildErr.Error())
		return nil, buildErr
	}

	var nextRun *time.Time
	if err := s.repo.ApplyPolicyChangesAndFinishRun(ctx, run.ID, policy, changes, stats, nextRun, true); err != nil {
		_ = s.repo.FinishPolicyRun(ctx, run.ID, TokenUsagePolicyRunStatusFailed, stats, nil, err.Error())
		return nil, err
	}
	s.invalidateAuthCacheForPolicyChanges(ctx, policy, changes)

	finished, _, err := s.repo.ListPolicyRuns(ctx, policy.ID, pagination.PaginationParams{Page: 1, PageSize: 1, SortBy: "created_at", SortOrder: "desc"})
	if err == nil && len(finished) > 0 {
		return &finished[0], nil
	}
	run.Status = TokenUsagePolicyRunStatusSuccess
	run.TotalUsers = stats.TotalUsers
	run.ClearCount = stats.ClearCount
	now := time.Now().UTC()
	run.FinishedAt = &now
	return run, nil
}

func (s *TokenUsageAutoPolicyService) buildPolicyClearChanges(ctx context.Context, policy TokenUsageAutoPolicy) ([]TokenUsageAutoPolicyChange, TokenUsageAutoPolicyRunStats, error) {
	if policy.ID <= 0 {
		return nil, TokenUsageAutoPolicyRunStats{}, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	states, err := s.repo.ListPolicyAssignmentStates(ctx, policy.ID)
	if err != nil {
		return nil, TokenUsageAutoPolicyRunStats{}, err
	}

	userIDs := make([]int64, 0, len(states))
	for userID := range states {
		userIDs = append(userIDs, userID)
	}
	userIDs = uniqueSortedInt64(userIDs)

	tierByID := make(map[int64]TokenUsageAutoPolicyTier, len(policy.Tiers))
	for _, tier := range policy.Tiers {
		tierByID[tier.ID] = tier
	}

	changes := make([]TokenUsageAutoPolicyChange, 0, len(userIDs))
	for _, userID := range userIDs {
		state := states[userID]
		if state.Assignment == nil {
			continue
		}
		assignment := state.Assignment
		targetGroupID := assignment.TargetGroupID
		if targetGroupID <= 0 {
			targetGroupID = policy.TargetGroupID
		}

		manualRate := assignment.ManualTakeover && assignment.ManualTakeoverReason != TokenUsagePolicyManualGroupRemovedReason
		if assignment.LastRateMultiplier != nil && !sameFloatPtr(state.CurrentRate, assignment.LastRateMultiplier) {
			manualRate = true
		}

		var tierMin *int64
		var tierConditionMode string
		var tierMinActualCost *float64
		if assignment.TierID != nil {
			if tier, ok := tierByID[*assignment.TierID]; ok {
				minTokens := tier.MinTokens
				minActualCost := tier.MinActualCost
				tierMin = &minTokens
				tierConditionMode = tier.ConditionMode
				tierMinActualCost = &minActualCost
			}
		}
		reason := "policy cleared by admin"
		if manualRate {
			reason = TokenUsagePolicyManualTakeoverPreservedReason
		}
		changes = append(changes, TokenUsageAutoPolicyChange{
			ChangeType:        TokenUsagePolicyChangeClear,
			UserID:            state.UserID,
			UserName:          state.UserName,
			UserEmail:         state.UserEmail,
			TokenUsage:        assignment.LastTokenUsage,
			ActualCost:        assignment.LastActualCost,
			TargetGroupID:     targetGroupID,
			TierID:            assignment.TierID,
			TierMinTokens:     tierMin,
			TierConditionMode: tierConditionMode,
			TierMinActualCost: tierMinActualCost,
			OldRateMultiplier: state.CurrentRate,
			Reason:            reason,
			GroupGranted:      assignment.GroupGrantedByPolicy,
			ManualTakeover:    manualRate,
		})
	}

	stats := TokenUsageAutoPolicyRunStats{
		TotalUsers: len(changes),
		ClearCount: len(changes),
	}
	return changes, stats, nil
}

func (s *TokenUsageAutoPolicyService) buildPolicyChanges(ctx context.Context, policy TokenUsageAutoPolicy) ([]TokenUsageAutoPolicyChange, TokenUsageAutoPolicyRunStats, error) {
	if policy.ID <= 0 {
		return nil, TokenUsageAutoPolicyRunStats{}, infraerrors.BadRequest("INVALID_POLICY_ID", "invalid policy id")
	}
	if len(policy.Tiers) == 0 {
		return nil, TokenUsageAutoPolicyRunStats{}, infraerrors.BadRequest("EMPTY_TIERS", "policy has no tiers")
	}

	since := time.Now().UTC().AddDate(0, 0, -policy.WindowDays)
	usageRows, err := s.repo.AggregatePolicyUsage(ctx, policy, since)
	if err != nil {
		return nil, TokenUsageAutoPolicyRunStats{}, err
	}

	userIDs := make([]int64, 0, len(usageRows))
	usageByUser := make(map[int64]TokenUsageAutoPolicyUsageRow, len(usageRows))
	for _, row := range usageRows {
		if row.UserID <= 0 {
			continue
		}
		userIDs = append(userIDs, row.UserID)
		usageByUser[row.UserID] = row
	}

	states, err := s.repo.ListPolicyStates(ctx, policy.ID, policy.TargetGroupID, userIDs)
	if err != nil {
		return nil, TokenUsageAutoPolicyRunStats{}, err
	}
	for userID, state := range states {
		if _, ok := usageByUser[userID]; !ok {
			userIDs = append(userIDs, userID)
		}
		_ = state
	}
	userIDs = uniqueSortedInt64(userIDs)

	tiers := append([]TokenUsageAutoPolicyTier(nil), policy.Tiers...)

	changes := make([]TokenUsageAutoPolicyChange, 0, len(userIDs))
	stats := TokenUsageAutoPolicyRunStats{TotalUsers: len(userIDs)}
	for _, userID := range userIDs {
		row := usageByUser[userID]
		state := states[userID]
		if state.UserID == 0 {
			state.UserID = userID
			state.UserName = row.UserName
			state.UserEmail = row.UserEmail
		}
		if row.UserName != "" {
			state.UserName = row.UserName
		}
		if row.UserEmail != "" {
			state.UserEmail = row.UserEmail
		}

		tier := selectTokenUsageTier(tiers, row.TokenUsage, row.ActualCost)
		change := buildTokenUsagePolicyChange(policy, state, row.TokenUsage, row.ActualCost, tier, tiers)
		if change == nil {
			continue
		}
		changes = append(changes, *change)
		switch change.ChangeType {
		case TokenUsagePolicyChangeCreate:
			stats.CreateCount++
		case TokenUsagePolicyChangeUpdate:
			stats.UpdateCount++
		case TokenUsagePolicyChangeDowngrade:
			stats.DowngradeCount++
		case TokenUsagePolicyChangeClear:
			stats.ClearCount++
		case TokenUsagePolicyChangeSkipManual:
			stats.SkipCount++
		}
	}

	return changes, stats, nil
}

func (s *TokenUsageAutoPolicyService) invalidateAuthCacheForPolicyChanges(ctx context.Context, policy TokenUsageAutoPolicy, changes []TokenUsageAutoPolicyChange) {
	seen := make(map[int64]struct{}, len(changes))
	for _, change := range changes {
		switch change.ChangeType {
		case TokenUsagePolicyChangeCreate, TokenUsagePolicyChangeUpdate, TokenUsagePolicyChangeDowngrade, TokenUsagePolicyChangeClear:
		default:
			continue
		}
		if change.UserID <= 0 {
			continue
		}
		if _, ok := seen[change.UserID]; ok {
			continue
		}
		seen[change.UserID] = struct{}{}
		if s.authCacheInvalidator != nil && (policy.ActionMode == TokenUsagePolicyActionGrantGroupAndRate || change.GroupGranted) {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, change.UserID)
		}
		invalidateUserGroupRateCache(change.UserID, change.TargetGroupID)
	}
}

func invalidateUserGroupRateCacheForPolicyUpdate(existing, policy TokenUsageAutoPolicy) {
	groups := make(map[int64]struct{}, 2)
	addGroup := func(groupID int64) {
		if groupID <= 0 {
			return
		}
		groups[groupID] = struct{}{}
	}

	if existing.Enabled != policy.Enabled {
		addGroup(existing.TargetGroupID)
	}
	if existing.TargetGroupID != policy.TargetGroupID {
		addGroup(existing.TargetGroupID)
		addGroup(policy.TargetGroupID)
	}

	for groupID := range groups {
		invalidateUserGroupRateCacheByGroupID(groupID)
	}
}

func buildTokenUsagePolicyChange(policy TokenUsageAutoPolicy, state TokenUsageAutoPolicyState, tokenUsage int64, actualCost float64, tier *TokenUsageAutoPolicyTier, tiers []TokenUsageAutoPolicyTier) *TokenUsageAutoPolicyChange {
	if state.Assignment != nil && state.Assignment.ManualTakeover && policy.ConflictMode == TokenUsagePolicyConflictManualPriority {
		groupOwnershipRemoved := state.Assignment.GroupGrantedByPolicy && !state.HasAllowedGroup
		reason := state.Assignment.ManualTakeoverReason
		if groupOwnershipRemoved {
			reason = TokenUsagePolicyManualGroupRemovedReason
		}
		return &TokenUsageAutoPolicyChange{
			ChangeType:     TokenUsagePolicyChangeSkipManual,
			UserID:         state.UserID,
			UserName:       state.UserName,
			UserEmail:      state.UserEmail,
			TokenUsage:     tokenUsage,
			ActualCost:     actualCost,
			TargetGroupID:  policy.TargetGroupID,
			Reason:         reason,
			GroupGranted:   groupOwnershipRemoved,
			ManualTakeover: true,
		}
	}

	if policy.ConflictMode == TokenUsagePolicyConflictManualPriority && state.Assignment != nil {
		if state.Assignment.LastRateMultiplier != nil && !sameFloatPtr(state.CurrentRate, state.Assignment.LastRateMultiplier) {
			reason := "rate_multiplier was changed manually"
			return &TokenUsageAutoPolicyChange{
				ChangeType:        TokenUsagePolicyChangeSkipManual,
				UserID:            state.UserID,
				UserName:          state.UserName,
				UserEmail:         state.UserEmail,
				TokenUsage:        tokenUsage,
				ActualCost:        actualCost,
				TargetGroupID:     policy.TargetGroupID,
				OldRateMultiplier: state.CurrentRate,
				Reason:            reason,
				GroupGranted:      state.Assignment.GroupGrantedByPolicy && !state.HasAllowedGroup,
				ManualTakeover:    true,
			}
		}
		if state.Assignment.GroupGrantedByPolicy && !state.HasAllowedGroup {
			return &TokenUsageAutoPolicyChange{
				ChangeType:     TokenUsagePolicyChangeSkipManual,
				UserID:         state.UserID,
				UserName:       state.UserName,
				UserEmail:      state.UserEmail,
				TokenUsage:     tokenUsage,
				ActualCost:     actualCost,
				TargetGroupID:  policy.TargetGroupID,
				Reason:         TokenUsagePolicyManualGroupRemovedReason,
				GroupGranted:   true,
				ManualTakeover: true,
			}
		}
	}

	if tier == nil {
		if state.Assignment == nil {
			return nil
		}
		return &TokenUsageAutoPolicyChange{
			ChangeType:        TokenUsagePolicyChangeClear,
			UserID:            state.UserID,
			UserName:          state.UserName,
			UserEmail:         state.UserEmail,
			TokenUsage:        tokenUsage,
			ActualCost:        actualCost,
			TargetGroupID:     policy.TargetGroupID,
			OldRateMultiplier: state.CurrentRate,
			Reason:            "usage below the lowest tier",
			GroupGranted:      state.Assignment.GroupGrantedByPolicy,
		}
	}

	newRate := tier.RateMultiplier
	changeType := TokenUsagePolicyChangeCreate
	var oldRate *float64
	if state.CurrentRate != nil {
		v := *state.CurrentRate
		oldRate = &v
	}
	if state.Assignment != nil {
		if isDowngradeTier(state.Assignment.TierID, tier, tiers) {
			changeType = TokenUsagePolicyChangeDowngrade
		} else if !sameFloatPtr(state.CurrentRate, &newRate) {
			changeType = TokenUsagePolicyChangeUpdate
		} else if policy.ActionMode == TokenUsagePolicyActionGrantGroupAndRate && state.Assignment.GroupGrantedByPolicy && !state.HasAllowedGroup {
			changeType = TokenUsagePolicyChangeUpdate
		} else {
			return nil
		}
	} else if state.CurrentRate != nil {
		if policy.ConflictMode == TokenUsagePolicyConflictManualPriority {
			tierID := tier.ID
			minTokens := tier.MinTokens
			reason := "existing rate_multiplier is manual"
			return &TokenUsageAutoPolicyChange{
				ChangeType:        TokenUsagePolicyChangeSkipManual,
				UserID:            state.UserID,
				UserName:          state.UserName,
				UserEmail:         state.UserEmail,
				TokenUsage:        tokenUsage,
				ActualCost:        actualCost,
				TargetGroupID:     policy.TargetGroupID,
				TierID:            &tierID,
				TierMinTokens:     &minTokens,
				TierConditionMode: tier.ConditionMode,
				TierMinActualCost: float64Ptr(tier.MinActualCost),
				OldRateMultiplier: oldRate,
				NewRateMultiplier: &newRate,
				Reason:            reason,
				ManualTakeover:    true,
			}
		}
		changeType = TokenUsagePolicyChangeUpdate
	}

	tierID := tier.ID
	minTokens := tier.MinTokens
	return &TokenUsageAutoPolicyChange{
		ChangeType:        changeType,
		UserID:            state.UserID,
		UserName:          state.UserName,
		UserEmail:         state.UserEmail,
		TokenUsage:        tokenUsage,
		ActualCost:        actualCost,
		TargetGroupID:     policy.TargetGroupID,
		TierID:            &tierID,
		TierMinTokens:     &minTokens,
		TierConditionMode: tier.ConditionMode,
		TierMinActualCost: float64Ptr(tier.MinActualCost),
		OldRateMultiplier: oldRate,
		NewRateMultiplier: &newRate,
		GroupGranted:      policy.ActionMode == TokenUsagePolicyActionGrantGroupAndRate && !state.HasAllowedGroup,
	}
}

func isDowngradeTier(previousTierID *int64, current *TokenUsageAutoPolicyTier, tiers []TokenUsageAutoPolicyTier) bool {
	if previousTierID == nil || current == nil {
		return false
	}
	for i := range tiers {
		if tiers[i].ID == *previousTierID {
			previous := tiers[i]
			if current.SortOrder > 0 && previous.SortOrder > 0 {
				return current.SortOrder < previous.SortOrder
			}
			return current.MinTokens < previous.MinTokens
		}
	}
	return false
}

func selectTokenUsageTier(tiers []TokenUsageAutoPolicyTier, tokenUsage int64, actualCost float64) *TokenUsageAutoPolicyTier {
	var selected *TokenUsageAutoPolicyTier
	for i := range tiers {
		matches := false
		switch tiers[i].ConditionMode {
		case TokenUsagePolicyConditionActualCost:
			matches = actualCost >= tiers[i].MinActualCost
		case TokenUsagePolicyConditionBoth:
			matches = tokenUsage >= tiers[i].MinTokens && actualCost >= tiers[i].MinActualCost
		default:
			matches = tokenUsage >= tiers[i].MinTokens
		}
		if matches {
			selected = &tiers[i]
		}
	}
	return selected
}

func sameFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 0.00001
}

func uniqueSortedInt64(values []int64) []int64 {
	if len(values) == 0 {
		return []int64{}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	out := values[:0]
	var last int64
	for i, v := range values {
		if v <= 0 {
			continue
		}
		if i > 0 && v == last {
			continue
		}
		out = append(out, v)
		last = v
	}
	return out
}

func normalizeTokenUsagePolicyInput(input TokenUsageAutoPolicyInput, id int64) (*TokenUsageAutoPolicy, []TokenUsageAutoPolicyTier, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, infraerrors.BadRequest("INVALID_POLICY_NAME", "policy name is required")
	}
	if input.WindowDays == 0 {
		input.WindowDays = 30
	}
	if input.WindowDays != 7 && input.WindowDays != 30 {
		return nil, nil, infraerrors.BadRequest("INVALID_WINDOW_DAYS", "window_days must be 7 or 30")
	}
	if input.TargetGroupID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_TARGET_GROUP", "target_group_id is required")
	}
	if input.ActionMode == "" {
		input.ActionMode = TokenUsagePolicyActionRateOnly
	}
	if !isValidTokenUsageActionMode(input.ActionMode) {
		return nil, nil, infraerrors.BadRequest("INVALID_ACTION_MODE", "invalid action_mode")
	}
	if input.ConflictMode == "" {
		input.ConflictMode = TokenUsagePolicyConflictManualPriority
	}
	if !isValidTokenUsageConflictMode(input.ConflictMode) {
		return nil, nil, infraerrors.BadRequest("INVALID_CONFLICT_MODE", "invalid conflict_mode")
	}
	if input.ScheduleFrequency == "" {
		input.ScheduleFrequency = TokenUsagePolicyFrequencyDaily
	}
	if !isValidTokenUsageFrequency(input.ScheduleFrequency) {
		return nil, nil, infraerrors.BadRequest("INVALID_SCHEDULE_FREQUENCY", "invalid schedule_frequency")
	}
	if input.Filters.GroupID != nil && *input.Filters.GroupID <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_FILTER_GROUP", "filters.group_id must be positive")
	}
	if input.Filters.Model != nil {
		model := strings.TrimSpace(*input.Filters.Model)
		if model == "" {
			input.Filters.Model = nil
		} else {
			input.Filters.Model = &model
		}
	}
	if input.Filters.RequestType != nil && !RequestType(*input.Filters.RequestType).IsValid() {
		return nil, nil, infraerrors.BadRequest("INVALID_FILTER_REQUEST_TYPE", "invalid filters.request_type")
	}
	if input.Filters.BillingType != nil {
		switch *input.Filters.BillingType {
		case BillingTypeBalance, BillingTypeSubscription:
		default:
			return nil, nil, infraerrors.BadRequest("INVALID_FILTER_BILLING_TYPE", "invalid filters.billing_type")
		}
	}
	if len(input.Tiers) == 0 {
		return nil, nil, infraerrors.BadRequest("EMPTY_TIERS", "at least one tier is required")
	}

	tiers := make([]TokenUsageAutoPolicyTier, 0, len(input.Tiers))
	for _, tier := range input.Tiers {
		conditionMode := tier.ConditionMode
		if conditionMode == "" {
			conditionMode = TokenUsagePolicyConditionToken
		}
		if !isValidTokenUsageConditionMode(conditionMode) {
			return nil, nil, infraerrors.BadRequest("INVALID_TIER_CONDITION_MODE", "invalid tier condition_mode")
		}
		if tier.MinTokens < 0 {
			return nil, nil, infraerrors.BadRequest("INVALID_TIER_MIN_TOKENS", "tier min_tokens must be non-negative")
		}
		if tier.MinActualCost < 0 || math.IsNaN(tier.MinActualCost) || math.IsInf(tier.MinActualCost, 0) {
			return nil, nil, infraerrors.BadRequest("INVALID_TIER_MIN_ACTUAL_COST", "tier min_actual_cost must be finite and non-negative")
		}
		if tier.RateMultiplier <= 0 || math.IsNaN(tier.RateMultiplier) || math.IsInf(tier.RateMultiplier, 0) {
			return nil, nil, infraerrors.BadRequest("INVALID_TIER_RATE", "tier rate_multiplier must be greater than 0")
		}
		minTokens := tier.MinTokens
		minActualCost := tier.MinActualCost
		if conditionMode == TokenUsagePolicyConditionActualCost {
			minTokens = 0
		}
		if conditionMode == TokenUsagePolicyConditionToken {
			minActualCost = 0
		}
		for _, existing := range tiers {
			if existing.ConditionMode == conditionMode && existing.MinTokens == minTokens && existing.MinActualCost == minActualCost {
				return nil, nil, infraerrors.BadRequest("DUPLICATE_TIER_THRESHOLD", "tier condition must be unique")
			}
		}
		tiers = append(tiers, TokenUsageAutoPolicyTier{
			ConditionMode:  conditionMode,
			MinTokens:      minTokens,
			MinActualCost:  minActualCost,
			RateMultiplier: tier.RateMultiplier,
			SortOrder:      len(tiers) + 1,
		})
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	return &TokenUsageAutoPolicy{
		ID:                id,
		Name:              name,
		Enabled:           enabled,
		WindowDays:        input.WindowDays,
		TargetGroupID:     input.TargetGroupID,
		ActionMode:        input.ActionMode,
		ConflictMode:      input.ConflictMode,
		ScheduleFrequency: input.ScheduleFrequency,
		Filters:           input.Filters,
	}, tiers, nil
}

func isValidTokenUsageActionMode(value string) bool {
	return value == TokenUsagePolicyActionRateOnly || value == TokenUsagePolicyActionGrantGroupAndRate
}

func isValidTokenUsageConditionMode(value string) bool {
	return value == TokenUsagePolicyConditionToken || value == TokenUsagePolicyConditionActualCost || value == TokenUsagePolicyConditionBoth
}

func isValidTokenUsageConflictMode(value string) bool {
	return value == TokenUsagePolicyConflictManualPriority || value == TokenUsagePolicyConflictAutoPriority
}

func isValidTokenUsageFrequency(value string) bool {
	return value == TokenUsagePolicyFrequencyEvery6h || value == TokenUsagePolicyFrequencyDaily || value == TokenUsagePolicyFrequencyWeekly
}

func nextTokenUsagePolicyRunAt(frequency string, from time.Time) *time.Time {
	next := from.UTC()
	switch frequency {
	case TokenUsagePolicyFrequencyEvery6h:
		next = next.Add(6 * time.Hour)
	case TokenUsagePolicyFrequencyWeekly:
		next = next.Add(7 * 24 * time.Hour)
	default:
		next = next.Add(24 * time.Hour)
	}
	return &next
}
