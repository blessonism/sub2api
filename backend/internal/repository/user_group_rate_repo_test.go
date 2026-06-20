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
		{UserID: 1, RateMultiplier: 1.25},
		{UserID: 2, RateMultiplier: 1.50},
	})
	require.ErrorIs(t, err, upsertErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
