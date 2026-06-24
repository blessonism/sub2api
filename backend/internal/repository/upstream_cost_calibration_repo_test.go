package repository

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpstreamCostCalibrationRepositoryApplyRunSuggestionsUpdatesPriorityAndAudit(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamCostCalibrationRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT r\\.status, r\\.applied, r\\.target_group_id").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "target_group_id"}).
			AddRow(service.UpstreamCostCalibrationRunStatusSuccess, false, int64(7)))
	mock.ExpectQuery("SELECT account_id, old_priority, new_priority").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "old_priority", "new_priority"}).
			AddRow(int64(101), 30, 10).
			AddRow(int64(102), 40, 20))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(7), 10, int64(30)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(102), int64(7), 20, int64(40)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_cost_calibration_suggestions").
		WithArgs(int64(12), int64(9), int64(66)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE upstream_cost_calibration_runs").
		WithArgs(int64(12), int64(9), int64(66)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectCalibrationRunDetail(mock, now, true)

	run, err := repo.ApplyRunSuggestions(ctx, 9, 12, 66)

	require.NoError(t, err)
	require.True(t, run.Applied)
	require.Len(t, run.Suggestions, 2)
	require.True(t, run.Suggestions[0].Applied)
	require.Equal(t, int64(66), *run.Suggestions[0].AppliedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamCostCalibrationRepositoryApplyRunSuggestionsRejectsStalePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamCostCalibrationRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT r\\.status, r\\.applied, r\\.target_group_id").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "target_group_id"}).
			AddRow(service.UpstreamCostCalibrationRunStatusSuccess, false, int64(7)))
	mock.ExpectQuery("SELECT account_id, old_priority, new_priority").
		WithArgs(int64(12), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "old_priority", "new_priority"}).
			AddRow(int64(101), 30, 10))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(7), 10, int64(30)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(101), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	_, err := repo.ApplyRunSuggestions(ctx, 9, 12, 66)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "CALIBRATION_PRIORITY_CHANGED", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamCostCalibrationRepositoryCreateTaskRejectsAccountOutsideTargetGroup(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamCostCalibrationRepository(db)
	ctx := context.Background()
	task := &service.UpstreamCostCalibrationTask{
		Name:          "calibration",
		Enabled:       true,
		TargetGroupID: 7,
		Model:         "claude-sonnet-4-20250514",
		AdapterType:   service.UpstreamCostCalibrationAdapterManual,
		Unit:          "credit",
		TestPrompt:    "ping",
		SampleCount:   1,
		PriorityStart: 10,
		PriorityStep:  10,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO upstream_cost_calibration_tasks").
		WithArgs(
			task.Name,
			task.Enabled,
			task.TargetGroupID,
			task.Model,
			task.AdapterType,
			task.Unit,
			task.TestPrompt,
			task.SampleCount,
			task.PriorityStart,
			task.PriorityStep,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectExec("INSERT INTO upstream_cost_calibration_task_accounts").
		WithArgs(int64(9), int64(101), int64(7), "{}").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := repo.CreateTask(ctx, task, []service.UpstreamCostCalibrationAccount{
		{AccountID: 101, AdapterConfig: map[string]any{}},
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "CALIBRATION_ACCOUNT_NOT_IN_GROUP", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectCalibrationRunDetail(mock sqlmock.Sqlmock, now time.Time, applied bool) {
	mock.ExpectQuery("SELECT id, task_id, target_group_id, status, total_accounts").
		WithArgs(int64(9), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "target_group_id", "status", "total_accounts", "valid_accounts", "invalid_accounts",
			"suggestion_count", "applied", "applied_by", "applied_at", "error_message",
			"started_at", "finished_at", "created_at",
		}).AddRow(
			int64(12), int64(9), int64(7), service.UpstreamCostCalibrationRunStatusSuccess,
			2, 2, 0, 2, applied, int64(66), now, nil, now, now, now,
		))
	mock.ExpectQuery("SELECT id, run_id, task_id, account_id, account_name").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "run_id", "task_id", "account_id", "account_name", "account_platform",
			"current_priority", "before_balance", "after_balance", "cost_delta", "unit",
			"test_status", "latency_ms", "valid", "rank", "suggested_priority",
			"error_message", "created_at",
		}))
	mock.ExpectQuery("SELECT id, run_id, task_id, account_id, old_priority").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "run_id", "task_id", "account_id", "old_priority", "new_priority",
			"reason", "applied", "applied_by", "applied_at", "created_at",
		}).
			AddRow(int64(1), int64(12), int64(9), int64(101), 30, 10, "rank 1", applied, int64(66), now, now).
			AddRow(int64(2), int64(12), int64(9), int64(102), 40, 20, "rank 2", applied, int64(66), now, now))
}
