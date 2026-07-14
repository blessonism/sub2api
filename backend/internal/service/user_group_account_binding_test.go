//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type userGroupAccountBindingRepoStub struct {
	binding *UserGroupAccountBinding
	err     error
}

type groupAccountBindingValidationGroupRepo struct {
	GroupRepository
	group      *Group
	accountIDs []int64
}

func (r groupAccountBindingValidationGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func (r groupAccountBindingValidationGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return r.accountIDs, nil
}

type groupAccountBindingValidationAccountRepo struct {
	AccountRepository
	accounts map[int64]*Account
}

func (r groupAccountBindingValidationAccountRepo) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	result := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if account := r.accounts[id]; account != nil {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *userGroupAccountBindingRepoStub) GetByUserAndGroup(context.Context, int64, int64) (*UserGroupAccountBinding, error) {
	return cloneUserGroupAccountBinding(r.binding), r.err
}

func (r *userGroupAccountBindingRepoStub) GetByUserID(context.Context, int64) (map[int64]UserGroupAccountBinding, error) {
	return nil, r.err
}

func (r *userGroupAccountBindingRepoStub) ReplaceByUserID(context.Context, int64, map[int64]UserGroupAccountBinding) error {
	return r.err
}

func bindingRequestContext(userID int64) context.Context {
	return context.WithValue(context.Background(), ctxkey.UserID, userID)
}

func TestGatewayUserGroupAccountBindingStrictAndFallback(t *testing.T) {
	groupID := int64(10)
	accounts := []Account{
		{ID: 1, Platform: PlatformAnthropic, Priority: 1, Status: StatusActive, Schedulable: true},
		{ID: 2, Platform: PlatformAnthropic, Priority: 2, Status: StatusActive, Schedulable: true},
	}
	repo := &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{1: &accounts[0], 2: &accounts[1]}}
	groupRepo := &mockGroupRepoForGateway{groups: map[int64]*Group{
		groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
	}}

	t.Run("strict selects only bound account", func(t *testing.T) {
		svc := &GatewayService{
			accountRepo: repo,
			groupRepo:   groupRepo,
			cfg:         testConfig(),
			userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{
				binding: &UserGroupAccountBinding{AccountIDs: []int64{2}},
			}),
		}
		account, err := svc.SelectAccountForModelWithExclusions(bindingRequestContext(7), &groupID, "", "claude-sonnet-4-6", nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), account.ID)
	})

	t.Run("fallback uses another group account", func(t *testing.T) {
		svc := &GatewayService{
			accountRepo: repo,
			groupRepo:   groupRepo,
			cfg:         testConfig(),
			userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{
				binding: &UserGroupAccountBinding{AccountIDs: []int64{99}, FallbackToGroup: true},
			}),
		}
		account, err := svc.SelectAccountForModelWithExclusions(bindingRequestContext(7), &groupID, "", "claude-sonnet-4-6", nil)
		require.NoError(t, err)
		require.Equal(t, int64(1), account.ID)
	})
}

func TestOpenAIAndGeminiUserGroupAccountBinding(t *testing.T) {
	groupID := int64(11)
	ctx := bindingRequestContext(8)

	t.Run("openai", func(t *testing.T) {
		repo := stubOpenAIAccountRepo{accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}}
		svc := &OpenAIGatewayService{
			accountRepo: repo,
			userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{
				binding: &UserGroupAccountBinding{AccountIDs: []int64{2}},
			}),
		}
		account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "gpt-4", nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), account.ID)
	})

	t.Run("gemini", func(t *testing.T) {
		accounts := []Account{
			{ID: 1, Platform: PlatformGemini, Type: AccountTypeOAuth, Priority: 1, Status: StatusActive, Schedulable: true},
			{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth, Priority: 2, Status: StatusActive, Schedulable: true},
		}
		repo := &mockAccountRepoForGemini{accounts: accounts, accountsByID: map[int64]*Account{1: &accounts[0], 2: &accounts[1]}}
		svc := &GeminiMessagesCompatService{
			accountRepo: repo,
			userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{
				binding: &UserGroupAccountBinding{AccountIDs: []int64{2}},
			}),
		}
		geminiCtx := context.WithValue(ctx, ctxkey.Group, &Group{ID: groupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true})
		account, err := svc.SelectAccountForModelWithExclusions(geminiCtx, &groupID, "", "gemini-2.5-flash", nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), account.ID)
	})
}

func TestUserGroupAccountBindingReadErrorFailsClosed(t *testing.T) {
	groupID := int64(12)
	svc := &GatewayService{
		userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{err: errors.New("db unavailable")}),
	}
	_, err := svc.SelectAccountForModelWithExclusions(bindingRequestContext(9), &groupID, "", "claude-sonnet-4-6", nil)
	require.ErrorContains(t, err, "load user group account binding")
}

func TestOpenAIUserGroupAccountBindingBusyPolicy(t *testing.T) {
	groupID := int64(13)
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}}
	concurrency := NewConcurrencyService(stubConcurrencyCache{acquireResults: map[int64]bool{1: false, 2: true}})

	for _, tc := range []struct {
		name        string
		fallback    bool
		wantID      int64
		wantAcquire bool
	}{
		{name: "strict waits for bound account", fallback: false, wantID: 1, wantAcquire: false},
		{name: "fallback immediately uses group account", fallback: true, wantID: 2, wantAcquire: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{
				accountRepo:        repo,
				concurrencyService: concurrency,
				userGroupAccountBindingResolver: NewUserGroupAccountBindingResolver(&userGroupAccountBindingRepoStub{
					binding: &UserGroupAccountBinding{AccountIDs: []int64{1}, FallbackToGroup: tc.fallback},
				}),
			}
			selection, err := svc.SelectAccountWithLoadAwareness(bindingRequestContext(10), &groupID, "", "gpt-4", nil)
			require.NoError(t, err)
			require.Equal(t, tc.wantID, selection.Account.ID)
			require.Equal(t, tc.wantAcquire, selection.Acquired)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}

func TestAccountBindingContextFiltersStickyAndCandidates(t *testing.T) {
	ctx := withUserGroupAccountBinding(context.Background(), []int64{2})
	require.True(t, isAccountExcludedForRequest(ctx, nil, 1))
	require.False(t, isAccountExcludedForRequest(ctx, nil, 2))
	require.True(t, isAccountExcludedForRequest(ctx, map[int64]struct{}{2: {}}, 2))
}

func TestValidateUserGroupAccountBindings(t *testing.T) {
	group := &Group{ID: 9, SubscriptionType: SubscriptionTypeStandard}
	svc := &adminServiceImpl{
		groupRepo: groupAccountBindingValidationGroupRepo{group: group, accountIDs: []int64{2}},
		accountRepo: groupAccountBindingValidationAccountRepo{accounts: map[int64]*Account{
			2: {ID: 2},
		}},
	}

	normalized, err := svc.validateUserGroupAccountBindings(context.Background(), map[int64]UserGroupAccountBinding{
		9: {AccountIDs: []int64{2, 2}, FallbackToGroup: true},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, normalized[9].AccountIDs)

	_, err = svc.validateUserGroupAccountBindings(context.Background(), map[int64]UserGroupAccountBinding{
		9: {AccountIDs: []int64{3}},
	})
	require.ErrorContains(t, err, "does not belong")

	group.IsExclusive = true
	_, err = svc.validateUserGroupAccountBindings(context.Background(), map[int64]UserGroupAccountBinding{
		9: {AccountIDs: []int64{2}},
	})
	require.ErrorContains(t, err, "standard public groups")
}
