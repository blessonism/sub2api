package repository

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

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
	mock.ExpectExec(`VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7,\$8,\$9,\$10,\$11,\$12,\$13,\$14,\$15,FALSE,NOW\(\)\)`).
		WithArgs(
			int64(44), int64(9), int64(10), int64(101), "cheap-upstream",
			int64(7), oldPriority, 10, 0.75, "healthy", "rate_health_priority",
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
			TargetGroupID:       7,
			OldPriority:         &oldPriority,
			NewPriority:         10,
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

func TestUpstreamRelayRepositoryApplyRecommendationRunUpdatesPriorityAndAudit(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false))
	mock.ExpectQuery("SELECT candidate_id, account_id, target_group_id, old_priority, new_priority").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"candidate_id", "account_id", "target_group_id", "old_priority", "new_priority"}).
			AddRow(int64(1), int64(101), int64(7), 50, 10).
			AddRow(int64(2), int64(101), int64(8), 60, 20))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(7), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(8), 20, int64(60)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountGroupsChanged, sqlmock.AnyArg(), nil, sqlmock.AnyArg()).
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
	mock.ExpectQuery("SELECT status, applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false))
	mock.ExpectQuery("SELECT candidate_id, account_id, target_group_id, old_priority, new_priority").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"candidate_id", "account_id", "target_group_id", "old_priority", "new_priority"}).
			AddRow(int64(99), int64(101), int64(7), 50, 10))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(7), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_suggestions").
		WithArgs(int64(44), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upstream_relay_recommendation_runs").
		WithArgs(int64(44), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountGroupsChanged, sqlmock.AnyArg(), nil, sqlmock.AnyArg()).
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
	mock.ExpectQuery("SELECT status, applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false))
	mock.ExpectQuery("SELECT candidate_id, account_id, target_group_id, old_priority, new_priority").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"candidate_id", "account_id", "target_group_id", "old_priority", "new_priority"}).
			AddRow(int64(1), int64(101), int64(7), 50, 10))
	mock.ExpectExec("UPDATE account_groups").
		WithArgs(int64(101), int64(7), 10, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "UPSTREAM_RELAY_RECOMMENDATION_STALE", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamRelayRepositoryApplyRecommendationRunRejectsEmptyPendingSuggestions(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUpstreamRelayRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, applied FROM upstream_relay_recommendation_runs").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "applied"}).
			AddRow(service.UpstreamRelayRunStatusSuccess, false))
	mock.ExpectQuery("SELECT candidate_id, account_id, target_group_id, old_priority, new_priority").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"candidate_id", "account_id", "target_group_id", "old_priority", "new_priority"}))
	mock.ExpectRollback()

	_, err := repo.ApplyRecommendationRun(ctx, 44, 99)

	require.ErrorIs(t, err, service.ErrUpstreamRelayNoPendingSuggestions)
	require.NoError(t, mock.ExpectationsWereMet())
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
	})

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

func sqlNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func expectUpstreamRelayConnectorGet(mock sqlmock.Sqlmock, id int64, now time.Time, connector service.UpstreamRelayConnector) {
	mock.ExpectQuery("SELECT id, name, base_url, auth_mode").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "base_url", "auth_mode", "bearer_token_encrypted",
			"refresh_token_encrypted", "login_email_encrypted", "cookie_encrypted", "user_agent_encrypted",
			"status", "credential_version", "last_verified_at", "last_synced_at", "last_error",
			"created_by", "created_at", "updated_at",
		}).AddRow(
			connector.ID, connector.Name, connector.BaseURL, connector.AuthMode, connector.BearerTokenEncrypted,
			connector.RefreshTokenEncrypted, connector.LoginEmailEncrypted, connector.CookieEncrypted, connector.UserAgentEncrypted,
			connector.Status, connector.CredentialVersion, connector.LastVerifiedAt, connector.LastSyncedAt, connector.LastError,
			connector.CreatedBy, now, now,
		))
}

func expectUpstreamRelayRunDetail(mock sqlmock.Sqlmock, now time.Time, applied bool) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 2, 2, applied, int64(99), now, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, s\\.candidate_id").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "run_id", "candidate_id", "connector_id", "connector_name",
			"account_id", "account_name", "upstream_group_id", "upstream_group_name",
			"target_group_id", "target_group_name", "old_priority", "new_priority",
			"final_rate_multiplier", "health_status", "reason_code", "confidence",
			"health_summary", "rate_source", "reason", "applied", "applied_by",
			"applied_at", "created_at",
		}).
			AddRow(int64(1), int64(44), int64(1), int64(10), "relay", int64(101), "account-a", "cheap", "cheap upstream", int64(7), "local-a", 50, 10, 0.8, "success", "rate_health_priority", "high", "成功率 100%", "login_user_group_rates", "rate", applied, int64(99), now, now).
			AddRow(int64(2), int64(44), int64(2), int64(10), "relay", int64(101), "account-a", "fast", "fast upstream", int64(8), "local-b", 60, 20, 1.1, "success", "rate_health_priority", "medium", "成功率 100%", "login_available_groups", "rate", applied, int64(99), now, now))
}

func expectUpstreamRelayRunDetailSingle(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery("SELECT id, status, total_candidates, suggestion_count").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "total_candidates", "suggestion_count", "applied", "applied_by",
			"applied_at", "error_message", "created_by", "created_at",
		}).AddRow(
			int64(44), service.UpstreamRelayRunStatusSuccess, 1, 1, false, nil, nil, "", int64(88), now,
		))
	mock.ExpectQuery("SELECT s\\.id, s\\.run_id, s\\.candidate_id").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "run_id", "candidate_id", "connector_id", "connector_name",
			"account_id", "account_name", "upstream_group_id", "upstream_group_name",
			"target_group_id", "target_group_name", "old_priority", "new_priority",
			"final_rate_multiplier", "health_status", "reason_code", "confidence",
			"health_summary", "rate_source", "reason", "applied", "applied_by",
			"applied_at", "created_at",
		}).AddRow(
			int64(1), int64(44), int64(9), int64(10), "relay", int64(101), "account-a",
			"cheap-upstream", "cheap upstream", int64(7), "local-a", 50, 10, 0.75,
			"healthy", "rate_health_priority", "high",
			"最近3次成功率 100%，连续成功 3，连续失败 0，p95 80ms",
			service.UpstreamRelayRateSourceOverride, "结构化建议原因", false, nil, nil, now,
		))
}
