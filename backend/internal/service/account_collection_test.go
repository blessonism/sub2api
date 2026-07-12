package service

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAccountCollectionBatchMembersAddIsIncrementalAndIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT EXISTS .* FROM accounts`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS .* FROM account_collections`).WithArgs(int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`INSERT INTO account_collection_members`).WithArgs(int64(11), int64(21)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	service := NewAccountCollectionService(db)
	require.NoError(t, service.BatchMembers(context.Background(), []int64{11}, []int64{21}, "add"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountCollectionBatchMembersRejectsMissingCollection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT EXISTS .* FROM accounts`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS .* FROM account_collections`).WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	service := NewAccountCollectionService(db)
	err = service.BatchMembers(context.Background(), []int64{11}, []int64{99}, "remove")
	require.EqualError(t, err, "account collection 99 not found")
	require.NoError(t, mock.ExpectationsWereMet())
}
