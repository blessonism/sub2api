package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryCreateSyncRequestTypeAndLegacyFields(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	createdAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	log := &service.UsageLog{
		UserID:         1,
		APIKeyID:       2,
		AccountID:      3,
		RequestID:      "req-1",
		Model:          "gpt-5",
		RequestedModel: "gpt-5",
		InputTokens:    10,
		OutputTokens:   20,
		TotalCost:      1,
		ActualCost:     1,
		BillingType:    service.BillingTypeBalance,
		RequestType:    service.RequestTypeWSV2,
		Stream:         false,
		OpenAIWSMode:   false,
		CreatedAt:      createdAt,
	}

	mock.ExpectQuery("INSERT INTO usage_logs").
		WithArgs(
			log.UserID,
			log.APIKeyID,
			log.AccountID,
			log.RequestID,
			log.Model,
			log.RequestedModel,
			sqlmock.AnyArg(), // upstream_model
			sqlmock.AnyArg(), // upstream_response_model
			sqlmock.AnyArg(), // upstream_model_mismatch
			sqlmock.AnyArg(), // group_id
			sqlmock.AnyArg(), // subscription_id
			log.InputTokens,
			log.OutputTokens,
			log.CacheCreationTokens,
			log.CacheReadTokens,
			log.CacheCreation5mTokens,
			log.CacheCreation1hTokens,
			log.ImageOutputTokens,
			log.ImageOutputCost,
			log.ImageInputTokens,
			log.ImageInputCost,
			log.InputCost,
			log.OutputCost,
			log.CacheCreationCost,
			log.CacheReadCost,
			log.TotalCost,
			log.ActualCost,
			log.RateMultiplier,
			log.VisibleRateMultiplier,
			log.AccountRateMultiplier,
			log.BillingType,
			int16(service.RequestTypeWSV2),
			true,
			true,
			sqlmock.AnyArg(), // duration_ms
			sqlmock.AnyArg(), // first_token_ms
			sqlmock.AnyArg(), // user_agent
			sqlmock.AnyArg(), // ip_address
			log.ImageCount,
			sqlmock.AnyArg(), // image_size
			sqlmock.AnyArg(), // image_input_size
			sqlmock.AnyArg(), // image_output_size
			sqlmock.AnyArg(), // image_size_source
			sqlmock.AnyArg(), // image_size_breakdown
			sqlmock.AnyArg(), // video_count
			sqlmock.AnyArg(), // video_resolution
			sqlmock.AnyArg(), // video_duration_seconds
			sqlmock.AnyArg(), // service_tier
			sqlmock.AnyArg(), // reasoning_effort
			sqlmock.AnyArg(), // inbound_endpoint
			sqlmock.AnyArg(), // upstream_endpoint
			log.CacheTTLOverridden,
			log.LongContextBillingApplied,
			sqlmock.AnyArg(), // channel_id
			sqlmock.AnyArg(), // model_mapping_chain
			sqlmock.AnyArg(), // billing_tier
			sqlmock.AnyArg(), // billing_mode
			sqlmock.AnyArg(), // account_stats_cost
			sqlmock.AnyArg(), // session_id
			createdAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(99), createdAt))

	inserted, err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, int64(99), log.ID)
	require.Nil(t, log.ServiceTier)
	require.Equal(t, service.RequestTypeWSV2, log.RequestType)
	require.True(t, log.Stream)
	require.True(t, log.OpenAIWSMode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryCreate_PersistsServiceTier(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	createdAt := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	serviceTier := "priority"
	log := &service.UsageLog{
		UserID:         1,
		APIKeyID:       2,
		AccountID:      3,
		RequestID:      "req-service-tier",
		Model:          "gpt-5.4",
		RequestedModel: "gpt-5.4",
		ServiceTier:    &serviceTier,
		CreatedAt:      createdAt,
	}

	mock.ExpectQuery("INSERT INTO usage_logs").
		WithArgs(
			log.UserID,
			log.APIKeyID,
			log.AccountID,
			log.RequestID,
			log.Model,
			log.RequestedModel,
			sqlmock.AnyArg(), // upstream_model
			sqlmock.AnyArg(), // upstream_response_model
			sqlmock.AnyArg(), // upstream_model_mismatch
			sqlmock.AnyArg(), // group_id
			sqlmock.AnyArg(), // subscription_id
			log.InputTokens,
			log.OutputTokens,
			log.CacheCreationTokens,
			log.CacheReadTokens,
			log.CacheCreation5mTokens,
			log.CacheCreation1hTokens,
			log.ImageOutputTokens,
			log.ImageOutputCost,
			log.ImageInputTokens,
			log.ImageInputCost,
			log.InputCost,
			log.OutputCost,
			log.CacheCreationCost,
			log.CacheReadCost,
			log.TotalCost,
			log.ActualCost,
			log.RateMultiplier,
			log.VisibleRateMultiplier,
			log.AccountRateMultiplier,
			log.BillingType,
			int16(service.RequestTypeSync),
			false,
			false,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			log.ImageCount,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(), // image_input_size
			sqlmock.AnyArg(), // image_output_size
			sqlmock.AnyArg(), // image_size_source
			sqlmock.AnyArg(), // image_size_breakdown
			sqlmock.AnyArg(), // video_count
			sqlmock.AnyArg(), // video_resolution
			sqlmock.AnyArg(), // video_duration_seconds
			serviceTier,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			log.CacheTTLOverridden,
			log.LongContextBillingApplied,
			sqlmock.AnyArg(), // channel_id
			sqlmock.AnyArg(), // model_mapping_chain
			sqlmock.AnyArg(), // billing_tier
			sqlmock.AnyArg(), // billing_mode
			sqlmock.AnyArg(), // account_stats_cost
			sqlmock.AnyArg(), // session_id
			createdAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(100), createdAt))

	inserted, err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildUsageLogBestEffortInsertQuery_IncludesRequestedModelColumn(t *testing.T) {
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:         1,
		APIKeyID:       2,
		AccountID:      3,
		RequestID:      "req-best-effort-query",
		Model:          "gpt-5",
		RequestedModel: "gpt-5",
		CreatedAt:      time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC),
	})

	query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})

	require.Contains(t, query, "INSERT INTO usage_logs (")
	require.Contains(t, query, "\n\t\t\tmodel,\n\t\t\trequested_model,\n\t\t\tupstream_model,\n\t\t\tupstream_response_model,\n\t\t\tupstream_model_mismatch,")
	require.Contains(t, query, "\n\t\t\trequest_id,\n\t\t\tmodel,\n\t\t\trequested_model,\n\t\t\tupstream_model,\n\t\t\tupstream_response_model,\n\t\t\tupstream_model_mismatch,")
	require.Len(t, args, len(prepared.args))
	require.Equal(t, prepared.args[5], args[5])
}

