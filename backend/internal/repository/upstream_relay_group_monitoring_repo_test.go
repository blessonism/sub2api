package repository

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func testIntPtr(value int) *int {
	return &value
}

func testBoolPtr(value bool) *bool {
	return &value
}

func TestRelayUsageHistoryWhereHidesZeroUsageByDefault(t *testing.T) {
	where, args := relayUsageHistoryWhere(service.UpstreamRelayUsageHistoryListFilters{})

	require.Empty(t, args)
	require.Contains(t, where, "(h.actual_cost > 0 OR h.total_tokens > 0)")
}

func TestRelayUsageHistoryWhereCanIncludeZeroUsage(t *testing.T) {
	where, args := relayUsageHistoryWhere(service.UpstreamRelayUsageHistoryListFilters{
		StartDate:        "2026-06-28",
		EndDate:          "2026-06-29",
		IncludeZeroUsage: true,
	})

	require.Len(t, args, 2)
	require.NotContains(t, where, "h.actual_cost > 0")
	require.NotContains(t, where, "h.total_tokens > 0")
	require.True(t, strings.Contains(where, "h.usage_date >= $1::date"))
	require.True(t, strings.Contains(where, "h.usage_date <= $2::date"))
}

func TestUpstreamRelayRepositoryCreateConnectorPersistsPasswordLoginFields(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	createdBy := int64(77)
	connector := &service.UpstreamRelayConnector{
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              service.UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc-access",
		RefreshTokenEncrypted: "enc-refresh",
		LoginEmailEncrypted:   "enc-admin@example.com",
		Status:                service.UpstreamRelayConnectorStatusInvalid,
		CreatedBy:             createdBy,
	}

	mock.ExpectQuery("INSERT INTO upstream_relay_connectors").
		WithArgs("relay", "https://relay.example.com", service.UpstreamRelayAuthModePasswordLogin, "enc-access", "enc-refresh", "enc-admin@example.com", nil, nil, service.UpstreamRelayConnectorStatusInvalid, createdBy).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	expectUpstreamRelayConnectorGet(mock, 42, now, service.UpstreamRelayConnector{
		ID:                    42,
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              service.UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc-access",
		RefreshTokenEncrypted: "enc-refresh",
		LoginEmailEncrypted:   "enc-admin@example.com",
		Status:                service.UpstreamRelayConnectorStatusInvalid,
		CredentialVersion:     1,
		CreatedBy:             createdBy,
	})

	got, err := repo.CreateConnector(ctx, connector)

	require.NoError(t, err)
	require.Equal(t, "enc-refresh", got.RefreshTokenEncrypted)
	require.Equal(t, "enc-admin@example.com", got.LoginEmailEncrypted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryUpdateConnectorIncrementsCredentialVersionAndPersistsNewFields(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	connector := &service.UpstreamRelayConnector{
		ID:                    42,
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              service.UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc-access",
		RefreshTokenEncrypted: "enc-refresh",
		LoginEmailEncrypted:   "enc-admin@example.com",
		Status:                service.UpstreamRelayConnectorStatusInvalid,
	}

	mock.ExpectExec("credential_version=credential_version \\+ 1").
		WithArgs(int64(42), "relay", "https://relay.example.com", service.UpstreamRelayAuthModePasswordLogin, "enc-access", "enc-refresh", "enc-admin@example.com", nil, nil, service.UpstreamRelayConnectorStatusInvalid).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectUpstreamRelayConnectorGet(mock, 42, now, service.UpstreamRelayConnector{
		ID:                    42,
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              service.UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc-access",
		RefreshTokenEncrypted: "enc-refresh",
		LoginEmailEncrypted:   "enc-admin@example.com",
		Status:                service.UpstreamRelayConnectorStatusInvalid,
		CredentialVersion:     2,
		CreatedBy:             77,
	})

	got, err := repo.UpdateConnector(ctx, connector, true)

	require.NoError(t, err)
	require.Equal(t, int64(2), got.CredentialVersion)
	require.Equal(t, "enc-refresh", got.RefreshTokenEncrypted)
	require.Equal(t, "enc-admin@example.com", got.LoginEmailEncrypted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRelayCandidateSelectUsesRemoteSnapshotUsage(t *testing.T) {
	query := relayCandidateSelect()

	require.NotContains(t, query, "upstream_account_balance")
	require.NotContains(t, query, "FROM usage_logs")
	require.Contains(t, query, "s.today_actual_cost, s.today_total_tokens, s.today_usage_checked_at")
	require.Contains(t, query, "LEFT JOIN upstream_relay_group_rate_snapshots s ON s.connector_id = c.connector_id AND s.upstream_group_id = c.upstream_group_id")
}

func TestUpstreamRelayRepositoryUpsertUsageHistoryUsesDailyConnectorGroupConflict(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("ON CONFLICT \\(usage_date, connector_id, upstream_group_id\\) DO UPDATE SET").
		WithArgs("2026-06-29", int64(7), "team-a", "Team A", "openai", 0.0, int64(0), checkedAt, false).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.UpsertUsageHistory(ctx, []service.UpstreamRelayGroupUsageHistoryUpsert{
		{
			UsageDate:       "2026-06-29",
			ConnectorID:     7,
			UpstreamGroupID: "team-a",
			GroupName:       "Team A",
			Platform:        "openai",
			ActualCost:      0,
			TotalTokens:     0,
			CheckedAt:       checkedAt,
		},
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryListCandidatesScansRemoteTodayUsage(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM upstream_relay_candidates c WHERE c\\.deleted_at IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("FROM upstream_relay_candidates c").
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "connector_id", "connector_name", "connector_status", "account_id",
			"today_actual_cost", "today_total_tokens", "today_usage_checked_at",
			"account_name", "account_platform", "account_schedulable", "account_gate_active", "upstream_group_id", "upstream_group_name",
			"upstream_api_key_id", "upstream_api_key_name", "upstream_api_key_masked",
			"probe_model", "probe_protocol", "priority", "enabled", "notes",
			"last_probe_result_id", "created_by", "created_at", "updated_at",
			"probe_id", "probe_success", "latency_ms", "http_status", "error_class", "error_message", "probed_at",
			"snapshot_id", "default_rate_multiplier", "override_rate_multiplier", "final_rate_multiplier",
			"snapshot_today_actual_cost", "snapshot_today_total_tokens", "snapshot_today_usage_checked_at",
			"snapshot_platform", "snapshot_status", "snapshot_source", "last_seen_at",
		}).AddRow(
			int64(9), int64(2), "relay", service.UpstreamRelayConnectorStatusActive, int64(101),
			1.2345, int64(12345), now,
			"account-a", "openai", true, false, "upstream-cheap", "Upstream Cheap",
			int64(855), "特惠", "sk-***",
			"gpt-4o-mini", service.MonitorAPIModeChatCompletions, 10, true, "", nil, int64(77), now, now,
			nil, nil, nil, nil, "", "", nil,
			nil, nil, nil, nil,
			nil, nil, nil,
			"", "", "", nil,
		))
	mock.ExpectQuery("FROM upstream_relay_candidate_health_snapshots").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"candidate_id", "probe_count", "success_count", "success_rate",
			"avg_latency_ms", "p95_latency_ms", "consecutive_successes", "consecutive_failures",
			"last_error_class", "last_success_at", "window_minutes", "sample_size", "calculated_at",
		}).AddRow(
			int64(9), 3, 2, 0.666667,
			90, 120, 0, 1,
			"rate_limited", now.Add(-time.Hour), 1440, 3, now,
		))
	mock.ExpectQuery("FROM upstream_relay_usage_delta_samples").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "candidate_id", "probe_result_id", "model", "status",
			"before_cost", "before_actual_cost", "after_cost", "after_actual_cost",
			"cost_delta", "actual_cost_delta", "derived_rate_multiplier",
			"unreliable_reason", "sampled_at",
		}))

	candidates, page, err := repo.ListCandidates(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, service.UpstreamRelayCandidateListFilters{})

	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].TodayActualCost)
	require.Equal(t, 1.2345, *candidates[0].TodayActualCost)
	require.NotNil(t, candidates[0].TodayTotalTokens)
	require.Equal(t, int64(12345), *candidates[0].TodayTotalTokens)
	require.NotNil(t, candidates[0].TodayUsageCheckedAt)
	require.NotNil(t, candidates[0].CurrentPriority)
	require.Equal(t, 10, *candidates[0].CurrentPriority)
	require.NotNil(t, candidates[0].UpstreamAPIKeyID)
	require.Equal(t, int64(855), *candidates[0].UpstreamAPIKeyID)
	require.Equal(t, "特惠", candidates[0].UpstreamAPIKeyName)
	require.Equal(t, "sk-***", candidates[0].UpstreamAPIKeyMasked)
	require.NotNil(t, candidates[0].Health)
	require.Equal(t, 3, candidates[0].Health.SampleSize)
	require.Equal(t, now, candidates[0].Health.CalculatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryUpdateSnapshotTodayUsageDoesNotRebuildSnapshots(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE upstream_relay_group_rate_snapshots").
		WithArgs(int64(42), &checkedAt).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE upstream_relay_group_rate_snapshots").
		WithArgs(int64(42), "g1", 1.25, int64(1234), &checkedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateSnapshotTodayUsage(ctx, 42, map[string]service.UpstreamRelayGroupTodayUsage{
		"g1": {ActualCost: 1.25, TotalTokens: 1234},
	}, &checkedAt, true)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryUpdateSnapshotTodayUsagePartialKeepsUnknownSnapshots(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE upstream_relay_group_rate_snapshots").
		WithArgs(int64(42), "g1", 1.25, int64(1234), &checkedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateSnapshotTodayUsage(ctx, 42, map[string]service.UpstreamRelayGroupTodayUsage{
		"g1": {ActualCost: 1.25, TotalTokens: 1234},
	}, &checkedAt, false)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryListCandidateUsageBindings(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("(?s)FROM upstream_relay_candidates.*deleted_at IS NULL").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "connector_id", "account_id", "upstream_group_id", "upstream_api_key_id", "upstream_api_key_name"}).
			AddRow(int64(1), int64(42), int64(10), "g1", int64(855), "特惠").
			AddRow(int64(2), int64(42), int64(11), "g1", int64(856), "稳定"))

	bindings, err := repo.ListCandidateUsageBindings(ctx, 42)

	require.NoError(t, err)
	require.Len(t, bindings, 2)
	require.Equal(t, int64(10), bindings[0].AccountID)
	require.Equal(t, "g1", bindings[0].UpstreamGroupID)
	require.Equal(t, int64(855), bindings[0].UpstreamAPIKeyID)
	require.Equal(t, "特惠", bindings[0].UpstreamAPIKeyName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryListCandidateUsageBindingsIncludesDisabledBindings(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("(?s)WHERE connector_id=\\$1\\s+AND deleted_at IS NULL\\s+ORDER BY id ASC").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "connector_id", "account_id", "upstream_group_id", "upstream_api_key_id", "upstream_api_key_name"}).
			AddRow(int64(1), int64(42), int64(10), "g1", int64(855), "特惠"))

	bindings, err := repo.ListCandidateUsageBindings(ctx, 42)

	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, int64(10), bindings[0].AccountID)
	require.Equal(t, int64(855), bindings[0].UpstreamAPIKeyID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryCreateCandidateValidatesConnectorAndAccount(t *testing.T) {
	t.Run("missing connector returns bad request", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := NewUpstreamRelayRepository(db)
		ctx := context.Background()

		mock.ExpectQuery("FROM upstream_relay_connectors").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		_, err := repo.CreateCandidate(ctx, &service.UpstreamRelayCandidate{
			ConnectorID:     42,
			AccountID:       101,
			UpstreamGroupID: "upstream-cheap",
			ProbeModel:      "gpt-4o-mini",
			ProbeProtocol:   service.MonitorAPIModeChatCompletions,
			Enabled:         true,
		})

		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Equal(t, "UPSTREAM_RELAY_CANDIDATE_CONNECTOR_MISSING", infraerrors.Reason(err))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing account returns bad request", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := NewUpstreamRelayRepository(db)
		ctx := context.Background()

		mock.ExpectQuery("FROM upstream_relay_connectors").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("FROM accounts").
			WithArgs(int64(101)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		_, err := repo.CreateCandidate(ctx, &service.UpstreamRelayCandidate{
			ConnectorID:     42,
			AccountID:       101,
			UpstreamGroupID: "upstream-cheap",
			ProbeModel:      "gpt-4o-mini",
			ProbeProtocol:   service.MonitorAPIModeChatCompletions,
			Enabled:         true,
		})

		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Equal(t, "UPSTREAM_RELAY_CANDIDATE_ACCOUNT_MISSING", infraerrors.Reason(err))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpstreamRelayRepositoryUpdateCandidateValidatesBinding(t *testing.T) {
	t.Run("missing connector returns bad request", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := NewUpstreamRelayRepository(db)
		ctx := context.Background()

		mock.ExpectQuery("FROM upstream_relay_connectors").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		_, err := repo.UpdateCandidate(ctx, &service.UpstreamRelayCandidate{
			ID:              9,
			ConnectorID:     42,
			AccountID:       101,
			UpstreamGroupID: "upstream-cheap",
			ProbeModel:      "gpt-4o-mini",
			ProbeProtocol:   service.MonitorAPIModeChatCompletions,
			Enabled:         true,
		})

		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Equal(t, "UPSTREAM_RELAY_CANDIDATE_CONNECTOR_MISSING", infraerrors.Reason(err))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing account returns bad request", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := NewUpstreamRelayRepository(db)
		ctx := context.Background()

		mock.ExpectQuery("FROM upstream_relay_connectors").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("FROM accounts").
			WithArgs(int64(101)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		_, err := repo.UpdateCandidate(ctx, &service.UpstreamRelayCandidate{
			ID:              9,
			ConnectorID:     42,
			AccountID:       101,
			UpstreamGroupID: "upstream-cheap",
			ProbeModel:      "gpt-4o-mini",
			ProbeProtocol:   service.MonitorAPIModeChatCompletions,
			Enabled:         true,
		})

		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Equal(t, "UPSTREAM_RELAY_CANDIDATE_ACCOUNT_MISSING", infraerrors.Reason(err))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpstreamRelayRepositoryCreateRecommendationRunPersistsPhase2ReasonFields(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	oldPriority := 50

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO upstream_relay_recommendation_runs").
		WithArgs(service.UpstreamRelayRunStatusSuccess, 1, 1, nil, int64(88)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(44)))
	mock.ExpectExec("INSERT INTO upstream_relay_recommendation_suggestions").
		WithArgs(
			int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(9), int64(10), int64(101), "cheap-upstream",
			oldPriority, 10, nil, nil, 0.75, "healthy", "rate_health_priority",
			"high", "最近3次成功率 100%，连续成功 3，连续失败 0，p95 80ms",
			service.UpstreamRelayRateSourceOverride, "结构化建议原因",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetailSingle(mock, now)

	run, err := repo.CreateRecommendationRun(ctx, service.UpstreamRelayRecommendationRun{
		Status:          service.UpstreamRelayRunStatusSuccess,
		TotalCandidates: 1,
		SuggestionCount: 1,
		CreatedBy:       88,
	}, []service.UpstreamRelayRecommendationSuggestion{
		{
			CandidateID:         9,
			ConnectorID:         10,
			AccountID:           101,
			UpstreamGroupID:     "cheap-upstream",
			OldPriority:         &oldPriority,
			NewPriority:         testIntPtr(10),
			FinalRateMultiplier: 0.75,
			HealthStatus:        "healthy",
			ReasonCode:          "rate_health_priority",
			Confidence:          "high",
			HealthSummary:       "最近3次成功率 100%，连续成功 3，连续失败 0，p95 80ms",
			RateSource:          service.UpstreamRelayRateSourceOverride,
			Reason:              "结构化建议原因",
		},
	})

	require.NoError(t, err)
	require.Len(t, run.Suggestions, 1)
	require.Equal(t, "rate_health_priority", run.Suggestions[0].ReasonCode)
	require.Equal(t, "high", run.Suggestions[0].Confidence)
	require.Equal(t, service.UpstreamRelayRateSourceOverride, run.Suggestions[0].RateSource)
	require.Equal(t, "结构化建议原因", run.Suggestions[0].Reason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryListRecommendationRunsFiltersSuggestions(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	hasSuggestions := true
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM upstream_relay_recommendation_runs WHERE 1=1 AND suggestion_count > 0").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(21)))
	mock.ExpectQuery("FROM upstream_relay_recommendation_runs").
		WithArgs(20, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "closed", "closed_by", "closed_at", "error_message", "created_by", "created_at",
		}).AddRow(int64(55), service.UpstreamRelayRunStatusSuccess, 3, 2, false, nil, nil, false, nil, nil, "", int64(88), now))

	runs, pageResult, err := repo.ListRecommendationRuns(ctx, pagination.PaginationParams{Page: 2, PageSize: 20}, service.UpstreamRelayRecommendationRunListFilters{HasSuggestions: &hasSuggestions})

	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, int64(55), runs[0].ID)
	require.Equal(t, int64(21), pageResult.Total)
	require.Equal(t, 2, pageResult.Page)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryUpsertsRecommendationPolicy(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO upstream_relay_recommendation_policy").
		WithArgs(60, 120, 15, 0.8, 5, false, 5, 5, sqlmock.AnyArg(), false, 0.0, false, 0, false, int64(88)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT snapshot_freshness_minutes").
		WillReturnRows(sqlmock.NewRows([]string{
			"snapshot_freshness_minutes", "usage_delta_freshness_minutes", "probe_freshness_minutes",
			"min_success_rate", "min_sample_size", "exclude_consecutive_failures",
			"priority_start", "priority_step", "sort_fields",
			"pause_rate_gap_enabled", "pause_rate_gap_threshold",
			"pause_consecutive_failures_enabled", "pause_consecutive_failures_threshold",
			"pause_success_rate_enabled", "updated_by", "created_at", "updated_at",
		}).AddRow(60, 120, 15, 0.8, 5, false, 5, 5, pq.Array([]string{
			service.UpstreamRelaySortSuccessRateDesc,
			service.UpstreamRelaySortRateAsc,
		}), false, 0.0, false, 0.0, false, int64(88), now, now))

	policy, err := repo.UpsertRecommendationPolicy(ctx, service.UpstreamRelayRecommendationPolicy{
		SnapshotFreshnessMinutes:   60,
		UsageDeltaFreshnessMinutes: 120,
		ProbeFreshnessMinutes:      15,
		MinSuccessRate:             0.8,
		MinSampleSize:              5,
		ExcludeConsecutiveFailures: false,
		PriorityStart:              5,
		PriorityStep:               5,
		SortFields: []string{
			service.UpstreamRelaySortSuccessRateDesc,
			service.UpstreamRelaySortRateAsc,
		},
	}, 88)

	require.NoError(t, err)
	require.Equal(t, 60, policy.SnapshotFreshnessMinutes)
	require.Equal(t, []string{service.UpstreamRelaySortSuccessRateDesc, service.UpstreamRelaySortRateAsc}, policy.SortFields)
	require.Equal(t, int64(88), policy.UpdatedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryUpsertsMonitoringPolicy(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO upstream_relay_monitoring_policy").
		WithArgs(true, 60, true, 15, true, 30, true, 8, 40, "high", true, 5, 3, 4, int64(88)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT auto_sync_enabled").
		WillReturnRows(sqlmock.NewRows([]string{
			"auto_sync_enabled", "sync_interval_minutes", "auto_probe_enabled", "probe_interval_minutes",
			"auto_recommendation_enabled", "recommendation_interval_minutes", "auto_apply_recommendations_enabled",
			"max_auto_apply_suggestions", "max_auto_apply_priority_delta", "min_auto_apply_confidence",
			"allow_auto_apply_degraded_health",
			"failure_retry_interval_minutes", "sync_concurrency", "probe_concurrency",
			"updated_by", "created_at", "updated_at",
		}).AddRow(true, 60, true, 15, true, 30, true, 8, 40, "high", true, 5, 3, 4, int64(88), now, now))

	policy, err := repo.UpsertMonitoringPolicy(ctx, service.UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:                 true,
		SyncIntervalMinutes:             60,
		AutoProbeEnabled:                true,
		ProbeIntervalMinutes:            15,
		AutoRecommendationEnabled:       true,
		RecommendationIntervalMinutes:   30,
		AutoApplyRecommendationsEnabled: true,
		MaxAutoApplySuggestions:         8,
		MaxAutoApplyPriorityDelta:       40,
		MinAutoApplyConfidence:          "high",
		AllowAutoApplyDegradedHealth:    true,
		FailureRetryIntervalMinutes:     5,
		SyncConcurrency:                 3,
		ProbeConcurrency:                4,
	}, 88)

	require.NoError(t, err)
	require.True(t, policy.AutoSyncEnabled)
	require.True(t, policy.AutoProbeEnabled)
	require.True(t, policy.AutoRecommendationEnabled)
	require.True(t, policy.AutoApplyRecommendationsEnabled)
	require.Equal(t, 60, policy.SyncIntervalMinutes)
	require.Equal(t, 15, policy.ProbeIntervalMinutes)
	require.Equal(t, 30, policy.RecommendationIntervalMinutes)
	require.Equal(t, 8, policy.MaxAutoApplySuggestions)
	require.Equal(t, 40, policy.MaxAutoApplyPriorityDelta)
	require.Equal(t, "high", policy.MinAutoApplyConfidence)
	require.True(t, policy.AllowAutoApplyDegradedHealth)
	require.Equal(t, int64(88), policy.UpdatedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunUpdatesPriorityAndAudit(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(1), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(1), int64(101), 50, 10, nil, nil, "rate_health_priority", "rate").
			AddRow(int64(2), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(2), int64(102), 60, 20, nil, nil, "rate_health_priority", "rate"))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(102), 20, int64(60)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetail(mock, now, true)

	run, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.NoError(t, err)
	require.True(t, run.Applied)
	require.Len(t, run.Suggestions, 2)
	require.True(t, run.Suggestions[0].Applied)
	require.Equal(t, int64(99), *run.Suggestions[0].AppliedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunUsesSuggestionSnapshotWithoutLiveCandidateState(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(99), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(99), int64(101), 50, 10, nil, nil, "rate_health_priority", "rate"))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetail(mock, now, true)

	run, err := repo.ApplyRecommendationRun(ctx, 44, 77)

	require.NoError(t, err)
	require.True(t, run.Applied)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsStalePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(1), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(1), int64(101), 50, 10, nil, nil, "rate_health_priority", "rate"))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT priority FROM accounts").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"priority"}).AddRow(80))
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "UPSTREAM_RELAY_RECOMMENDATION_STALE", infraerrors.Reason(err))
	require.Contains(t, err.Error(), "账号 #101 的 priority 已变化")
	require.Contains(t, err.Error(), "生成建议时为 50，当前为 80，建议值为 10")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunAppliesAccountPause(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(7), service.UpstreamRelaySuggestionActionAccountPause, int64(9), int64(101), 50, nil, true, false, "account_gate_latest_probe_failed", "最近探测失败；建议暂停账号承接"))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), false, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO upstream_relay_account_gate_states").
		WithArgs(int64(101), true, "account_gate_latest_probe_failed", "最近探测失败；建议暂停账号承接", int64(44), int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetailWithSuggestions(mock, now, true, newRelaySuggestionRows().AddRow(
		int64(7), int64(44), service.UpstreamRelaySuggestionActionAccountPause, int64(9), int64(10), "relay", int64(101), "account-a",
		"cheap-upstream", "cheap upstream", 50, nil, true, false, 0.75, "failed", "account_gate_latest_probe_failed", "medium",
		"最近探测失败", service.UpstreamRelayRateSourceOverride, "最近探测失败；建议暂停账号承接", true, int64(99), now, now,
	))

	run, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.NoError(t, err)
	require.True(t, run.Applied)
	require.Len(t, run.Suggestions, 1)
	require.Equal(t, service.UpstreamRelaySuggestionActionAccountPause, run.Suggestions[0].ActionType)
	require.Equal(t, testBoolPtr(true), run.Suggestions[0].OldSchedulable)
	require.Equal(t, testBoolPtr(false), run.Suggestions[0].NewSchedulable)
	require.Nil(t, run.Suggestions[0].NewPriority)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunAppliesAccountResumeOnlyWithGateState(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(8), service.UpstreamRelaySuggestionActionAccountResume, int64(9), int64(101), 50, nil, false, true, "account_gate_recovered", "建议恢复账号承接"))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM upstream_relay_account_gate_states").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), true, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_account_gate_states").
		WithArgs(int64(101), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetailWithSuggestions(mock, now, true, newRelaySuggestionRows().AddRow(
		int64(8), int64(44), service.UpstreamRelaySuggestionActionAccountResume, int64(9), int64(10), "relay", int64(101), "account-a",
		"cheap-upstream", "cheap upstream", 50, nil, false, true, 0.75, "healthy", "account_gate_recovered", "high",
		"最近3次成功率 100%", service.UpstreamRelayRateSourceOverride, "建议恢复账号承接", true, int64(99), now, now,
	))

	run, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.NoError(t, err)
	require.True(t, run.Applied)
	require.Len(t, run.Suggestions, 1)
	require.Equal(t, service.UpstreamRelaySuggestionActionAccountResume, run.Suggestions[0].ActionType)
	require.Equal(t, testBoolPtr(false), run.Suggestions[0].OldSchedulable)
	require.Equal(t, testBoolPtr(true), run.Suggestions[0].NewSchedulable)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsResumeWithoutGateState(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(8), service.UpstreamRelaySuggestionActionAccountResume, int64(9), int64(101), 50, nil, false, true, "account_gate_recovered", "建议恢复账号承接"))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM upstream_relay_account_gate_states").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "UPSTREAM_RELAY_ACCOUNT_GATE_STATE_MISSING", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsInvalidAccountGateDirection(t *testing.T) {
	cases := []struct {
		name       string
		actionType string
		oldValue   bool
		newValue   bool
		wantMsg    string
	}{
		{
			name:       "pause must turn schedulable off",
			actionType: service.UpstreamRelaySuggestionActionAccountPause,
			oldValue:   false,
			newValue:   true,
			wantMsg:    "pause suggestion must change schedulable from true to false",
		},
		{
			name:       "resume must turn schedulable on",
			actionType: service.UpstreamRelaySuggestionActionAccountResume,
			oldValue:   true,
			newValue:   false,
			wantMsg:    "resume suggestion must change schedulable from false to true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := NewUpstreamRelayRepository(db)
			ctx := context.Background()

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
				WithArgs(int64(44)).
				WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
					AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
			mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
				WithArgs(int64(44)).
				WillReturnRows(newRelayPendingSuggestionRows().
					AddRow(int64(8), tc.actionType, int64(9), int64(101), 50, nil, tc.oldValue, tc.newValue, "account_gate_invalid", "invalid account gate direction"))
			mock.ExpectRollback()

			_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, "UPSTREAM_RELAY_INVALID_RECOMMENDATION_SUGGESTION", infraerrors.Reason(err))
			require.Contains(t, err.Error(), tc.wantMsg)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsStaleSchedulable(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows().
			AddRow(int64(7), service.UpstreamRelaySuggestionActionAccountPause, int64(9), int64(101), 50, nil, true, false, "account_gate_latest_probe_failed", "最近探测失败；建议暂停账号承接"))
	mock.ExpectExec("UPDATE accounts").
		WithArgs(int64(101), false, true).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT schedulable FROM accounts").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"schedulable"}).AddRow(false))
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "UPSTREAM_RELAY_RECOMMENDATION_STALE", infraerrors.Reason(err))
	require.Contains(t, err.Error(), "账号 #101 的调度状态已变化")
	require.Contains(t, err.Error(), "生成建议时为 可调度，当前为 暂停，建议值为 暂停")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsEmptyPendingSuggestions(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectQuery("SELECT id, COALESCE\\(action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelayPendingSuggestionRows())
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.ErrorIs(t, err, service.ErrUpstreamRelayNoPendingSuggestions)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryCloseRecommendationRunPreservesHistory(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, false))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetailClosed(mock, now)

	run, err := repo.CloseRecommendationRun(ctx, 44, 99)

	require.NoError(t, err)
	require.True(t, run.Closed)
	require.False(t, run.Applied)
	require.Len(t, run.Suggestions, 2)
	require.False(t, run.Suggestions[0].Applied)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryRestoreRecommendationRunReopensClosedRun(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false, true))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectUpstreamRelayRunDetail(mock, now, false)

	run, err := repo.RestoreRecommendationRun(ctx, 44, 99)

	require.NoError(t, err)
	require.False(t, run.Closed)
	require.False(t, run.Applied)
	require.Len(t, run.Suggestions, 2)
	require.False(t, run.Suggestions[0].Applied)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryRestoreRecommendationRunRejectsInvalidStates(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		applied    bool
		closed     bool
		wantCode   int
		wantReason string
	}{
		{
			name:       "non success",
			status:     service.UpstreamRelayRunStatusFailed,
			applied:    false,
			closed:     true,
			wantCode:   http.StatusBadRequest,
			wantReason: "UPSTREAM_RELAY_RUN_NOT_SUCCESS",
		},
		{
			name:       "already applied",
			status:     service.UpstreamRelayRunStatusSuccess,
			applied:    true,
			closed:     true,
			wantCode:   http.StatusConflict,
			wantReason: "UPSTREAM_RELAY_RUN_ALREADY_APPLIED",
		},
		{
			name:       "not closed",
			status:     service.UpstreamRelayRunStatusSuccess,
			applied:    false,
			closed:     false,
			wantCode:   http.StatusConflict,
			wantReason: "UPSTREAM_RELAY_RUN_NOT_CLOSED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := NewUpstreamRelayRepository(db)
			ctx := context.Background()

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT status, applied, closed FROM upstream_relay_recommendation_runs").
				WithArgs(int64(44)).
				WillReturnRows(sqlmock.NewRows([]string{"status", "applied", "closed"}).
					AddRow(tt.status, tt.applied, tt.closed))
			mock.ExpectRollback()

			_, err := repo.RestoreRecommendationRun(ctx, 44, 99)

			require.Error(t, err)
			require.Equal(t, tt.wantCode, infraerrors.Code(err))
			require.Equal(t, tt.wantReason, infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpstreamRelayRepositoryInsertUsageDeltaSamplePersistsAndReadsLatest(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	probeID := int64(88)
	beforeCost := 10.0
	beforeActualCost := 2.0
	afterCost := 14.0
	afterActualCost := 3.0
	costDelta := 4.0
	actualCostDelta := 1.0
	derivedRate := 4.0

	mock.ExpectQuery("INSERT INTO upstream_relay_usage_delta_samples").
		WithArgs(int64(9), probeID, "gpt-5.5", "reliable", beforeCost, beforeActualCost, afterCost, afterActualCost, costDelta, actualCostDelta, derivedRate, "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))
	mock.ExpectQuery("SELECT id, candidate_id, probe_result_id, model, status").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "candidate_id", "probe_result_id", "model", "status",
			"before_cost", "before_actual_cost", "after_cost", "after_actual_cost",
			"cost_delta", "actual_cost_delta", "derived_rate_multiplier",
			"unreliable_reason", "sampled_at",
		}).AddRow(
			int64(101), int64(9), probeID, "gpt-5.5", "reliable",
			beforeCost, beforeActualCost, afterCost, afterActualCost,
			costDelta, actualCostDelta, derivedRate,
			"", now,
		))

	got, err := repo.InsertUsageDeltaSample(ctx, service.UpstreamRelayUsageDeltaSample{
		CandidateID:           9,
		ProbeResultID:         &probeID,
		Model:                 "gpt-5.5",
		Status:                "reliable",
		BeforeCost:            &beforeCost,
		BeforeActualCost:      &beforeActualCost,
		AfterCost:             &afterCost,
		AfterActualCost:       &afterActualCost,
		CostDelta:             &costDelta,
		ActualCostDelta:       &actualCostDelta,
		DerivedRateMultiplier: &derivedRate,
	})

	require.NoError(t, err)
	require.Equal(t, int64(101), got.ID)
	require.Equal(t, int64(9), got.CandidateID)
	require.NotNil(t, got.DerivedRateMultiplier)
	require.Equal(t, derivedRate, *got.DerivedRateMultiplier)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayBuildRelayHealthComputesWindowStats(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	latencyFast := sqlNullInt64(40)
	latencySlow := sqlNullInt64(120)
	latencyMid := sqlNullInt64(80)
	health := buildRelayHealth([]relayProbeRow{
		{candidateID: 1, success: true, latency: latencyFast, probedAt: now},
		{candidateID: 1, success: true, latency: latencySlow, probedAt: now.Add(-time.Minute)},
		{candidateID: 1, success: false, latency: latencyMid, errorClass: "rate_limited", probedAt: now.Add(-2 * time.Minute)},
		{candidateID: 1, success: true, latency: latencyMid, probedAt: now.Add(-3 * time.Minute)},
	}, 30)

	require.Equal(t, 4, health.ProbeCount)
	require.Equal(t, 3, health.SuccessCount)
	require.InDelta(t, 0.75, health.SuccessRate, 0.0001)
	require.Equal(t, 2, health.ConsecutiveSuccesses)
	require.Equal(t, 0, health.ConsecutiveFailures)
	require.Equal(t, "rate_limited", health.LastErrorClass)
	require.NotNil(t, health.AvgLatencyMs)
	require.Equal(t, 80, *health.AvgLatencyMs)
	require.NotNil(t, health.P95LatencyMs)
	require.Equal(t, 120, *health.P95LatencyMs)
	require.NotNil(t, health.LastSuccessAt)
	require.Equal(t, now, *health.LastSuccessAt)
}

func TestUpstreamRelayRepositoryRefreshCandidateHealthSnapshotUpsertsSummary(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db).(*upstreamRelayRepository)
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("FROM upstream_relay_probe_results").
		WithArgs(sqlmock.AnyArg(), upstreamRelayHealthSnapshotWindowMinutes, upstreamRelayHealthSnapshotSampleLimit).
		WillReturnRows(sqlmock.NewRows([]string{"candidate_id", "success", "latency_ms", "error_class", "probed_at"}).
			AddRow(int64(9), true, 80, "", now).
			AddRow(int64(9), false, nil, "rate_limited", now.Add(-time.Minute)))
	mock.ExpectExec("INSERT INTO upstream_relay_candidate_health_snapshots").
		WithArgs(
			int64(9), 2, 1, 0.5, 80, 80,
			1, 0, "rate_limited", sqlmock.AnyArg(),
			upstreamRelayHealthSnapshotWindowMinutes, 2, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, repo.refreshCandidateHealthSnapshot(ctx, tx, 9))
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func sqlNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func expectUpstreamRelayConnectorGet(mock sqlmock.Sqlmock, id int64, now time.Time, connector service.UpstreamRelayConnector) {
	mock.ExpectQuery("SELECT id, name, base_url, auth_mode").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "base_url", "auth_mode", "bearer_token_encrypted",
			"refresh_token_encrypted", "login_email_encrypted", "cookie_encrypted", "user_agent_encrypted",
			"upstream_account_balance", "upstream_account_balance_checked_at",
			"status", "credential_version", "last_verified_at", "last_synced_at", "last_error",
			"created_by", "created_at", "updated_at",
		}).AddRow(
			connector.ID, connector.Name, connector.BaseURL, connector.AuthMode, connector.BearerTokenEncrypted,
			connector.RefreshTokenEncrypted, connector.LoginEmailEncrypted, connector.CookieEncrypted, connector.UserAgentEncrypted,
			connector.UpstreamAccountBalance, connector.UpstreamAccountBalanceCheckedAt,
			connector.Status, connector.CredentialVersion, connector.LastVerifiedAt, connector.LastSyncedAt, connector.LastError,
			connector.CreatedBy, now, now,
		))
}

func expectUpstreamRelayRunDetail(mock sqlmock.Sqlmock, now time.Time, applied bool) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "closed", "closed_by", "closed_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 2, 2, applied, int64(99), now, false, nil, nil, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, COALESCE\\(s\\.action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelaySuggestionRows().
			AddRow(int64(1), int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(1), int64(10), "relay", int64(101), "account-a", "cheap", "cheap upstream", 50, 10, nil, nil, 0.8, "success", "rate_health_priority", "high", "成功率 100%", "login_user_group_rates", "rate", applied, int64(99), now, now).
			AddRow(int64(2), int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(2), int64(10), "relay", int64(102), "account-b", "fast", "fast upstream", 60, 20, nil, nil, 1.1, "success", "rate_health_priority", "medium", "成功率 100%", "login_available_groups", "rate", applied, int64(99), now, now))
}

func expectUpstreamRelayRunDetailClosed(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "closed", "closed_by", "closed_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 2, 2, false, nil, nil, true, int64(99), now, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, COALESCE\\(s\\.action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelaySuggestionRows().
			AddRow(int64(1), int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(1), int64(10), "relay", int64(101), "account-a", "cheap", "cheap upstream", 50, 10, nil, nil, 0.8, "success", "rate_health_priority", "high", "成功率 100%", "login_user_group_rates", "rate", false, nil, nil, now).
			AddRow(int64(2), int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(2), int64(10), "relay", int64(102), "account-b", "fast", "fast upstream", 60, 20, nil, nil, 1.1, "success", "rate_health_priority", "medium", "成功率 100%", "login_available_groups", "rate", false, nil, nil, now))
}

func expectUpstreamRelayRunDetailSingle(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "closed", "closed_by", "closed_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 1, 1, false, nil, nil, false, nil, nil, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, COALESCE\\(s\\.action_type").
		WithArgs(int64(44)).
		WillReturnRows(newRelaySuggestionRows().AddRow(
			int64(1), int64(44), service.UpstreamRelaySuggestionActionPriorityUpdate, int64(9), int64(10), "relay", int64(101), "account-a",
			"cheap-upstream", "cheap upstream", 50, 10, nil, nil, 0.75, "healthy", "rate_health_priority", "high",
			"最近3次成功率 100%，连续成功 3，连续失败 0，p95 80ms",
			service.UpstreamRelayRateSourceOverride, "结构化建议原因", false, nil, nil, now,
		))
}

func expectUpstreamRelayRunDetailWithSuggestions(mock sqlmock.Sqlmock, now time.Time, applied bool, rows *sqlmock.Rows) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "closed", "closed_by", "closed_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 1, 1, applied, int64(99), now, false, nil, nil, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, COALESCE\\(s\\.action_type").
		WithArgs(int64(44)).
		WillReturnRows(rows)
}

func newRelaySuggestionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "run_id", "action_type", "candidate_id", "connector_id", "connector_name",
		"account_id", "account_name", "upstream_group_id", "upstream_group_name",
		"old_priority", "new_priority", "old_schedulable", "new_schedulable",
		"final_rate_multiplier", "health_status", "reason_code", "confidence",
		"health_summary", "rate_source", "reason", "applied", "applied_by",
		"applied_at", "created_at",
	})
}

func newRelayPendingSuggestionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "action_type", "candidate_id", "account_id",
		"old_priority", "new_priority", "old_schedulable", "new_schedulable",
		"reason_code", "reason",
	})
}
