//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *UserRepoSuite) mustInsertUsageLog(userID int64, createdAt time.Time) {
	s.T().Helper()

	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-log-account"})
	s.mustInsertUsageLogCost(userID, account, 0.01, createdAt)
}

func (s *UserRepoSuite) mustInsertUsageLogCost(userID int64, account *service.Account, actualCost float64, createdAt time.Time) {
	s.T().Helper()

	apiKey := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: userID})

	_, err := integrationDB.ExecContext(
		s.ctx,
		`INSERT INTO usage_logs (user_id, api_key_id, account_id, model, input_tokens, output_tokens, total_cost, actual_cost, created_at)
		 VALUES ($1, $2, $3, 'gpt-test', 1, 1, $4, $5, $6)`,
		userID,
		apiKey.ID,
		account.ID,
		actualCost,
		actualCost,
		createdAt.UTC(),
	)
	s.Require().NoError(err)
}

func (s *UserRepoSuite) TestListWithFilters_SortByEmailAsc() {
	s.mustCreateUser(&service.User{Email: "z-last@example.com", Username: "z-user"})
	s.mustCreateUser(&service.User{Email: "a-first@example.com", Username: "a-user"})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "email",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal("a-first@example.com", users[0].Email)
	s.Require().Equal("z-last@example.com", users[1].Email)
}

func (s *UserRepoSuite) TestList_DefaultSortByNewestFirst() {
	first := s.mustCreateUser(&service.User{Email: "first@example.com"})
	second := s.mustCreateUser(&service.User{Email: "second@example.com"})

	users, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal(second.ID, users[0].ID)
	s.Require().Equal(first.ID, users[1].ID)
}

func (s *UserRepoSuite) TestCreateAndRead_PreservesSignupSourceAndActivityTimestamps() {
	lastLoginAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Microsecond)

	created := s.mustCreateUser(&service.User{
		Email:        "identity-meta@example.com",
		SignupSource: "linuxdo",
		LastLoginAt:  &lastLoginAt,
		LastActiveAt: &lastActiveAt,
	})

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("linuxdo", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestUpdate_PersistsSignupSourceAndActivityTimestamps() {
	created := s.mustCreateUser(&service.User{Email: "identity-update@example.com"})
	lastLoginAt := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-15 * time.Minute).UTC().Truncate(time.Microsecond)

	created.SignupSource = "oidc"
	created.LastLoginAt = &lastLoginAt
	created.LastActiveAt = &lastActiveAt

	s.Require().NoError(s.repo.Update(s.ctx, created, service.UserUpdateFields{SignupSource: true, LastLoginAt: true, LastActiveAt: true}))

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("oidc", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastActiveAtAsc() {
	earlier := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Microsecond)
	later := time.Now().Add(-45 * time.Minute).UTC().Truncate(time.Microsecond)

	s.mustCreateUser(&service.User{Email: "nil-active@example.com"})
	s.mustCreateUser(&service.User{Email: "later-active@example.com", LastActiveAt: &later})
	s.mustCreateUser(&service.User{Email: "earlier-active@example.com", LastActiveAt: &earlier})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_active_at",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal("earlier-active@example.com", users[0].Email)
	s.Require().Equal("later-active@example.com", users[1].Email)
	s.Require().Equal("nil-active@example.com", users[2].Email)
}

func (s *UserRepoSuite) TestGetLatestUsedAtByUserIDs_UsesUsageLogs() {
	older := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	newer := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Second)

	userWithUsage := s.mustCreateUser(&service.User{Email: "usage-source@example.com"})
	userWithoutUsage := s.mustCreateUser(&service.User{Email: "usage-missing@example.com"})
	s.mustInsertUsageLog(userWithUsage.ID, older)
	s.mustInsertUsageLog(userWithUsage.ID, newer)

	got, err := s.repo.GetLatestUsedAtByUserIDs(s.ctx, []int64{userWithUsage.ID, userWithoutUsage.ID})
	s.Require().NoError(err)
	s.Require().Contains(got, userWithUsage.ID)
	s.Require().NotContains(got, userWithoutUsage.ID)
	s.Require().NotNil(got[userWithUsage.ID])
	s.Require().True(got[userWithUsage.ID].Equal(newer))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastUsedAtDesc_UsesUsageLogsNotLastActiveAt() {
	lastUsedOlder := time.Now().Add(-6 * time.Hour).UTC().Truncate(time.Second)
	lastUsedNewer := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	lastActiveVeryRecent := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)

	nilUsage := s.mustCreateUser(&service.User{Email: "nil-last-used@example.com"})
	wrongSource := s.mustCreateUser(&service.User{
		Email:        "active-not-usage@example.com",
		LastActiveAt: &lastActiveVeryRecent,
	})
	rightSource := s.mustCreateUser(&service.User{Email: "usage-wins@example.com"})

	s.mustInsertUsageLog(wrongSource.ID, lastUsedOlder)
	s.mustInsertUsageLog(rightSource.ID, lastUsedNewer)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_used_at",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal(rightSource.ID, users[0].ID)
	s.Require().Equal(wrongSource.ID, users[1].ID)
	s.Require().Equal(nilUsage.ID, users[2].ID)
}