func TestExecUsageLogInsertNoResult_PersistsRequestedModel(t *testing.T) {
	db, mock := newSQLMock(t)
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:         1,
		APIKeyID:       2,
		AccountID:      3,
		RequestID:      "req-best-effort-exec",
		Model:          "gpt-5",
		RequestedModel: "gpt-5",
		CreatedAt:      time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC),
	})

	mock.ExpectExec("INSERT INTO usage_logs").
		WithArgs(anySliceToDriverValues(prepared.args)...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := execUsageLogInsertNoResult(context.Background(), db, prepared)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrepareUsageLogInsert_ArgCountMatchesTypes(t *testing.T) {
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:         1,
		APIKeyID:       2,
		AccountID:      3,
		RequestID:      "req-arg-count",
		Model:          "gpt-5",
		RequestedModel: "gpt-5",
		CreatedAt:      time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC),
	})

	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
}

func TestPrepareUsageLogInsert_PersistsImageSizeMetadata(t *testing.T) {
	imageSize := "4K"
	inputSize := "1024x1024"
	outputSize := "3840x2160"
	source := "output"
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:             1,
		APIKeyID:           2,
		AccountID:          3,
		RequestID:          "req-image-metadata",
		Model:              "gpt-image-2",
		RequestedModel:     "gpt-image-2",
		ImageCount:         2,
		ImageSize:          &imageSize,
		ImageInputSize:     &inputSize,
		ImageOutputSize:    &outputSize,
		ImageSizeSource:    &source,
		ImageSizeBreakdown: map[string]int{"1K": 1, "4K": 1},
		CreatedAt:          time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC),
	})

	require.Equal(t, sql.NullString{String: imageSize, Valid: true}, prepared.args[38])
	require.Equal(t, sql.NullString{String: inputSize, Valid: true}, prepared.args[39])
	require.Equal(t, sql.NullString{String: outputSize, Valid: true}, prepared.args[40])
	require.Equal(t, sql.NullString{String: source, Valid: true}, prepared.args[41])
	breakdownJSON, ok := prepared.args[42].(string)
	require.True(t, ok)
	require.JSONEq(t, `{"1K":1,"4K":1}`, breakdownJSON)
}

func TestCoalesceTrimmedString(t *testing.T) {
	require.Equal(t, "fallback", coalesceTrimmedString(sql.NullString{}, "fallback"))
	require.Equal(t, "fallback", coalesceTrimmedString(sql.NullString{Valid: true, String: "   "}, "fallback"))
	require.Equal(t, "value", coalesceTrimmedString(sql.NullString{Valid: true, String: "value"}, "fallback"))
}

func TestAppendUsageLogBillingModeWhereCondition(t *testing.T) {
	tests := []struct {
		name          string
		billingMode   string
		wantCondition string
	}{
		{
			name:          "image includes explicit image and legacy image rows",
			billingMode:   string(service.BillingModeImage),
			wantCondition: "(billing_mode = $1 OR ((billing_mode IS NULL OR billing_mode = '') AND COALESCE(image_count, 0) > 0))",
		},
		{
			name:          "video remains exact",
			billingMode:   string(service.BillingModeVideo),
			wantCondition: "billing_mode = $1",
		},
		{
			name:          "token includes legacy non-image rows",
			billingMode:   string(service.BillingModeToken),
			wantCondition: "(billing_mode = $1 OR ((billing_mode IS NULL OR billing_mode = '') AND COALESCE(image_count, 0) <= 0))",
		},
		{
			name:          "per request remains exact",
			billingMode:   string(service.BillingModePerRequest),
			wantCondition: "billing_mode = $1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions, args := appendUsageLogBillingModeWhereCondition(nil, nil, tt.billingMode)
			require.Equal(t, []string{tt.wantCondition}, conditions)
			require.Equal(t, []any{tt.billingMode}, args)
		})
	}
}

func TestAppendUsageLogBillingModeWhereConditionWithAlias(t *testing.T) {
	conditions, args := appendUsageLogBillingModeWhereConditionWithAlias(nil, nil, string(service.BillingModeImage), "ul")

	require.Equal(t, []string{"(ul.billing_mode = $1 OR ((ul.billing_mode IS NULL OR ul.billing_mode = '') AND COALESCE(ul.image_count, 0) > 0))"}, conditions)
	require.Equal(t, []any{string(service.BillingModeImage)}, args)
}

func TestAppendUsageLogBillingModeQueryFilter(t *testing.T) {
	query, args := appendUsageLogBillingModeQueryFilter("SELECT * FROM usage_logs WHERE user_id = $1", []any{int64(42)}, string(service.BillingModeToken), "")

	require.Equal(t, "SELECT * FROM usage_logs WHERE user_id = $1 AND (billing_mode = $2 OR ((billing_mode IS NULL OR billing_mode = '') AND COALESCE(image_count, 0) <= 0))", query)
	require.Equal(t, []any{int64(42), string(service.BillingModeToken)}, args)
}

