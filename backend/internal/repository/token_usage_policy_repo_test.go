package repository

import (
	"context"
	"net/http"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), newRate, false, oldRate, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE token_usage_auto_policies").
		WithArgs(int64(9), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ApplyPolicyChanges(context.Background(), policy, []service.TokenUsageAutoPolicyChange{change}, nil)

	require.NoError(t, err)
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
		WithArgs(int64(9), int64(7), int64(8), tierID, int64(1500), oldRate, "manual").
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
		"target_group_id", "tier_id", "last_token_usage", "last_rate_multiplier",
		"group_granted_by_policy", "previous_rate_multiplier", "manual_takeover",
		"manual_takeover_reason", "manual_takeover_at", "last_applied_at",
		"created_at", "updated_at", "rate_multiplier", "has_allowed_group",
	})
}
