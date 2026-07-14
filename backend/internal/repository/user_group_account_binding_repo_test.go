package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUserGroupAccountBindingRepositoryGetAndReplace(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &userGroupAccountBindingRepository{db: db}

	mock.ExpectQuery(`SELECT account_ids, fallback_to_group`).
		WithArgs(int64(7), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"account_ids", "fallback_to_group"}).AddRow(pq.Array([]int64{2, 3}), true))
	binding, err := repo.GetByUserAndGroup(context.Background(), 7, 9)
	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, binding.AccountIDs)
	require.True(t, binding.FallbackToGroup)

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM user_group_account_bindings`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO user_group_account_bindings`).
		WithArgs(int64(7), int64(9), pq.Array([]int64{2, 3}), true, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err = repo.ReplaceByUserID(context.Background(), 7, map[int64]service.UserGroupAccountBinding{
		9: {AccountIDs: []int64{2, 3}, FallbackToGroup: true},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
