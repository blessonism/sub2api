//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type userGroupRateRepoStubForUpdateUser struct {
	syncedUserID        int64
	syncedRates         map[int64]*float64
	syncedVisibleUserID int64
	syncedVisibleRates  map[int64]*float64
}

func (s *userGroupRateRepoStubForUpdateUser) GetByUserID(context.Context, int64) (map[int64]float64, error) {
	panic("unexpected GetByUserID call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetVisibleByUserID(context.Context, int64) (map[int64]float64, error) {
	panic("unexpected GetVisibleByUserID call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetVisibleByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	panic("unexpected GetVisibleByUserAndGroup call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetRPMOverrideByUserAndGroup(context.Context, int64, int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetByGroupID(context.Context, int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateRepoStubForUpdateUser) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	s.syncedUserID = userID
	s.syncedRates = rates
	return nil
}

func (s *userGroupRateRepoStubForUpdateUser) SyncUserGroupVisibleRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	s.syncedVisibleUserID = userID
	s.syncedVisibleRates = rates
	return nil
}

func (s *userGroupRateRepoStubForUpdateUser) SyncGroupRateMultipliers(context.Context, int64, []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForUpdateUser) SyncGroupRPMOverrides(context.Context, int64, []GroupRPMOverrideInput) error {
	panic("unexpected SyncGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForUpdateUser) ClearGroupRPMOverrides(context.Context, int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForUpdateUser) ClearGroupRateMultipliers(context.Context, int64) error {
	panic("unexpected ClearGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForUpdateUser) DeleteByGroupID(context.Context, int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateRepoStubForUpdateUser) DeleteByUserID(context.Context, int64) error {
	panic("unexpected DeleteByUserID call")
}

func TestAdminService_UpdateUser_SyncsVisibleGroupRates(t *testing.T) {
	userRepo := &userRepoStub{user: &User{ID: 7, Email: "alice@example.com", Status: StatusActive}}
	rateRepo := &userGroupRateRepoStubForUpdateUser{}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             userRepo,
		userGroupRateRepo:    rateRepo,
		authCacheInvalidator: invalidator,
	}
	visibleRate := ptrFloat(0.9)
	visibleRates := map[int64]*float64{
		11: visibleRate,
		12: nil,
	}

	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{VisibleGroupRates: visibleRates})

	require.NoError(t, err)
	require.Equal(t, int64(7), rateRepo.syncedVisibleUserID)
	require.Equal(t, visibleRates, rateRepo.syncedVisibleRates)
	require.Zero(t, rateRepo.syncedUserID)
	require.Equal(t, []int64{7}, invalidator.userIDs)
}

func TestAdminService_UpdateUser_RejectsInvalidVisibleGroupRate(t *testing.T) {
	userRepo := &userRepoStub{user: &User{ID: 7, Email: "alice@example.com", Status: StatusActive}}
	rateRepo := &userGroupRateRepoStubForUpdateUser{}
	svc := &adminServiceImpl{
		userRepo:          userRepo,
		userGroupRateRepo: rateRepo,
	}
	invalidRate := ptrFloat(0)

	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{
		VisibleGroupRates: map[int64]*float64{11: invalidRate},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "visible_rate_multiplier must be > 0")
	require.Zero(t, rateRepo.syncedVisibleUserID)
}
