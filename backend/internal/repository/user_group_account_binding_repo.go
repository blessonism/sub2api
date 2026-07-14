package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type userGroupAccountBindingRepository struct {
	db *sql.DB
}

func NewUserGroupAccountBindingRepository(db *sql.DB) service.UserGroupAccountBindingRepository {
	return &userGroupAccountBindingRepository{db: db}
}

func (r *userGroupAccountBindingRepository) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*service.UserGroupAccountBinding, error) {
	var accountIDs pq.Int64Array
	var fallback bool
	err := r.db.QueryRowContext(ctx, `
		SELECT account_ids, fallback_to_group
		FROM user_group_account_bindings
		WHERE user_id = $1 AND group_id = $2
	`, userID, groupID).Scan(&accountIDs, &fallback)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &service.UserGroupAccountBinding{AccountIDs: []int64(accountIDs), FallbackToGroup: fallback}, nil
}

func (r *userGroupAccountBindingRepository) GetByUserID(ctx context.Context, userID int64) (map[int64]service.UserGroupAccountBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT group_id, account_ids, fallback_to_group
		FROM user_group_account_bindings
		WHERE user_id = $1
		ORDER BY group_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	bindings := make(map[int64]service.UserGroupAccountBinding)
	for rows.Next() {
		var groupID int64
		var accountIDs pq.Int64Array
		var fallback bool
		if err := rows.Scan(&groupID, &accountIDs, &fallback); err != nil {
			return nil, err
		}
		bindings[groupID] = service.UserGroupAccountBinding{AccountIDs: []int64(accountIDs), FallbackToGroup: fallback}
	}
	return bindings, rows.Err()
}

func (r *userGroupAccountBindingRepository) ReplaceByUserID(ctx context.Context, userID int64, bindings map[int64]service.UserGroupAccountBinding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user group account binding transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_group_account_bindings WHERE user_id = $1`, userID); err != nil {
		return err
	}

	groupIDs := make([]int64, 0, len(bindings))
	for groupID := range bindings {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	now := time.Now()
	for _, groupID := range groupIDs {
		binding := bindings[groupID]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_group_account_bindings
				(user_id, group_id, account_ids, fallback_to_group, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, userID, groupID, pq.Array(binding.AccountIDs), binding.FallbackToGroup, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user group account binding transaction: %w", err)
	}
	return nil
}