func anySliceToDriverValues(values []any) []driver.Value {
	out := make([]driver.Value, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func TestUsageLogRepositoryListWithFiltersRequestTypePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	requestType := int16(service.RequestTypeWSV2)
	stream := false
	filters := usagestats.UsageLogFilters{
		RequestType: &requestType,
		Stream:      &stream,
		ExactTotal:  true,
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usage_logs WHERE \\(request_type = \\$1 OR \\(request_type = 0 AND openai_ws_mode = TRUE\\)\\)").
		WithArgs(requestType).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT .* FROM usage_logs WHERE \\(request_type = \\$1 OR \\(request_type = 0 AND openai_ws_mode = TRUE\\)\\) ORDER BY id DESC LIMIT \\$2 OFFSET \\$3").
		WithArgs(requestType, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	logs, page, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, filters)
	require.NoError(t, err)
	require.Empty(t, logs)
	require.NotNil(t, page)
	require.Equal(t, int64(0), page.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryListWithFiltersSharedIPUsers(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	filters := usagestats.UsageLogFilters{
		GroupID:       7,
		SharedIPUsers: true,
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usage_logs WHERE group_id = \\$1 AND ip_address IN").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery("HAVING COUNT\\(DISTINCT user_id\\) > 1").
		WithArgs(int64(7), 20, 0).
		WillReturnRows(sqlmock.NewRows(strings.Split(usageLogSelectColumns, ", ")))

	logs, page, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, filters)
	require.NoError(t, err)
	require.Empty(t, logs)
	require.NotNil(t, page)
	require.Equal(t, int64(0), page.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryListWithFiltersRequestID(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	filters := usagestats.UsageLogFilters{RequestID: " req-0123 "}

	mock.ExpectQuery("SELECT .* FROM usage_logs WHERE request_id = \\$1 ORDER BY id DESC LIMIT \\$2 OFFSET \\$3").
		WithArgs("req-0123", 21, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	logs, page, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, filters)
	require.NoError(t, err)
	require.Empty(t, logs)
	require.NotNil(t, page)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryListWithFiltersRequestedModelSource(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	filters := usagestats.UsageLogFilters{
		Model:             "gpt-5",
		ModelFilterSource: usagestats.ModelSourceRequested,
	}

	mock.ExpectQuery("SELECT .* FROM usage_logs WHERE COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) = \\$1 ORDER BY id DESC LIMIT \\$2 OFFSET \\$3").
		WithArgs("gpt-5", 21, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	logs, page, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, filters)
	require.NoError(t, err)
	require.Empty(t, logs)
	require.NotNil(t, page)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetSharedIPUsersSummary(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	lastUsed := start.Add(2 * time.Hour)
	betaLastUsed := lastUsed.Add(-time.Hour)
	filters := usagestats.UsageLogFilters{StartTime: &start}

	mock.ExpectQuery("SELECT COUNT\\(DISTINCT ip_address\\), COUNT\\(DISTINCT user_id\\), COUNT\\(\\*\\) FROM usage_logs WHERE created_at >= \\$1 AND ip_address IN").
		WithArgs(start).
		WillReturnRows(sqlmock.NewRows([]string{"ip_count", "user_count", "record_count"}).
			AddRow(int64(2), int64(51), int64(80)))
	groupRows := sqlmock.NewRows([]string{"ip_address", "user_count", "record_count", "last_used_at", "total_tokens", "actual_cost", "user_id", "email", "deleted", "user_record_count", "user_last_used_at", "user_total_tokens", "user_actual_cost"}).
		AddRow("192.0.2.10", int64(2), int64(6), lastUsed, int64(3000), 0.6, int64(7), "alpha@example.com", false, int64(4), lastUsed, int64(2000), 0.4).
		AddRow("192.0.2.10", int64(2), int64(6), lastUsed, int64(3000), 0.6, int64(8), "beta@example.com", false, int64(2), betaLastUsed, int64(1000), 0.2)
	mock.ExpectQuery("WITH matched_logs AS").
		WithArgs(start).
		WillReturnRows(groupRows)
	summary, err := repo.GetSharedIPUsersSummary(context.Background(), filters)
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.IPCount)
	require.Equal(t, int64(51), summary.UserCount)
	require.Equal(t, int64(80), summary.RecordCount)
	require.Equal(t, sharedIPGroupSummaryLimit, summary.IPGroupsLimit)
	require.True(t, summary.IPGroupsTruncated)
	require.Equal(t, int64(1), summary.HiddenIPGroupCount)
	require.Len(t, summary.IPGroups, 1)
	require.Equal(t, usagestats.SharedIPGroupSummaryItem{
		IPAddress:   "192.0.2.10",
		UserCount:   2,
		RecordCount: 6,
		LastUsedAt:  &lastUsed,
		TotalTokens: 3000,
		ActualCost:  0.6,
		UsersLimit:  sharedIPGroupUserSummaryLimit,
		Users: []usagestats.SharedIPGroupUserSummaryItem{
			{UserID: 7, Email: "alpha@example.com", Deleted: false, RecordCount: 4, LastUsedAt: &lastUsed, TotalTokens: 2000, ActualCost: 0.4},
			{UserID: 8, Email: "beta@example.com", Deleted: false, RecordCount: 2, LastUsedAt: &betaLastUsed, TotalTokens: 1000, ActualCost: 0.2},
		},
	}, summary.IPGroups[0])
	require.Empty(t, summary.Users)
	require.Zero(t, summary.UsersLimit)
	require.False(t, summary.UsersTruncated)
	require.Zero(t, summary.HiddenUserCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUsageTrendWithFiltersRequestTypePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	requestType := int16(service.RequestTypeStream)
	stream := true

	mock.ExpectQuery("AND \\(request_type = \\$3 OR \\(request_type = 0 AND stream = TRUE AND openai_ws_mode = FALSE\\)\\)").
		WithArgs(start, end, requestType).
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost"}))

	trend, err := repo.GetUsageTrendWithFilters(context.Background(), start, end, "day", 0, 0, 0, 0, "", &requestType, &stream, nil)
	require.NoError(t, err)
	require.Empty(t, trend)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUsageTrendWithUsageFiltersRequestedModelSource(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filters := usagestats.UsageLogFilters{
		Model:             "gpt-5",
		ModelFilterSource: usagestats.ModelSourceRequested,
	}

	mock.ExpectQuery("AND COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) = \\$3").
		WithArgs(start, end, "gpt-5").
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost"}))

	trend, err := repo.GetUsageTrendWithUsageFilters(context.Background(), start, end, "day", filters)
	require.NoError(t, err)
	require.Empty(t, trend)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetModelStatsWithFiltersRequestTypePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	requestType := int16(service.RequestTypeWSV2)
	stream := false

	mock.ExpectQuery("AND \\(request_type = \\$3 OR \\(request_type = 0 AND openai_ws_mode = TRUE\\)\\)").
		WithArgs(start, end, requestType).
		WillReturnRows(sqlmock.NewRows([]string{"model", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost", "account_cost"}))

	stats, err := repo.GetModelStatsWithFilters(context.Background(), start, end, 0, 0, 0, 0, &requestType, &stream, nil)
	require.NoError(t, err)
	require.Empty(t, stats)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserModelStatsUsesRequestedModel(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery("(?s)SELECT\\s+COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) as model,.*WHERE created_at >= \\$1 AND created_at < \\$2\\s+AND user_id = \\$3.*GROUP BY COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) ORDER BY total_tokens DESC").
		WithArgs(start, end, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"model", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).AddRow("gpt-5.5", int64(2), int64(10), int64(20), int64(0), int64(0), int64(30), 0.1, 0.08, 0.07))

	stats, err := repo.GetUserModelStats(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "gpt-5.5", stats[0].Model)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetStatsWithFiltersRequestedModelSource(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	filters := usagestats.UsageLogFilters{
		Model:             "gpt-5",
		ModelFilterSource: usagestats.ModelSourceRequested,
	}

	mock.ExpectQuery("(?s)FROM usage_logs\\s+WHERE COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) = \\$1.*GROUP BY GROUPING SETS").
		WithArgs("gpt-5").
		WillReturnRows(sqlmock.NewRows([]string{
			"inbound_grouped",
			"upstream_grouped",
			"inbound_endpoint",
			"upstream_endpoint",
			"requests",
			"input_tokens",
			"output_tokens",
			"cache_creation_tokens",
			"cache_read_tokens",
			"cost",
			"actual_cost",
			"account_cost",
			"avg_duration_ms",
		}).
			AddRow(1, 1, nil, nil, int64(1), int64(2), int64(3), int64(1), int64(3), 1.2, 1.0, 1.2, 20.0).
			AddRow(0, 1, "/v1/responses", nil, int64(1), int64(2), int64(3), int64(1), int64(3), 1.2, 1.0, 1.2, 20.0).
			AddRow(1, 0, nil, "/v1/responses", int64(1), int64(2), int64(3), int64(1), int64(3), 1.2, 1.0, 1.2, 20.0).
			AddRow(0, 0, "/v1/responses", "/v1/responses", int64(1), int64(2), int64(3), int64(1), int64(3), 1.2, 1.0, 1.2, 20.0))

	stats, err := repo.GetStatsWithFilters(context.Background(), filters)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.TotalRequests)
	require.Equal(t, "/v1/responses", stats.Endpoints[0].Endpoint)
	require.Equal(t, "/v1/responses -> /v1/responses", stats.EndpointPaths[0].Endpoint)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetStatsWithFiltersRequestTypePriority(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	requestType := int16(service.RequestTypeSync)
	stream := true
	filters := usagestats.UsageLogFilters{
		RequestType: &requestType,
		Stream:      &stream,
	}

	mock.ExpectQuery("(?s)FROM usage_logs\\s+WHERE \\(request_type = \\$1 OR \\(request_type = 0 AND stream = FALSE AND openai_ws_mode = FALSE\\)\\).*GROUP BY GROUPING SETS").
		WithArgs(requestType).
		WillReturnRows(sqlmock.NewRows([]string{
			"inbound_grouped",
			"upstream_grouped",
			"inbound_endpoint",
			"upstream_endpoint",
			"requests",
			"input_tokens",
			"output_tokens",
			"cache_creation_tokens",
			"cache_read_tokens",
			"cost",
			"actual_cost",
			"account_cost",
			"avg_duration_ms",
		}).AddRow(1, 1, nil, nil, int64(1), int64(2), int64(3), int64(1), int64(3), 1.2, 1.0, 1.2, 20.0))

	stats, err := repo.GetStatsWithFilters(context.Background(), filters)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.TotalRequests)
	require.Equal(t, int64(9), stats.TotalTokens)
	require.NotNil(t, stats.TotalAccountCost, "TotalAccountCost should always be returned")
	require.Equal(t, 1.2, *stats.TotalAccountCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetModelStatsAccountCostColumn(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"model", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).
			AddRow("claude-opus-4-6", int64(10), int64(100), int64(200), int64(5), int64(3), int64(308), 2.5, 2.0, 1.8).
			AddRow("claude-sonnet-4-6", int64(5), int64(50), int64(100), int64(0), int64(0), int64(150), 1.0, 0.8, 0.7))

	results, err := repo.GetModelStatsWithFilters(context.Background(), start, end, 0, 0, 0, 0, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "claude-opus-4-6", results[0].Model)
	require.Equal(t, 2.5, results[0].Cost)
	require.Equal(t, 2.0, results[0].ActualCost)
	require.Equal(t, 1.8, results[0].AccountCost)
	require.Equal(t, "claude-sonnet-4-6", results[1].Model)
	require.Equal(t, 0.7, results[1].AccountCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetModelStatsWithUsageFiltersAppliesRequestedModelFilter(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filters := usagestats.UsageLogFilters{Model: "gpt-5"}

	mock.ExpectQuery("AND COALESCE\\(NULLIF\\(TRIM\\(requested_model\\), ''\\), model\\) = \\$3").
		WithArgs(start, end, "gpt-5").
		WillReturnRows(sqlmock.NewRows([]string{
			"model", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).AddRow("gpt-5", int64(1), int64(10), int64(20), int64(0), int64(0), int64(30), 0.1, 0.08, 0.07))

	results, err := repo.GetModelStatsWithUsageFiltersBySource(context.Background(), start, end, filters, usagestats.ModelSourceRequested)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "gpt-5", results[0].Model)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetGroupStatsAccountCostColumn(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "group_name", "requests", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).
			AddRow(int64(1), "azure-cc", int64(100), int64(5000), 10.0, 8.5, 7.2).
			AddRow(int64(2), "max", int64(50), int64(2000), 5.0, 4.0, 3.5))

	results, err := repo.GetGroupStatsWithFilters(context.Background(), start, end, 0, 0, 0, 0, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, int64(1), results[0].GroupID)
	require.Equal(t, "azure-cc", results[0].GroupName)
	require.Equal(t, 10.0, results[0].Cost)
	require.Equal(t, 8.5, results[0].ActualCost)
	require.Equal(t, 7.2, results[0].AccountCost)
	require.Equal(t, int64(2), results[1].GroupID)
	require.Equal(t, 3.5, results[1].AccountCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetGroupStatsWithUsageFiltersAppliesRequestedModelFilter(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filters := usagestats.UsageLogFilters{Model: "gpt-5"}

	mock.ExpectQuery("AND COALESCE\\(NULLIF\\(TRIM\\(ul.requested_model\\), ''\\), ul.model\\) = \\$3").
		WithArgs(start, end, "gpt-5").
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "group_name", "requests", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).AddRow(int64(1), "default", int64(1), int64(30), 0.1, 0.08, 0.07))

	results, err := repo.GetGroupStatsWithUsageFilters(context.Background(), start, end, filters)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, int64(1), results[0].GroupID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetStatsWithFiltersAlwaysReturnsAccountCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	// No AccountID filter set - TotalAccountCost should still be returned
	filters := usagestats.UsageLogFilters{}

	mock.ExpectQuery("(?s)FROM usage_logs.*GROUP BY GROUPING SETS").
		WillReturnRows(sqlmock.NewRows([]string{
			"inbound_grouped", "upstream_grouped", "inbound_endpoint", "upstream_endpoint",
			"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
			"cost", "actual_cost", "account_cost", "avg_duration_ms",
		}).AddRow(1, 1, nil, nil, int64(50), int64(1000), int64(2000), int64(60), int64(40), 15.0, 12.5, 11.0, 100.0))

	stats, err := repo.GetStatsWithFilters(context.Background(), filters)
	require.NoError(t, err)
	require.NotNil(t, stats.TotalAccountCost, "TotalAccountCost must always be returned, even without AccountID filter")
	require.Equal(t, 11.0, *stats.TotalAccountCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserSpendingRanking(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	betaLastUsed := start.Add(12 * time.Hour)
	alphaLastUsed := start.Add(10 * time.Hour)
	gammaLastUsed := start.Add(8 * time.Hour)

	rows := sqlmock.NewRows([]string{"user_id", "email", "username", "actual_cost", "requests", "tokens", "last_used_at", "total_actual_cost", "total_requests", "total_tokens"}).
		AddRow(int64(2), "beta@example.com", "beta", 12.5, int64(9), int64(900), betaLastUsed, 40.0, int64(30), int64(2600)).
		AddRow(int64(1), "alpha@example.com", "alpha", 12.5, int64(8), int64(800), alphaLastUsed, 40.0, int64(30), int64(2600)).
		AddRow(int64(3), "gamma@example.com", "", 4.25, int64(5), int64(300), gammaLastUsed, 40.0, int64(30), int64(2600))

	mock.ExpectQuery("WITH raw_user_spend AS \\(").
		WithArgs(start, end, 12, "2025-01-01", "2025-01-02").
		WillReturnRows(rows)

	got, err := repo.GetUserSpendingRanking(context.Background(), start, end, 12)
	require.NoError(t, err)
	require.Equal(t, &usagestats.UserSpendingRankingResponse{
		Ranking: []usagestats.UserSpendingRankingItem{
			{UserID: 2, Email: "beta@example.com", Username: "beta", ActualCost: 12.5, Requests: 9, Tokens: 900, LastUsedAt: betaLastUsed},
			{UserID: 1, Email: "alpha@example.com", Username: "alpha", ActualCost: 12.5, Requests: 8, Tokens: 800, LastUsedAt: alphaLastUsed},
			{UserID: 3, Email: "gamma@example.com", ActualCost: 4.25, Requests: 5, Tokens: 300, LastUsedAt: gammaLastUsed},
		},
		TotalActualCost: 40.0,
		TotalRequests:   30,
		TotalTokens:     2600,
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetAdminTokenLeaderboard(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	registeredAt := time.Date(2024, 12, 1, 8, 0, 0, 0, time.UTC)
	lastUsedAt := time.Date(2025, 1, 1, 15, 30, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"rank", "user_id", "email", "username", "status", "registered_at",
		"last_used_at", "requests", "tokens", "cost", "actual_cost", "account_cost",
		"total_requests", "total_tokens", "total_cost", "total_actual_cost", "total_account_cost",
	}).AddRow(
		int64(1), int64(7), "alice@example.com", "alice", "active", registeredAt,
		lastUsedAt,
		int64(6), int64(2000), 3.4, 2.8, 1.7,
		int64(6), int64(2000), 3.4, 2.8, 1.7,
	)

	mock.ExpectQuery("WITH raw_user_usage AS").
		WithArgs(start, end, "%alice%", int64(3), "claude-opus", "active", 20).
		WillReturnRows(rows)

	got, err := repo.GetAdminTokenLeaderboard(context.Background(), start, end, usagestats.AdminTokenLeaderboardFilters{
		Email:      "alice",
		GroupID:    3,
		Model:      "claude-opus",
		ModelType:  usagestats.ModelSourceRequested,
		UserStatus: "active",
		Limit:      20,
	})
	require.NoError(t, err)
	require.Equal(t, &usagestats.AdminTokenLeaderboardResponse{
		Ranking: []usagestats.AdminTokenLeaderboardUser{
			{
				Rank:         1,
				UserID:       7,
				Email:        "alice@example.com",
				Username:     "alice",
				Status:       "active",
				RegisteredAt: registeredAt,
				LastUsedAt:   lastUsedAt,
				Requests:     6,
				Tokens:       2000,
				Cost:         3.4,
				ActualCost:   2.8,
				AccountCost:  1.7,
			},
		},
		TotalRequests:    6,
		TotalTokens:      2000,
		TotalCost:        3.4,
		TotalActualCost:  2.8,
		TotalAccountCost: 1.7,
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserSpendingRankingBackfillsSubscriptionQuotaCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	lastUsed := start.Add(15 * time.Hour)

	rows := sqlmock.NewRows([]string{"user_id", "email", "actual_cost", "requests", "tokens", "last_used_at", "total_actual_cost", "total_requests", "total_tokens"}).
		AddRow(int64(8), "sub@example.com", 6.0, int64(3), int64(1200), lastUsed, 6.0, int64(3), int64(1200))

	mock.ExpectQuery(fmt.Sprintf("CASE WHEN \\(u\\.subscription_id IS NOT NULL OR u\\.billing_type = %d\\) AND COALESCE\\(u\\.actual_cost, 0\\) <= 0", service.BillingTypeSubscription)).
		WithArgs(start, end, 5, "2025-01-02", "2025-01-03").
		WillReturnRows(rows)

	got, err := repo.GetUserSpendingRanking(context.Background(), start, end, 5)
	require.NoError(t, err)
	require.Equal(t, []usagestats.UserSpendingRankingItem{
		{UserID: 8, Email: "sub@example.com", ActualCost: 6.0, Requests: 3, Tokens: 1200, LastUsedAt: lastUsed},
	}, got.Ranking)
	require.InDelta(t, 6.0, got.TotalActualCost, 1e-9)
	require.Equal(t, int64(3), got.TotalRequests)
	require.Equal(t, int64(1200), got.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetAdminTokenLeaderboardUserDetails(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	userID := int64(7)

	mock.ExpectQuery("COALESCE\\(ul.api_key_id, 0\\) as api_key_id").
		WithArgs(start, end, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"api_key_id", "api_key_name", "requests", "tokens", "cost", "actual_cost", "account_cost",
		}).AddRow(int64(11), "prod-key", int64(3), int64(900), 1.2, 1.1, 0.7))

	mock.ExpectQuery("COALESCE\\(ul.group_id, 0\\) as group_id").
		WithArgs(start, end, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "group_name", "requests", "tokens", "cost", "actual_cost", "account_cost",
		}).AddRow(int64(5), "vip", int64(2), int64(600), 0.9, 0.8, 0.5))

	mock.ExpectQuery("requested_model[\\s\\S]*GROUP BY 1").
		WithArgs(start, end, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"model", "requests", "tokens", "cost", "actual_cost", "account_cost",
		}).AddRow("claude-opus", int64(4), int64(1200), 1.6, 1.4, 0.9))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(token_delta\\), 0\\) FROM admin_usage_calibration_daily_allocations").
		WithArgs(userID, "2025-01-01", "2025-01-02").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(-150)))
	mock.ExpectQuery("(?s)WITH balance_deltas AS .*admin_usage_calibration_daily_allocations.*SELECT COALESCE\\(SUM\\(balance_delta\\), 0\\) FROM balance_deltas").
		WithArgs(userID, "2025-01-01", "2025-01-02", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(-2.5))

	got, err := repo.GetAdminTokenLeaderboardUserDetails(context.Background(), start, end, userID, usagestats.AdminTokenLeaderboardFilters{})
	require.NoError(t, err)
	require.Equal(t, &usagestats.AdminTokenLeaderboardUserDetails{
		CalibrationTokens:       -150,
		CalibrationBalanceDelta: -2.5,
		APIKeys: []usagestats.AdminTokenLeaderboardAPIKeyUsage{
			{APIKeyID: 11, APIKeyName: "prod-key", Requests: 3, Tokens: 900, Cost: 1.2, ActualCost: 1.1, AccountCost: 0.7},
		},
		Groups: []usagestats.AdminTokenLeaderboardGroupUsage{
			{GroupID: 5, GroupName: "vip", Requests: 2, Tokens: 600, Cost: 0.9, ActualCost: 0.8, AccountCost: 0.5},
		},
		Models: []usagestats.AdminTokenLeaderboardModelUsage{
			{Model: "claude-opus", Requests: 4, Tokens: 1200, Cost: 1.6, ActualCost: 1.4, AccountCost: 0.9},
		},
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserTokenLeaderboardIncludesCurrentUserOutsideTop(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)

	rows := sqlmock.NewRows([]string{"row_type", "rank", "user_id", "email", "requests", "tokens", "discount_rate_multiplier"}).
		AddRow("top", int64(1), int64(2), "beta@example.com", int64(9), int64(900), 0.7).
		AddRow("top", int64(2), int64(1), "alpha@example.com", int64(8), int64(900), 0.8).
		AddRow("current", int64(4), currentUserID, "current@example.com", int64(3), int64(120), 0.9)

	mock.ExpectQuery("ROW_NUMBER\\(\\) OVER \\(ORDER BY uu\\.tokens DESC, uu\\.requests DESC, uu\\.user_id ASC\\)").
		WithArgs(start, end, 2, currentUserID, "2026-06-18", "2026-06-19").
		WillReturnRows(rows)

	got, err := repo.GetUserTokenLeaderboard(context.Background(), start, end, 2, currentUserID)
	require.NoError(t, err)
	require.Equal(t, []usagestats.UserTokenLeaderboardRow{
		{Rank: 1, UserID: 2, Email: "beta@example.com", Requests: 9, Tokens: 900, DiscountRateMultiplier: ptrLeaderboardRateForRepoTest(0.7)},
		{Rank: 2, UserID: 1, Email: "alpha@example.com", Requests: 8, Tokens: 900, DiscountRateMultiplier: ptrLeaderboardRateForRepoTest(0.8)},
	}, got.Ranking)
	require.NotNil(t, got.MyRank)
	require.Equal(t, &usagestats.UserTokenLeaderboardRow{
		Rank:                   4,
		UserID:                 currentUserID,
		Email:                  "current@example.com",
		Requests:               3,
		Tokens:                 120,
		DiscountRateMultiplier: ptrLeaderboardRateForRepoTest(0.9),
	}, got.MyRank)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserTokenLeaderboardDefaultsLimitToTop10(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)

	rows := sqlmock.NewRows([]string{"row_type", "rank", "user_id", "email", "requests", "tokens", "discount_rate_multiplier"})
	mock.ExpectQuery("ROW_NUMBER\\(\\) OVER \\(ORDER BY uu\\.tokens DESC, uu\\.requests DESC, uu\\.user_id ASC\\)").
		WithArgs(start, end, 10, currentUserID, "2026-06-18", "2026-06-19").
		WillReturnRows(rows)

	got, err := repo.GetUserTokenLeaderboard(context.Background(), start, end, 0, currentUserID)
	require.NoError(t, err)
	require.Empty(t, got.Ranking)
	require.Nil(t, got.MyRank)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserTokenLeaderboardUsesCurrentAutoMultiplier(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)

	rows := sqlmock.NewRows([]string{"row_type", "rank", "user_id", "email", "requests", "tokens", "discount_rate_multiplier"}).
		AddRow("top", int64(1), currentUserID, "current@example.com", int64(3), int64(120), 0.7)

	mock.ExpectQuery("MIN\\([\\s\\S]*CASE[\\s\\S]*WHEN ugr\\.visible_rate_multiplier IS NOT NULL THEN ugr\\.visible_rate_multiplier[\\s\\S]*ELSE LEAST\\(ugr\\.rate_multiplier, COALESCE\\(target_group\\.visible_rate_multiplier, target_group\\.rate_multiplier\\)\\)[\\s\\S]*END[\\s\\S]*\\) AS rate_multiplier[\\s\\S]*JOIN token_usage_auto_policies p ON p\\.id = a\\.policy_id AND p\\.enabled = TRUE[\\s\\S]*JOIN groups target_group ON target_group\\.id = a\\.target_group_id AND target_group\\.status = 'active'[\\s\\S]*JOIN user_group_rate_multipliers ugr ON ugr\\.user_id = a\\.user_id AND ugr\\.group_id = a\\.target_group_id[\\s\\S]*ugr\\.rate_multiplier = a\\.last_rate_multiplier").
		WithArgs(start, end, 10, currentUserID, "2026-06-18", "2026-06-19").
		WillReturnRows(rows)

	got, err := repo.GetUserTokenLeaderboard(context.Background(), start, end, 10, currentUserID)
	require.NoError(t, err)
	require.Len(t, got.Ranking, 1)
	require.Equal(t, ptrLeaderboardRateForRepoTest(0.7), got.Ranking[0].DiscountRateMultiplier)
	require.NotNil(t, got.MyRank)
	require.Equal(t, ptrLeaderboardRateForRepoTest(0.7), got.MyRank.DiscountRateMultiplier)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserTokenLeaderboardCommonGroupFallsBackToRealMultiplier(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	currentUserID := int64(9)

	rows := sqlmock.NewRows([]string{"row_type", "rank", "user_id", "email", "requests", "tokens", "discount_rate_multiplier"}).
		AddRow("top", int64(1), currentUserID, "current@example.com", int64(3), int64(120), 0.85)

	mock.ExpectQuery("common_multiplier AS \\([\\s\\S]*SELECT COALESCE\\(g\\.visible_rate_multiplier, g\\.rate_multiplier\\) AS rate_multiplier[\\s\\S]*s\\.key = 'token_leaderboard_common_group_id'[\\s\\S]*g\\.subscription_type = 'standard'[\\s\\S]*g\\.is_exclusive = FALSE").
		WithArgs(start, end, 10, currentUserID, "2026-06-18", "2026-06-19").
		WillReturnRows(rows)

	got, err := repo.GetUserTokenLeaderboard(context.Background(), start, end, 10, currentUserID)
	require.NoError(t, err)
	require.Len(t, got.Ranking, 1)
	require.Equal(t, ptrLeaderboardRateForRepoTest(0.85), got.Ranking[0].DiscountRateMultiplier)
	require.NotNil(t, got.MyRank)
	require.Equal(t, ptrLeaderboardRateForRepoTest(0.85), got.MyRank.DiscountRateMultiplier)
	require.NoError(t, mock.ExpectationsWereMet())
}

func ptrLeaderboardRateForRepoTest(v float64) *float64 {
	return &v
}

func TestBuildRequestTypeFilterConditionLegacyFallback(t *testing.T) {
	tests := []struct {
		name      string
		request   int16
		wantWhere string
		wantArg   int16
	}{
		{
			name:      "sync_with_legacy_fallback",
			request:   int16(service.RequestTypeSync),
			wantWhere: "(request_type = $3 OR (request_type = 0 AND stream = FALSE AND openai_ws_mode = FALSE))",
			wantArg:   int16(service.RequestTypeSync),
		},
		{
			name:      "stream_with_legacy_fallback",
			request:   int16(service.RequestTypeStream),
			wantWhere: "(request_type = $3 OR (request_type = 0 AND stream = TRUE AND openai_ws_mode = FALSE))",
			wantArg:   int16(service.RequestTypeStream),
		},
		{
			name:      "ws_v2_with_legacy_fallback",
			request:   int16(service.RequestTypeWSV2),
			wantWhere: "(request_type = $3 OR (request_type = 0 AND openai_ws_mode = TRUE))",
			wantArg:   int16(service.RequestTypeWSV2),
		},
		{
			name:      "invalid_request_type_normalized_to_unknown",
			request:   int16(99),
			wantWhere: "request_type = $3",
			wantArg:   int16(service.RequestTypeUnknown),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := buildRequestTypeFilterCondition(3, tt.request)
			require.Equal(t, tt.wantWhere, where)
			require.Equal(t, []any{tt.wantArg}, args)
		})
	}
}

type usageLogScannerStub struct {
	values []any
}

func (s usageLogScannerStub) Scan(dest ...any) error {
	if len(dest) != len(s.values) {
		return fmt.Errorf("scan arg count mismatch: got %d want %d", len(dest), len(s.values))
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Pointer {
			return fmt.Errorf("dest[%d] is not pointer", i)
		}
		dv.Elem().Set(reflect.ValueOf(s.values[i]))
	}
	return nil
}

func TestScanUsageLogRequestTypeAndLegacyFallback(t *testing.T) {
	t.Run("image_size_metadata_is_scanned", func(t *testing.T) {
		now := time.Now().UTC()
		log, err := scanUsageLog(usageLogScannerStub{values: []any{
			int64(4),
			int64(13),
			int64(23),
			int64(33),
			sql.NullString{Valid: true, String: "req-image-metadata"},
			"gpt-image-2",
			sql.NullString{Valid: true, String: "gpt-image-2"},
			sql.NullString{},
			sql.NullString{},
			sql.NullBool{},
			sql.NullInt64{},
			sql.NullInt64{},
			0, 0, 0, 0, 0, 0,
			0, 0.0, // image_output_tokens, image_output_cost
			0, 0.0, // image_input_tokens, image_input_cost
			0.0, 0.0, 0.0, 0.0, 0.8, 0.8,
			1.0,
			sql.NullFloat64{}, // visible_rate_multiplier
			sql.NullFloat64{},
			int16(service.BillingTypeBalance),
			int16(service.RequestTypeSync),
			false,
			false,
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			2,
			sql.NullString{Valid: true, String: "4K"},
			sql.NullString{Valid: true, String: "1024x1024"},
			sql.NullString{Valid: true, String: "3840x2160"},
			sql.NullString{Valid: true, String: "output"},
			sql.NullString{Valid: true, String: `{"4K":2}`},
			0,                // video_count
			sql.NullString{}, // video_resolution
			sql.NullInt64{},  // video_duration_seconds
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			false,
			false,
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullFloat64{},
			sql.NullString{},
			now,
		}})
		require.NoError(t, err)
		require.Equal(t, 2, log.ImageCount)
		require.NotNil(t, log.ImageSize)
		require.Equal(t, "4K", *log.ImageSize)
		require.NotNil(t, log.ImageInputSize)
		require.Equal(t, "1024x1024", *log.ImageInputSize)
		require.NotNil(t, log.ImageOutputSize)
		require.Equal(t, "3840x2160", *log.ImageOutputSize)
		require.NotNil(t, log.ImageSizeSource)
		require.Equal(t, "output", *log.ImageSizeSource)
		require.Equal(t, map[string]int{"4K": 2}, log.ImageSizeBreakdown)
	})

	t.Run("request_type_ws_v2_overrides_legacy", func(t *testing.T) {
		now := time.Now().UTC()
		log, err := scanUsageLog(usageLogScannerStub{values: []any{
			int64(1),  // id
			int64(10), // user_id
			int64(20), // api_key_id
			int64(30), // account_id
			sql.NullString{Valid: true, String: "req-1"},
			"gpt-5", // model
			sql.NullString{Valid: true, String: "gpt-5"}, // requested_model
			sql.NullString{},  // upstream_model
			sql.NullString{},  // upstream_response_model
			sql.NullBool{},    // upstream_model_mismatch
			sql.NullInt64{},   // group_id
			sql.NullInt64{},   // subscription_id
			1,                 // input_tokens
			2,                 // output_tokens
			3,                 // cache_creation_tokens
			4,                 // cache_read_tokens
			5,                 // cache_creation_5m_tokens
			6,                 // cache_creation_1h_tokens
			0,                 // image_output_tokens
			0.0,               // image_output_cost
			0,                 // image_input_tokens
			0.0,               // image_input_cost
			0.1,               // input_cost
			0.2,               // output_cost
			0.3,               // cache_creation_cost
			0.4,               // cache_read_cost
			1.0,               // total_cost
			0.9,               // actual_cost
			1.0,               // rate_multiplier
			sql.NullFloat64{}, // visible_rate_multiplier
			sql.NullFloat64{}, // account_rate_multiplier
			int16(service.BillingTypeBalance),
			int16(service.RequestTypeWSV2),
			false, // legacy stream
			false, // legacy openai ws
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			0,
			sql.NullString{},
			sql.NullString{}, // image_input_size
			sql.NullString{}, // image_output_size
			sql.NullString{}, // image_size_source
			sql.NullString{}, // image_size_breakdown
			0,                // video_count
			sql.NullString{}, // video_resolution
			sql.NullInt64{},  // video_duration_seconds
			sql.NullString{Valid: true, String: "priority"},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			false,
			false,
			sql.NullInt64{},   // channel_id
			sql.NullString{},  // model_mapping_chain
			sql.NullString{},  // billing_tier
			sql.NullString{},  // billing_mode
			sql.NullFloat64{}, // account_stats_cost
			sql.NullString{},  // session_id
			now,
		}})
		require.NoError(t, err)
		require.NotNil(t, log.ServiceTier)
		require.Equal(t, "priority", *log.ServiceTier)
		require.Equal(t, service.RequestTypeWSV2, log.RequestType)
		require.True(t, log.Stream)
		require.True(t, log.OpenAIWSMode)
	})

	t.Run("request_type_unknown_falls_back_to_legacy", func(t *testing.T) {
		now := time.Now().UTC()
		log, err := scanUsageLog(usageLogScannerStub{values: []any{
			int64(2),
			int64(11),
			int64(21),
			int64(31),
			sql.NullString{Valid: true, String: "req-2"},
			"gpt-5",
			sql.NullString{Valid: true, String: "gpt-5"},
			sql.NullString{},
			sql.NullString{},
			sql.NullBool{},
			sql.NullInt64{},
			sql.NullInt64{},
			1, 2, 3, 4, 5, 6,
			0, 0.0, // image_output_tokens, image_output_cost
			0, 0.0, // image_input_tokens, image_input_cost
			0.1, 0.2, 0.3, 0.4, 1.0, 0.9,
			1.0,
			sql.NullFloat64{}, // visible_rate_multiplier
			sql.NullFloat64{},
			int16(service.BillingTypeBalance),
			int16(service.RequestTypeUnknown),
			true,
			false,
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			0,
			sql.NullString{},
			sql.NullString{}, // image_input_size
			sql.NullString{}, // image_output_size
			sql.NullString{}, // image_size_source
			sql.NullString{}, // image_size_breakdown
			0,                // video_count
			sql.NullString{}, // video_resolution
			sql.NullInt64{},  // video_duration_seconds
			sql.NullString{Valid: true, String: "flex"},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			false,
			false,
			sql.NullInt64{},   // channel_id
			sql.NullString{},  // model_mapping_chain
			sql.NullString{},  // billing_tier
			sql.NullString{},  // billing_mode
			sql.NullFloat64{}, // account_stats_cost
			sql.NullString{},  // session_id
			now,
		}})
		require.NoError(t, err)
		require.NotNil(t, log.ServiceTier)
		require.Equal(t, "flex", *log.ServiceTier)
		require.Equal(t, service.RequestTypeStream, log.RequestType)
		require.True(t, log.Stream)
		require.False(t, log.OpenAIWSMode)
	})

	t.Run("service_tier_is_scanned", func(t *testing.T) {
		now := time.Now().UTC()
		log, err := scanUsageLog(usageLogScannerStub{values: []any{
			int64(3),
			int64(12),
			int64(22),
			int64(32),
			sql.NullString{Valid: true, String: "req-3"},
			"gpt-5.4",
			sql.NullString{Valid: true, String: "gpt-5.4"},
			sql.NullString{},
			sql.NullString{},
			sql.NullBool{},
			sql.NullInt64{},
			sql.NullInt64{},
			1, 2, 3, 4, 5, 6,
			0, 0.0, // image_output_tokens, image_output_cost
			0, 0.0, // image_input_tokens, image_input_cost
			0.1, 0.2, 0.3, 0.4, 1.0, 0.9,
			1.0,
			sql.NullFloat64{}, // visible_rate_multiplier
			sql.NullFloat64{},
			int16(service.BillingTypeBalance),
			int16(service.RequestTypeSync),
			false,
			false,
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			0,
			sql.NullString{},
			sql.NullString{}, // image_input_size
			sql.NullString{}, // image_output_size
			sql.NullString{}, // image_size_source
			sql.NullString{}, // image_size_breakdown
			0,                // video_count
			sql.NullString{}, // video_resolution
			sql.NullInt64{},  // video_duration_seconds
			sql.NullString{Valid: true, String: "priority"},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			false,
			false,
			sql.NullInt64{},   // channel_id
			sql.NullString{},  // model_mapping_chain
			sql.NullString{},  // billing_tier
			sql.NullString{},  // billing_mode
			sql.NullFloat64{}, // account_stats_cost
			sql.NullString{},  // session_id
			now,
		}})
		require.NoError(t, err)
		require.NotNil(t, log.ServiceTier)
		require.Equal(t, "priority", *log.ServiceTier)
	})

}
