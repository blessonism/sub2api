//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type balanceSummaryUserRepoStub struct {
	userRepoStub
	users []User
	err   error
}

func (s *balanceSummaryUserRepoStub) ListBalanceSummaryUsers(context.Context) ([]User, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]User, len(s.users))
	copy(out, s.users)
	return out, nil
}

type balanceSummarySettingRepoStub struct {
	values map[string]string
}

func (s *balanceSummarySettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *balanceSummarySettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *balanceSummarySettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *balanceSummarySettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *balanceSummarySettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *balanceSummarySettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *balanceSummarySettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestAdminServiceGetBalanceSummaryNoExclusions(t *testing.T) {
	svc := &adminServiceImpl{
		userRepo: &balanceSummaryUserRepoStub{users: []User{
			{ID: 1, Role: RoleUser, Status: StatusActive, Balance: 10.5},
			{ID: 2, Role: RoleUser, Status: StatusDisabled, Balance: 2.25},
			{ID: 3, Role: RoleAdmin, Status: StatusActive, Balance: 100},
		}},
		settingService: &SettingService{settingRepo: &balanceSummarySettingRepoStub{}},
	}

	got, err := svc.GetBalanceSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, got.TotalUsers)
	require.Equal(t, 3, got.IncludedUsers)
	require.Equal(t, 0, got.ExcludedUserCount)
	require.Equal(t, 112.75, got.TotalBalance)
	require.ElementsMatch(t, []AdminBalanceSummaryBucket{
		{Key: RoleAdmin, UserCount: 1, Balance: 100},
		{Key: RoleUser, UserCount: 2, Balance: 12.75},
	}, got.ByRole)
	require.ElementsMatch(t, []AdminBalanceSummaryBucket{
		{Key: StatusActive, UserCount: 2, Balance: 110.5},
		{Key: StatusDisabled, UserCount: 1, Balance: 2.25},
	}, got.ByStatus)
}

func TestAdminServiceGetBalanceSummaryExcludesUsersAndReportsInvalidStoredIDs(t *testing.T) {
	svc := &adminServiceImpl{
		userRepo: &balanceSummaryUserRepoStub{users: []User{
			{ID: 1, Email: "a@example.com", Role: RoleUser, Status: StatusActive, Balance: 10},
			{ID: 2, Email: "b@example.com", Role: RoleAdmin, Status: StatusActive, Balance: 100},
		}},
		settingService: &SettingService{settingRepo: &balanceSummarySettingRepoStub{values: map[string]string{
			settingKeyAdminBalanceSummaryExcludedUserIDs: `[2,99,2,0,-7]`,
		}}},
	}

	got, err := svc.GetBalanceSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int64{2, 99}, got.ExcludedUserIDs)
	require.Equal(t, 2, got.TotalUsers)
	require.Equal(t, 1, got.IncludedUsers)
	require.Equal(t, 1, got.ExcludedUserCount)
	require.Equal(t, 1, got.InvalidExclusions)
	require.Equal(t, 10.0, got.TotalBalance)
	require.Len(t, got.ExcludedUsersList, 2)
	require.True(t, got.ExcludedUsersList[0].Valid)
	require.Equal(t, int64(2), got.ExcludedUsersList[0].UserID)
	require.False(t, got.ExcludedUsersList[1].Valid)
	require.Equal(t, "not_found_or_deleted", got.ExcludedUsersList[1].Reason)
}

func TestAdminServiceUpdateBalanceSummaryExclusionsValidatesAndSaves(t *testing.T) {
	settingRepo := &balanceSummarySettingRepoStub{}
	svc := &adminServiceImpl{
		userRepo: &balanceSummaryUserRepoStub{users: []User{
			{ID: 1, Role: RoleUser, Status: StatusActive, Balance: 10},
			{ID: 2, Role: RoleUser, Status: StatusActive, Balance: 20},
		}},
		settingService: &SettingService{settingRepo: settingRepo},
	}

	got, err := svc.UpdateBalanceSummaryExclusions(context.Background(), []int64{2})
	require.NoError(t, err)
	require.Equal(t, `[2]`, settingRepo.values[settingKeyAdminBalanceSummaryExcludedUserIDs])
	require.Equal(t, 1, got.IncludedUsers)
	require.Equal(t, 10.0, got.TotalBalance)
}

func TestAdminServiceUpdateBalanceSummaryExclusionsRejectsInvalidInput(t *testing.T) {
	svc := &adminServiceImpl{
		userRepo:       &balanceSummaryUserRepoStub{users: []User{{ID: 1}}},
		settingService: &SettingService{settingRepo: &balanceSummarySettingRepoStub{}},
	}

	_, err := svc.UpdateBalanceSummaryExclusions(context.Background(), []int64{1, 1})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))

	_, err = svc.UpdateBalanceSummaryExclusions(context.Background(), []int64{2})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}
