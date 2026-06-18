package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const settingKeyAdminBalanceSummaryExcludedUserIDs = "admin_balance_summary_excluded_user_ids"

type balanceSummaryUserReader interface {
	ListBalanceSummaryUsers(ctx context.Context) ([]User, error)
}

func (s *adminServiceImpl) GetBalanceSummary(ctx context.Context) (*AdminBalanceSummary, error) {
	excludedIDs, err := s.loadBalanceSummaryExcludedUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildBalanceSummary(ctx, excludedIDs)
}

func (s *adminServiceImpl) UpdateBalanceSummaryExclusions(ctx context.Context, userIDs []int64) (*AdminBalanceSummary, error) {
	normalized, err := normalizeBalanceSummaryExcludedUserIDs(userIDs)
	if err != nil {
		return nil, err
	}

	users, err := s.listBalanceSummaryUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list balance summary users: %w", err)
	}
	existing := make(map[int64]struct{}, len(users))
	for _, u := range users {
		existing[u.ID] = struct{}{}
	}
	for _, id := range normalized {
		if _, ok := existing[id]; !ok {
			return nil, infraerrors.BadRequest("BALANCE_SUMMARY_EXCLUDED_USER_NOT_FOUND", fmt.Sprintf("excluded user %d does not exist or has been deleted", id))
		}
	}

	if err := s.saveBalanceSummaryExcludedUserIDs(ctx, normalized); err != nil {
		return nil, err
	}
	return buildAdminBalanceSummary(users, normalized), nil
}

func (s *adminServiceImpl) buildBalanceSummary(ctx context.Context, excludedIDs []int64) (*AdminBalanceSummary, error) {
	users, err := s.listBalanceSummaryUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list balance summary users: %w", err)
	}
	return buildAdminBalanceSummary(users, excludedIDs), nil
}

func (s *adminServiceImpl) listBalanceSummaryUsers(ctx context.Context) ([]User, error) {
	reader, ok := s.userRepo.(balanceSummaryUserReader)
	if !ok {
		return nil, infraerrors.InternalServer("BALANCE_SUMMARY_REPOSITORY_UNAVAILABLE", "balance summary repository is not available")
	}
	return reader.ListBalanceSummaryUsers(ctx)
}

func (s *adminServiceImpl) loadBalanceSummaryExcludedUserIDs(ctx context.Context) ([]int64, error) {
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, nil
	}
	raw, err := s.settingService.settingRepo.GetValue(ctx, settingKeyAdminBalanceSummaryExcludedUserIDs)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get balance summary exclusions: %w", err)
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("parse balance summary exclusions: %w", err)
	}
	return normalizeStoredBalanceSummaryExcludedUserIDs(ids), nil
}

func (s *adminServiceImpl) saveBalanceSummaryExcludedUserIDs(ctx context.Context, userIDs []int64) error {
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return infraerrors.InternalServer("SETTING_SERVICE_UNAVAILABLE", "setting service is not available")
	}
	data, err := json.Marshal(userIDs)
	if err != nil {
		return fmt.Errorf("marshal balance summary exclusions: %w", err)
	}
	if err := s.settingService.settingRepo.Set(ctx, settingKeyAdminBalanceSummaryExcludedUserIDs, string(data)); err != nil {
		return fmt.Errorf("save balance summary exclusions: %w", err)
	}
	if s.settingService.onUpdate != nil {
		s.settingService.onUpdate()
	}
	return nil
}

func normalizeBalanceSummaryExcludedUserIDs(userIDs []int64) ([]int64, error) {
	if len(userIDs) == 0 {
		return []int64{}, nil
	}
	seen := make(map[int64]struct{}, len(userIDs))
	result := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			return nil, infraerrors.BadRequest("BALANCE_SUMMARY_INVALID_USER_ID", "excluded user IDs must be positive integers")
		}
		if _, ok := seen[id]; ok {
			return nil, infraerrors.BadRequest("BALANCE_SUMMARY_DUPLICATE_USER_ID", fmt.Sprintf("excluded user %d is duplicated", id))
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func normalizeStoredBalanceSummaryExcludedUserIDs(userIDs []int64) []int64 {
	if len(userIDs) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(userIDs))
	result := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func buildAdminBalanceSummary(users []User, excludedIDs []int64) *AdminBalanceSummary {
	excludedSet := make(map[int64]struct{}, len(excludedIDs))
	for _, id := range excludedIDs {
		excludedSet[id] = struct{}{}
	}

	userByID := make(map[int64]User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}

	byRole := make(map[string]*AdminBalanceSummaryBucket)
	byStatus := make(map[string]*AdminBalanceSummaryBucket)
	summary := &AdminBalanceSummary{
		TotalUsers:        len(users),
		ExcludedUserIDs:   append([]int64(nil), excludedIDs...),
		ExcludedUsersList: make([]AdminBalanceSummaryExcludedUser, 0, len(excludedIDs)),
		GeneratedAt:       time.Now().UTC(),
	}

	for _, id := range excludedIDs {
		u, ok := userByID[id]
		if !ok {
			summary.InvalidExclusions++
			summary.ExcludedUsersList = append(summary.ExcludedUsersList, AdminBalanceSummaryExcludedUser{
				UserID: id,
				Valid:  false,
				Reason: "not_found_or_deleted",
			})
			continue
		}
		summary.ExcludedUserCount++
		summary.ExcludedUsersList = append(summary.ExcludedUsersList, AdminBalanceSummaryExcludedUser{
			UserID:   u.ID,
			Email:    u.Email,
			Username: u.Username,
			Role:     u.Role,
			Status:   u.Status,
			Balance:  u.Balance,
			Valid:    true,
		})
	}

	for _, u := range users {
		if _, excluded := excludedSet[u.ID]; excluded {
			incrementSummaryBucket(byRole, u.Role, 0, true)
			incrementSummaryBucket(byStatus, u.Status, 0, true)
			continue
		}
		summary.IncludedUsers++
		summary.TotalBalance += u.Balance
		incrementSummaryBucket(byRole, u.Role, u.Balance, false)
		incrementSummaryBucket(byStatus, u.Status, u.Balance, false)
	}

	summary.ByRole = sortedBalanceSummaryBuckets(byRole)
	summary.ByStatus = sortedBalanceSummaryBuckets(byStatus)
	return summary
}

func incrementSummaryBucket(buckets map[string]*AdminBalanceSummaryBucket, key string, balance float64, excluded bool) {
	if key == "" {
		key = "unknown"
	}
	b, ok := buckets[key]
	if !ok {
		b = &AdminBalanceSummaryBucket{Key: key}
		buckets[key] = b
	}
	if excluded {
		b.ExcludedCount++
		return
	}
	b.UserCount++
	b.Balance += balance
}

func sortedBalanceSummaryBuckets(buckets map[string]*AdminBalanceSummaryBucket) []AdminBalanceSummaryBucket {
	result := make([]AdminBalanceSummaryBucket, 0, len(buckets))
	for _, b := range buckets {
		result = append(result, *b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}
