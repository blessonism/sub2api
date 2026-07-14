package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

const userGroupAccountBindingCacheTTL = 30 * time.Second

type UserGroupAccountBinding struct {
	AccountIDs      []int64 `json:"account_ids"`
	FallbackToGroup bool    `json:"fallback_to_group"`
}

type UserGroupAccountBindingRepository interface {
	GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*UserGroupAccountBinding, error)
	GetByUserID(ctx context.Context, userID int64) (map[int64]UserGroupAccountBinding, error)
	ReplaceByUserID(ctx context.Context, userID int64, bindings map[int64]UserGroupAccountBinding) error
}

type userGroupAccountBindingCacheEntry struct {
	Binding *UserGroupAccountBinding
}

type userGroupAccountBindingRequestState struct {
	allowedAccountIDs map[int64]struct{}
}

type userGroupAccountBindingContextKey struct{}

// UserGroupAccountBindingResolver 是实时调度与管理员配置共享的绑定读取入口。
type UserGroupAccountBindingResolver struct {
	repo  UserGroupAccountBindingRepository
	cache *gocache.Cache
	sf    singleflight.Group
}

func NewUserGroupAccountBindingResolver(repo UserGroupAccountBindingRepository) *UserGroupAccountBindingResolver {
	return &UserGroupAccountBindingResolver{
		repo:  repo,
		cache: gocache.New(userGroupAccountBindingCacheTTL, time.Minute),
	}
}

func (r *UserGroupAccountBindingResolver) Get(ctx context.Context, userID, groupID int64) (*UserGroupAccountBinding, error) {
	if r == nil || r.repo == nil || userID <= 0 || groupID <= 0 {
		return nil, nil
	}
	key := fmt.Sprintf("%d:%d", userID, groupID)
	if cached, ok := r.cache.Get(key); ok {
		return cloneUserGroupAccountBinding(cached.(userGroupAccountBindingCacheEntry).Binding), nil
	}

	value, err, _ := r.sf.Do(key, func() (any, error) {
		if cached, ok := r.cache.Get(key); ok {
			return cached.(userGroupAccountBindingCacheEntry).Binding, nil
		}
		binding, err := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if err != nil {
			return nil, err
		}
		binding = cloneUserGroupAccountBinding(binding)
		r.cache.SetDefault(key, userGroupAccountBindingCacheEntry{Binding: binding})
		return binding, nil
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return cloneUserGroupAccountBinding(value.(*UserGroupAccountBinding)), nil
}

func (r *UserGroupAccountBindingResolver) ListByUserID(ctx context.Context, userID int64) (map[int64]UserGroupAccountBinding, error) {
	if r == nil || r.repo == nil {
		return nil, nil
	}
	return r.repo.GetByUserID(ctx, userID)
}

func (r *UserGroupAccountBindingResolver) ReplaceByUserID(ctx context.Context, userID int64, bindings map[int64]UserGroupAccountBinding) error {
	if r == nil || r.repo == nil {
		return fmt.Errorf("user group account binding repository is unavailable")
	}
	if err := r.repo.ReplaceByUserID(ctx, userID, bindings); err != nil {
		return err
	}
	// 管理配置写入很少，整表缓存失效比维护用户到缓存键的反向索引更可靠。
	r.cache.Flush()
	return nil
}

func cloneUserGroupAccountBinding(binding *UserGroupAccountBinding) *UserGroupAccountBinding {
	if binding == nil {
		return nil
	}
	cloned := *binding
	cloned.AccountIDs = append([]int64(nil), binding.AccountIDs...)
	sort.Slice(cloned.AccountIDs, func(i, j int) bool { return cloned.AccountIDs[i] < cloned.AccountIDs[j] })
	return &cloned
}

func resolveRequestUserGroupAccountBinding(ctx context.Context, resolver *UserGroupAccountBindingResolver, groupID *int64) (*UserGroupAccountBinding, bool, error) {
	if _, handled := ctx.Value(userGroupAccountBindingContextKey{}).(userGroupAccountBindingRequestState); handled {
		return nil, false, nil
	}
	if resolver == nil || groupID == nil || *groupID <= 0 {
		return nil, false, nil
	}
	userID, _ := ctx.Value(ctxkey.UserID).(int64)
	if userID <= 0 {
		return nil, false, nil
	}
	binding, err := resolver.Get(ctx, userID, *groupID)
	if err != nil {
		return nil, false, fmt.Errorf("load user group account binding: %w", err)
	}
	return binding, binding != nil, nil
}

func withUserGroupAccountBinding(ctx context.Context, accountIDs []int64) context.Context {
	allowed := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		allowed[accountID] = struct{}{}
	}
	return context.WithValue(ctx, userGroupAccountBindingContextKey{}, userGroupAccountBindingRequestState{allowedAccountIDs: allowed})
}

func withoutUserGroupAccountBinding(ctx context.Context) context.Context {
	return context.WithValue(ctx, userGroupAccountBindingContextKey{}, userGroupAccountBindingRequestState{})
}

func isAccountExcludedForRequest(ctx context.Context, excludedIDs map[int64]struct{}, accountID int64) bool {
	if _, excluded := excludedIDs[accountID]; excluded {
		return true
	}
	state, ok := ctx.Value(userGroupAccountBindingContextKey{}).(userGroupAccountBindingRequestState)
	if !ok || state.allowedAccountIDs == nil {
		return false
	}
	_, allowed := state.allowedAccountIDs[accountID]
	return !allowed
}

func excludeUserGroupBoundAccounts(excludedIDs map[int64]struct{}, accountIDs []int64) map[int64]struct{} {
	result := make(map[int64]struct{}, len(excludedIDs)+len(accountIDs))
	for accountID := range excludedIDs {
		result[accountID] = struct{}{}
	}
	for _, accountID := range accountIDs {
		result[accountID] = struct{}{}
	}
	return result
}

func isNoAvailableAccountSelectionError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNoAvailableAccounts) || errors.Is(err, ErrNoAvailableCompactAccounts) || strings.Contains(strings.ToLower(err.Error()), "no available")
}
