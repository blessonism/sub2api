//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryPreviewAllUserBalanceReduction(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUserRepositoryWithSQL(nil, db)
	queryPattern := regexp.QuoteMeta("COUNT(*) FILTER (WHERE balance <> GREATEST(ROUND(balance / $1::numeric, 8), 0))") +
		`.*` + regexp.QuoteMeta("WHERE deleted_at IS NULL")
	mock.ExpectQuery(queryPattern).
		WithArgs("2.5").
		WillReturnRows(sqlmock.NewRows([]string{"users", "affected", "current", "reduced"}).AddRow(3, 2, 30.0, 12.0))

	result, err := repo.PreviewAllUserBalanceReduction(context.Background(), "2.5")

	require.NoError(t, err)
	require.Equal(t, int64(3), result.UserCount)
	require.Equal(t, int64(2), result.AffectedUsers)
	require.InDelta(t, 18.0, result.ReductionTotal, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepositoryReduceAllUserBalancesUsesAtomicUpdateAndAudit(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUserRepositoryWithSQL(nil, db)
	queryPattern := regexp.QuoteMeta("WITH targets AS MATERIALIZED") +
		`.*` + regexp.QuoteMeta("WHERE deleted_at IS NULL") +
		`.*` + regexp.QuoteMeta("FOR UPDATE") +
		`.*` + regexp.QuoteMeta("SET balance = GREATEST(ROUND(t.old_balance / $1::numeric, 8), 0)") +
		`.*` + regexp.QuoteMeta("INSERT INTO redeem_codes") +
		`.*` + regexp.QuoteMeta("'admin_balance'")
	mock.ExpectQuery(queryPattern).
		WithArgs("2.5", "operation-1", "batch notes").
		WillReturnRows(sqlmock.NewRows([]string{"users", "affected", "current", "reduced", "user_ids"}).
			AddRow(3, 2, 30.0, 12.0, "{1,2}"))

	result, err := repo.ReduceAllUserBalances(context.Background(), "2.5", "operation-1", "batch notes")

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.UserIDs)
	require.Equal(t, int64(2), result.AffectedUsers)
	require.InDelta(t, 18.0, result.ReductionTotal, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
