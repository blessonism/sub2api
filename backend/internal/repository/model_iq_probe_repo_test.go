package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestModelIQProbeSummaryQueryKeepsLatestPerTarget(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &modelIQProbeRepository{db: db}
	query := "WITH ranked AS (SELECT " + modelIQProbeSelectColumns + ", ROW_NUMBER() OVER (PARTITION BY model, account_id ORDER BY created_at DESC, id DESC) AS target_rank, ROW_NUMBER() OVER (PARTITION BY model ORDER BY created_at DESC, id DESC) AS model_rank FROM model_iq_probe_results WHERE created_at >= $1 AND (account_id, model) IN (SELECT * FROM unnest($3::bigint[], $4::text[]))) SELECT " + modelIQProbeSelectColumns + " FROM ranked WHERE target_rank = 1 OR model_rank <= $2 ORDER BY created_at DESC, id DESC"
	columns := []string{"id", "account_id", "account_name", "model", "protocol", "status", "override_status", "error_category", "error_message", "expected_answer", "actual_answer", "latency_ms", "input_tokens", "output_tokens", "cache_read_tokens", "created_at"}
	rows := sqlmock.NewRows(columns).AddRow(1, 42, "private", "gpt-5.4", "responses", "wrong_answer", nil, "", "", "21", "20", 100, nil, nil, nil, time.Now())
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(sqlmock.AnyArg(), 60, `{42}`, `{"gpt-5.4"}`).WillReturnRows(rows)
	results, err := repo.ListForSummary(context.Background(), []service.ModelIQProbeTargetRef{{AccountID: 42, Model: "gpt-5.4"}}, 60)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "wrong_answer", results[0].EffectiveStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelIQProbeSummaryQueryPropagatesFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &modelIQProbeRepository{db: db}
	mock.ExpectQuery("WITH ranked AS").WithArgs(sqlmock.AnyArg(), 60, `{42}`, `{"gpt-5.4"}`).WillReturnError(sql.ErrConnDone)
	_, err = repo.ListForSummary(context.Background(), []service.ModelIQProbeTargetRef{{AccountID: 42, Model: "gpt-5.4"}}, 60)
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelIQProbeSummaryModelsAndEmptyTargets(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &modelIQProbeRepository{db: db}
	models, err := repo.ListSummaryModels(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, models)
	rows, err := repo.ListForSummary(context.Background(), nil, 60)
	require.NoError(t, err)
	require.Empty(t, rows)
	mock.ExpectQuery("SELECT DISTINCT model FROM model_iq_probe_results").WithArgs(sqlmock.AnyArg(), `{42}`).WillReturnRows(sqlmock.NewRows([]string{"model"}).AddRow("gpt-5.4"))
	models, err = repo.ListSummaryModels(context.Background(), []int64{42})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.4"}, models)
	require.NoError(t, mock.ExpectationsWereMet())
}
