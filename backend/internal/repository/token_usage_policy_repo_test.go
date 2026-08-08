package repository

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestTokenUsagePolicyRepositoryGetPolicyByIDNotFoundReturnsBadRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	mock.ExpectQuery("SELECT p\\.id, p\\.name, p\\.enabled").
		WithArgs(int64(99)).
		WillReturnRows(tokenUsagePolicyRows())

	_, err = repo.GetPolicyByID(context.Background(), 99)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_POLICY_ID", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryCreatePolicyDuplicateEnabledTargetGroupReturnsConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	policy := &service.TokenUsageAutoPolicy{
		Name:              "policy",
		Enabled:           true,
		WindowDays:        30,
		TargetGroupID:     8,
		ActionMode:        service.TokenUsagePolicyActionRateOnly,
		ConflictMode:      service.TokenUsagePolicyConflictManualPriority,
		ScheduleFrequency: service.TokenUsagePolicyFrequencyDaily,
	}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO token_usage_auto_policies").
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	_, err = repo.CreatePolicy(context.Background(), policy, []service.TokenUsageAutoPolicyTier{{MinTokens: 0, RateMultiplier: 1, SortOrder: 1}})

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "DUPLICATE_ENABLED_TARGET_GROUP", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryCreatePolicyInvalidGroupReturnsBadRequest(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		reason     string
	}{
		{
			name:       "target group",
			constraint: "token_usage_auto_policies_target_group_id_fkey",
			reason:     "INVALID_TARGET_GROUP",
		},
		{
			name:       "filter group",
			constraint: "token_usage_auto_policies_filter_group_id_fkey",
			reason:     "INVALID_FILTER_GROUP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := NewTokenUsageAutoPolicyRepository(db)

			policy := &service.TokenUsageAutoPolicy{
				Name:              "policy",
				Enabled:           true,
				WindowDays:        30,
				TargetGroupID:     8,
				ActionMode:        service.TokenUsagePolicyActionRateOnly,
				ConflictMode:      service.TokenUsagePolicyConflictManualPriority,
				ScheduleFrequency: service.TokenUsagePolicyFrequencyDaily,
			}
			mock.ExpectBegin()
			mock.ExpectQuery("INSERT INTO token_usage_auto_policies").
				WillReturnError(&pq.Error{Code: "23503", Constraint: tt.constraint})
			mock.ExpectRollback()

			_, err = repo.CreatePolicy(context.Background(), policy, []service.TokenUsageAutoPolicyTier{{MinTokens: 0, RateMultiplier: 1, SortOrder: 1}})

			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, tt.reason, infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTokenUsagePolicyRepositoryDeletePolicyNoRowsReturnsBadRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	mock.ExpectExec("DELETE FROM token_usage_auto_policies").
		WithArgs(int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.DeletePolicy(context.Background(), 99)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_POLICY_ID", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryCountAssignmentsIgnoresManualOnlyTakeovers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM token_usage_auto_assignments").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	count, err := repo.CountAssignments(context.Background(), 9)

	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryListPolicyStatesScopesAssignmentsToTargetGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	mock.ExpectQuery("SELECT a\\.id, a\\.policy_id").
		WithArgs(int64(9), int64(8)).
		WillReturnRows(tokenUsageAssignmentStateRows())

	states, err := repo.ListPolicyStates(context.Background(), 9, 8, nil)

	require.NoError(t, err)
	require.Empty(t, states)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearPreservesManualTakeoverRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	policy := service.TokenUsageAutoPolicy{
		ID:            9,
		TargetGroupID: 8,
		ActionMode:    service.TokenUsagePolicyActionGrantGroupAndRate,
	}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:     service.TokenUsagePolicyChangeClear,
		UserID:         7,
		TargetGroupID:  8,
		ManualTakeover: true,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(nil, 0.7, 0.6, true, int64(8)))
	mock.ExpectExec("DELETE FROM user_allowed_groups").
		WithArgs(int64(7), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyPolicyChangesDoesNotMarkConflictedGroupGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	oldRate := 1.0
	newRate := 0.7
	policy := service.TokenUsageAutoPolicy{
		ID:            9,
		TargetGroupID: 8,
		ActionMode:    service.TokenUsagePolicyActionGrantGroupAndRate,
	}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeCreate,
		UserID:            7,
		TokenUsage:        1500,
		TierID:            &tierID,
		OldRateMultiplier: &oldRate,
		NewRateMultiplier: &newRate,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO user_allowed_groups").
		WithArgs(int64(7), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(int64(7), int64(8), newRate).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT previous_rate_multiplier FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier"}).AddRow(nil))
	mock.ExpectExec("INSERT INTO token_usage_auto_assignments").
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), float64(0), newRate, false, oldRate, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearDetectsManualRateBeforeRestore(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:     service.TokenUsagePolicyChangeClear,
		UserID:         7,
		TargetGroupID:  8,
		ManualTakeover: false,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(1.0, 0.7, 0.6, false, int64(8)))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearRestoresPreviousRateConditionally(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:    service.TokenUsagePolicyChangeClear,
		UserID:        7,
		TargetGroupID: 8,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(1.0, 0.7, 0.7, false, int64(8)))
	mock.ExpectExec("rate_multiplier = \\$4::decimal").
		WithArgs(int64(7), int64(8), 1.0, 0.7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearMarksManualTakeoverWhenConditionalRestoreMisses(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	currentRate := 0.7
	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, ClearCount: 1}
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:    service.TokenUsagePolicyChangeClear,
		UserID:        7,
		TokenUsage:    1500,
		TargetGroupID: 8,
		Reason:        "policy cleared by admin",
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(1.0, currentRate, currentRate, false, int64(8)))
	mock.ExpectExec("rate_multiplier = \\$4::decimal").
		WithArgs(int64(7), int64(8), 1.0, currentRate).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(
			int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail,
			change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, &currentRate,
			change.NewRateMultiplier, service.TokenUsagePolicyManualTakeoverPreservedReason, false, true,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_runs").
		WithArgs(int64(12), service.TokenUsagePolicyRunStatusSuccess, stats.TotalUsers, stats.CreateCount, stats.UpdateCount, stats.DowngradeCount, stats.ClearCount, stats.SkipCount, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChangesAndFinishRun(context.Background(), 12, policy, []service.TokenUsageAutoPolicyChange{change}, stats, nil, false)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryFinishPolicyRunPersistsChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	minTokens := int64(1000)
	oldRate := 1.0
	newRate := 0.7
	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, UpdateCount: 1}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeUpdate,
		UserID:            7,
		UserName:          "u7",
		UserEmail:         "u7@example.com",
		TokenUsage:        1500,
		TargetGroupID:     8,
		TierID:            &tierID,
		TierMinTokens:     &minTokens,
		OldRateMultiplier: &oldRate,
		NewRateMultiplier: &newRate,
		Reason:            "tier matched",
		GroupGranted:      true,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT policy_id FROM token_usage_auto_runs").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"policy_id"}).AddRow(int64(9)))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail, change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, change.OldRateMultiplier, change.NewRateMultiplier, change.Reason, change.GroupGranted, change.ManualTakeover).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_runs").
		WithArgs(int64(12), service.TokenUsagePolicyRunStatusSuccess, stats.TotalUsers, stats.CreateCount, stats.UpdateCount, stats.DowngradeCount, stats.ClearCount, stats.SkipCount, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.FinishPolicyRun(context.Background(), 12, service.TokenUsagePolicyRunStatusSuccess, stats, []service.TokenUsageAutoPolicyChange{change}, "")

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryFinishPolicyRunMarksFailedWhenPersistingChangesFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, UpdateCount: 1}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:    service.TokenUsagePolicyChangeUpdate,
		UserID:        7,
		TokenUsage:    1500,
		TargetGroupID: 8,
	}
	insertErr := errors.New("insert audit failed")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT policy_id FROM token_usage_auto_runs").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"policy_id"}).AddRow(int64(9)))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail, change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, change.OldRateMultiplier, change.NewRateMultiplier, change.Reason, change.GroupGranted, change.ManualTakeover).
		WillReturnError(insertErr)
	mock.ExpectRollback()

	err = repo.FinishPolicyRun(context.Background(), 12, service.TokenUsagePolicyRunStatusSuccess, stats, []service.TokenUsageAutoPolicyChange{change}, "")

	require.ErrorIs(t, err, insertErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyPolicyChangesAndFinishRunIsAtomicOnAuditFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	oldRate := 1.0
	newRate := 0.7
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, UpdateCount: 1}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeUpdate,
		UserID:            7,
		TokenUsage:        1500,
		TargetGroupID:     8,
		TierID:            &tierID,
		OldRateMultiplier: &oldRate,
		NewRateMultiplier: &newRate,
	}
	insertErr := errors.New("insert audit failed")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(int64(7), int64(8), newRate).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT previous_rate_multiplier FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier"}).AddRow(nil))
	mock.ExpectExec("INSERT INTO token_usage_auto_assignments").
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), float64(0), newRate, false, oldRate, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail, change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, change.OldRateMultiplier, change.NewRateMultiplier, change.Reason, change.GroupGranted, change.ManualTakeover).
		WillReturnError(insertErr)
	mock.ExpectRollback()

	err = repo.ApplyPolicyChangesAndFinishRun(context.Background(), 12, policy, []service.TokenUsageAutoPolicyChange{change}, stats, nil, false)

	require.ErrorIs(t, err, insertErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearAndFinishRunPersistsHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	oldRate := 0.7
	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, ClearCount: 1}
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeClear,
		UserID:            7,
		TokenUsage:        1500,
		TargetGroupID:     8,
		OldRateMultiplier: &oldRate,
		Reason:            "policy cleared by admin",
		GroupGranted:      true,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(nil, 0.7, 0.7, true, int64(8)))
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(7), int64(8), 0.7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(7), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM user_allowed_groups").
		WithArgs(int64(7), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail, change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, change.OldRateMultiplier, change.NewRateMultiplier, change.Reason, change.GroupGranted, change.ManualTakeover).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_runs").
		WithArgs(int64(12), service.TokenUsagePolicyRunStatusSuccess, stats.TotalUsers, stats.CreateCount, stats.UpdateCount, stats.DowngradeCount, stats.ClearCount, stats.SkipCount, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChangesAndFinishRun(context.Background(), 12, policy, []service.TokenUsageAutoPolicyChange{change}, stats, nil, true)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyClearAndFinishRunAuditsManualRateDetectedInTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	stats := service.TokenUsageAutoPolicyRunStats{TotalUsers: 1, ClearCount: 1}
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:    service.TokenUsagePolicyChangeClear,
		UserID:        7,
		TokenUsage:    1500,
		TargetGroupID: 8,
		Reason:        "policy cleared by admin",
	}
	currentManualRate := 0.6

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a\\.previous_rate_multiplier, a\\.last_rate_multiplier, ugr\\.rate_multiplier").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_multiplier", "last_rate_multiplier", "rate_multiplier", "group_granted_by_policy", "target_group_id"}).
			AddRow(1.0, 0.7, currentManualRate, false, int64(8)))
	mock.ExpectExec("DELETE FROM token_usage_auto_assignments").
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO token_usage_auto_run_changes").
		WithArgs(
			int64(12), int64(9), change.ChangeType, change.UserID, change.UserName, change.UserEmail,
			change.TokenUsage, float64(0), change.TargetGroupID, change.TierID, change.TierMinTokens, "", nil, &currentManualRate,
			change.NewRateMultiplier, service.TokenUsagePolicyManualTakeoverPreservedReason, false, true,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_runs").
		WithArgs(int64(12), service.TokenUsagePolicyRunStatusSuccess, stats.TotalUsers, stats.CreateCount, stats.UpdateCount, stats.DowngradeCount, stats.ClearCount, stats.SkipCount, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChangesAndFinishRun(context.Background(), 12, policy, []service.TokenUsageAutoPolicyChange{change}, stats, nil, true)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryListPolicyRunsDoesNotAttachChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)
	now := time.Now().UTC()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM token_usage_auto_runs").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("SELECT id, policy_id, run_type, status").
		WithArgs(int64(9), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "policy_id", "run_type", "status", "total_users",
			"create_count", "update_count", "downgrade_count", "clear_count", "skip_count",
			"error_message", "started_at", "finished_at", "created_at",
		}).AddRow(
			int64(12), int64(9), service.TokenUsagePolicyRunTypeManual, service.TokenUsagePolicyRunStatusSuccess, 1,
			1, 0, 0, 0, 0,
			"", now, now, now,
		))

	runs, pageResult, err := repo.ListPolicyRuns(context.Background(), 9, pagination.PaginationParams{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Equal(t, int64(1), pageResult.Total)
	require.Len(t, runs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryListPolicyRunChangesScopesRunToPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	changes, pageResult, err := repo.ListPolicyRunChanges(context.Background(), 9, 12, pagination.PaginationParams{Page: 1, PageSize: 20})

	require.Error(t, err)
	require.Nil(t, changes)
	require.Nil(t, pageResult)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_POLICY_RUN_ID", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryListPolicyRunChangesReturnsScopedChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	minTokens := int64(1000)
	oldRate := 1.0
	newRate := 0.7
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(int64(9), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("SELECT run_id, change_type").
		WithArgs(int64(9), int64(12), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"run_id", "change_type", "user_id", "user_name", "user_email",
			"token_usage", "actual_cost", "target_group_id", "tier_id", "tier_min_tokens", "tier_condition_mode", "tier_min_actual_cost",
			"old_rate_multiplier", "new_rate_multiplier", "reason",
			"group_granted", "manual_takeover",
		}).AddRow(
			int64(12), service.TokenUsagePolicyChangeUpdate, int64(7), "u7", "u7@example.com",
			int64(1500), float64(2.5), int64(8), tierID, minTokens, service.TokenUsagePolicyConditionBoth, float64(2),
			oldRate, newRate, "tier matched",
			true, false,
		))

	changes, pageResult, err := repo.ListPolicyRunChanges(context.Background(), 9, 12, pagination.PaginationParams{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, int64(1), pageResult.Total)
	require.Equal(t, 1, pageResult.Page)
	require.Equal(t, 20, pageResult.PageSize)
	require.Equal(t, service.TokenUsagePolicyChangeUpdate, changes[0].ChangeType)
	require.Equal(t, int64(7), changes[0].UserID)
	require.InDelta(t, newRate, *changes[0].NewRateMultiplier, 0.00001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyPolicyChangesManualSkipUpdatesTargetGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	oldRate := 0.6
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeSkipManual,
		UserID:            7,
		TokenUsage:        1500,
		TierID:            &tierID,
		OldRateMultiplier: &oldRate,
		Reason:            "manual",
	}

	mock.ExpectBegin()
	mock.ExpectExec("target_group_id = EXCLUDED\\.target_group_id").
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), float64(0), oldRate, "manual", false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryApplyManualSkipClearsRemovedGroupOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	tierID := int64(3)
	oldRate := 0.7
	policy := service.TokenUsageAutoPolicy{ID: 9, TargetGroupID: 8}
	change := service.TokenUsageAutoPolicyChange{
		ChangeType:        service.TokenUsagePolicyChangeSkipManual,
		UserID:            7,
		TokenUsage:        1500,
		TierID:            &tierID,
		OldRateMultiplier: &oldRate,
		Reason:            service.TokenUsagePolicyManualGroupRemovedReason,
		GroupGranted:      true,
		ManualTakeover:    true,
	}

	mock.ExpectBegin()
	mock.ExpectExec("group_granted_by_policy = CASE").
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), float64(0), oldRate, service.TokenUsagePolicyManualGroupRemovedReason, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenUsagePolicyRepositoryBeginPolicyRunRecoversStaleRunningBeforeInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewTokenUsageAutoPolicyRepository(db)

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE token_usage_auto_runs").
		WithArgs(int64(9), service.TokenUsagePolicyRunStatusRunning, service.TokenUsagePolicyRunStatusFailed, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO token_usage_auto_runs").
		WithArgs(int64(9), service.TokenUsagePolicyRunTypeManual, service.TokenUsagePolicyRunStatusRunning).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "policy_id", "run_type", "status", "total_users",
			"create_count", "update_count", "downgrade_count", "clear_count", "skip_count",
			"error_message", "started_at", "finished_at", "created_at",
		}).AddRow(
			int64(12), int64(9), service.TokenUsagePolicyRunTypeManual, service.TokenUsagePolicyRunStatusRunning, 0,
			0, 0, 0, 0, 0,
			"", now, nil, now,
		))
	mock.ExpectCommit()

	run, err := repo.BeginPolicyRun(context.Background(), 9, service.TokenUsagePolicyRunTypeManual)

	require.NoError(t, err)
	require.Equal(t, int64(12), run.ID)
	require.Equal(t, service.TokenUsagePolicyRunStatusRunning, run.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func tokenUsagePolicyRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "enabled", "window_days", "target_group_id", "target_group_name",
		"action_mode", "conflict_mode", "schedule_frequency",
		"filter_group_id", "filter_model", "filter_request_type", "filter_billing_type",
		"last_run_at", "next_run_at", "created_at", "updated_at",
	})
}

func tokenUsageAssignmentStateRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "policy_id", "user_id", "username", "email",
		"target_group_id", "tier_id", "last_token_usage", "last_actual_cost", "last_rate_multiplier",
		"group_granted_by_policy", "previous_rate_multiplier", "manual_takeover",
		"manual_takeover_reason", "manual_takeover_at", "last_applied_at",
		"created_at", "updated_at", "rate_multiplier", "has_allowed_group",
	})
}
