package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpstreamRelayRepositoryDeleteRecommendationRunDeletesPendingRunAndSuggestions(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"applied"}).AddRow(false))
	mock.ExpectExec("DELETE FROM upstream_relay_recommendation_suggestions").
		WithArgs(int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.DeleteRecommendationRun(ctx, 44)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryDeleteRecommendationRunRejectsAppliedRun(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"applied"}).AddRow(true))
	mock.ExpectRollback()

	err := repo.DeleteRecommendationRun(ctx, 44)

	require.ErrorIs(t, err, service.ErrUpstreamRelayAppliedRunDelete)
	require.NoError(t, mock.ExpectationsWereMet())
}