func (s *UserRepoSuite) TestListWithFilters_SortByUsageTotalDescBeforePagination() {
	low := s.mustCreateUser(&service.User{Email: "usage-low@example.com"})
	high := s.mustCreateUser(&service.User{Email: "usage-high@example.com"})
	mid := s.mustCreateUser(&service.User{Email: "usage-mid@example.com"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-sort-account"})
	now := time.Now().UTC()

	s.mustInsertUsageLogCost(low.ID, account, 1.00, now)
	s.mustInsertUsageLogCost(high.ID, account, 9.00, now)
	s.mustInsertUsageLogCost(mid.ID, account, 5.00, now)

	users, page, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  1,
		SortBy:    "usage_total",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(int64(3), page.Total)
	s.Require().Equal(high.ID, users[0].ID)

	users, _, err = s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      2,
		PageSize:  1,
		SortBy:    "usage_total",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(mid.ID, users[0].ID)
}

func (s *UserRepoSuite) TestListWithFilters_SortByUsageTotalIncludesBalanceCalibrationSpend() {
	logOnly := s.mustCreateUser(&service.User{Email: "usage-log-only@example.com"})
	calibrated := s.mustCreateUser(&service.User{Email: "usage-calibrated@example.com"})
	admin := s.mustCreateUser(&service.User{Email: "usage-calibration-admin@example.com", Role: service.RoleAdmin})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-calibration-sort-account"})
	now := time.Now().UTC()

	s.mustInsertUsageLogCost(logOnly.ID, account, 7.00, now)
	s.mustInsertUsageLogCost(calibrated.ID, account, 1.00, now)
	var calibrationID int64
	err := integrationDB.QueryRowContext(
		s.ctx,
		`INSERT INTO admin_usage_calibrations (
			target_user_id, admin_user_id, reason,
			balance_mode, balance_input_value, balance_before_value, balance_after_value, balance_delta,
			created_at
		) VALUES ($1, $2, 'sort calibration spend', 'delta', -10, 20, 10, -10, $3)
		RETURNING id`,
		calibrated.ID,
		admin.ID,
		now.AddDate(0, 0, -60),
	).Scan(&calibrationID)
	s.Require().NoError(err)
	_, err = integrationDB.ExecContext(
		s.ctx,
		`INSERT INTO admin_usage_calibration_daily_allocations (
			calibration_id, target_user_id, allocation_date, original_tokens, token_delta, balance_delta
		) VALUES ($1, $2, $3::date, 100, 0, -10)`,
		calibrationID,
		calibrated.ID,
		now.Format("2006-01-02"),
	)
	s.Require().NoError(err)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "usage_total",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal(calibrated.ID, users[0].ID)
	s.Require().Equal(logOnly.ID, users[1].ID)
}

func (s *UserRepoSuite) TestListWithFilters_SortByGrokUsageTotalDescBeforePagination() {
	grokLow := s.mustCreateUser(&service.User{Email: "usage-grok-low@example.com"})
	grokHigh := s.mustCreateUser(&service.User{Email: "usage-grok-high@example.com"})
	openAIOnly := s.mustCreateUser(&service.User{Email: "usage-openai-only@example.com"})
	grokAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-sort-grok-account", Platform: service.PlatformGrok})
	openAIAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-sort-openai-account", Platform: service.PlatformOpenAI})
	now := time.Now().UTC()

	s.mustInsertUsageLogCost(grokLow.ID, grokAccount, 2.00, now)
	s.mustInsertUsageLogCost(grokHigh.ID, grokAccount, 8.00, now)
	s.mustInsertUsageLogCost(openAIOnly.ID, openAIAccount, 20.00, now)

	users, page, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  1,
		SortBy:    "usage_grok_total",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(int64(3), page.Total)
	s.Require().Equal(grokHigh.ID, users[0].ID)

	users, _, err = s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      2,
		PageSize:  1,
		SortBy:    "usage_grok_total",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(grokLow.ID, users[0].ID)
}

func TestUserRepoSortSuiteSmoke(_ *testing.T) {}
