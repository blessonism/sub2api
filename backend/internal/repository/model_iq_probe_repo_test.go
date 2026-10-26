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

func TestModelIQProbeGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &modelIQProbeRepository{db: db}
	columns := []string{"id", "account_id", "account_name", "model", "protocol", "status", "override_status", "error_category", "error_message", "expected_answer", "actual_answer", "latency_ms", "input_tokens", "output_tokens", "cache_read_tokens", "created_at"}
	getQuery := "SELECT " + modelIQProbeSelectColumns + " FROM model_iq_probe_results WHERE id = $1"

	mock.ExpectQuery(regexp.QuoteMeta(getQuery)).WithArgs(int64(7)).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(7, 42, "private", "gpt-5.2", "responses", "wrong_answer", nil, "", "", "21", "20", 100, nil, nil, nil, time.Now()))
	row, err := repo.GetByID(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.2", row.Model)
	require.Equal(t, "wrong_answer", row.ResolveEffectiveStatus())

	mock.ExpectQuery(regexp.QuoteMeta(getQuery)).WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows(columns))
	_, err = repo.GetByID(context.Background(), 99)
	require.ErrorIs(t, err, service.ErrModelIQProbeResultNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelIQProbeRandomPassAnswer(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &modelIQProbeRepository{db: db}
	sampleQuery := "SELECT actual_answer FROM model_iq_probe_results WHERE actual_answer <> '' AND id <> $1 AND COALESCE(override_status, status) = 'pass' ORDER BY (model = $2) DESC, RANDOM() LIMIT 1"

	mock.ExpectQuery(regexp.QuoteMeta(sampleQuery)).WithArgs(int64(7), "gpt-5.2").
		WillReturnRows(sqlmock.NewRows([]string{"actual_answer"}).AddRow("所以最少取出 21 个糖果。"))
	answer, ok, err := repo.RandomPassAnswer(context.Background(), "gpt-5.2", 7)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "所以最少取出 21 个糖果。", answer)

	// 无可用样本：ok=false 且不报错。
	mock.ExpectQuery(regexp.QuoteMeta(sampleQuery)).WithArgs(int64(7), "gpt-5.2").
		WillReturnRows(sqlmock.NewRows([]string{"actual_answer"}))
	answer, ok, err = repo.RandomPassAnswer(context.Background(), "gpt-5.2", 7)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, answer)

	mock.ExpectQuery(regexp.QuoteMeta(sampleQuery)).WithArgs(int64(7), "gpt-5.2").WillReturnError(sql.ErrConnDone)
	_, _, err = repo.RandomPassAnswer(context.Background(), "gpt-5.2", 7)
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, mock.ExpectationsWereMet())
}
