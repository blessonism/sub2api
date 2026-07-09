package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserGroupRateRepositorySyncGroupRateMultipliersRollsBackWhenUpsertFails(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserGroupRateRepository(db)
	upsertErr := errors.New("upsert failed")

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(10), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(int64(10), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(upsertErr)
	mock.ExpectRollback()

	err := repo.SyncGroupRateMultipliers(context.Background(), 10, []service.GroupRateMultiplierInput{
		{
			UserID:                   1,
			RateMultiplier:           ptrFloat64ForUserGroupRateRepoTest(1.25),
			VisibleRateMultiplier:    ptrFloat64ForUserGroupRateRepoTest(0.95),
			RateMultiplierSet:        true,
			VisibleRateMultiplierSet: true,
		},
		{
			UserID:            2,
			RateMultiplier:    ptrFloat64ForUserGroupRateRepoTest(1.50),
			RateMultiplierSet: true,
		},
	})
	require.ErrorIs(t, err, upsertErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGroupRateRepositorySyncGroupRateMultipliersSupportsExplicitNull(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserGroupRateRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(10), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(10), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.SyncGroupRateMultipliers(context.Background(), 10, []service.GroupRateMultiplierInput{
		{
			UserID:                   1,
			VisibleRateMultiplier:    nil,
			VisibleRateMultiplierSet: true,
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGroupRateRepositoryIsTokenUsageAutoRate(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserGroupRateRepository(db).(*userGroupRateRepository)

	mock.ExpectQuery("SELECT EXISTS \\([\\s\\S]*FROM token_usage_auto_assignments a[\\s\\S]*JOIN token_usage_auto_policies p ON p\\.id = a\\.policy_id AND p\\.enabled = TRUE[\\s\\S]*JOIN groups target_group ON target_group\\.id = a\\.target_group_id AND target_group\\.status = \\$4[\\s\\S]*a\\.manual_takeover = FALSE").
		WithArgs(int64(11), int64(7), 0.8, service.StatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	got, err := repo.IsTokenUsageAutoRate(context.Background(), 11, 7, 0.8)

	require.NoError(t, err)
	require.True(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func ptrFloat64ForUserGroupRateRepoTest(v float64) *float64 {
	return &v
}
