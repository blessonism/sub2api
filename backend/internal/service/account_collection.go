package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AccountCollection struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccountCollectionService struct{ db *sql.DB }

func NewAccountCollectionService(db *sql.DB) *AccountCollectionService {
	return &AccountCollectionService{db: db}
}

func (s *AccountCollectionService) List(ctx context.Context) ([]AccountCollection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, sort_order, created_at, updated_at FROM account_collections WHERE deleted_at IS NULL ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccountCollection, 0)
	for rows.Next() {
		var item AccountCollection
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func normalizeAccountCollectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("account collection name is required")
	}
	if len([]rune(name)) > 100 {
		return "", errors.New("account collection name must not exceed 100 characters")
	}
	return name, nil
}

func (s *AccountCollectionService) Create(ctx context.Context, name string) (*AccountCollection, error) {
	name, err := normalizeAccountCollectionName(name)
	if err != nil {
		return nil, err
	}
	item := &AccountCollection{}
	err = s.db.QueryRowContext(ctx, `INSERT INTO account_collections (name, sort_order) VALUES ($1, COALESCE((SELECT MAX(sort_order) + 1 FROM account_collections WHERE deleted_at IS NULL), 0)) RETURNING id, name, sort_order, created_at, updated_at`, name).Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (s *AccountCollectionService) Update(ctx context.Context, id int64, name string) (*AccountCollection, error) {
	name, err := normalizeAccountCollectionName(name)
	if err != nil {
		return nil, err
	}
	item := &AccountCollection{}
	err = s.db.QueryRowContext(ctx, `UPDATE account_collections SET name=$2, updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL RETURNING id, name, sort_order, created_at, updated_at`, id, name).Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (s *AccountCollectionService) Delete(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE account_collections SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_collection_members WHERE account_collection_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountCollectionService) UpdateSort(ctx context.Context, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		result, err := tx.ExecContext(ctx, `UPDATE account_collections SET sort_order=$2, updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, i)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("account collection %d not found", id)
		}
	}
	return tx.Commit()
}

func (s *AccountCollectionService) ListForAccount(ctx context.Context, accountID int64) ([]AccountCollection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.name, c.sort_order, c.created_at, c.updated_at FROM account_collections c JOIN account_collection_members m ON m.account_collection_id=c.id WHERE m.account_id=$1 AND c.deleted_at IS NULL ORDER BY c.sort_order, c.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccountCollection, 0)
	for rows.Next() {
		var item AccountCollection
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *AccountCollectionService) BatchMembers(ctx context.Context, accountIDs, collectionIDs []int64, operation string) error {
	if len(accountIDs) == 0 || len(collectionIDs) == 0 {
		return errors.New("account_ids and account_collection_ids are required")
	}
	if operation != "add" && operation != "remove" {
		return errors.New("operation must be add or remove")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, accountID := range accountIDs {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM accounts WHERE id=$1 AND deleted_at IS NULL)`, accountID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("account %d not found", accountID)
		}
	}
	for _, collectionID := range collectionIDs {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM account_collections WHERE id=$1 AND deleted_at IS NULL)`, collectionID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("account collection %d not found", collectionID)
		}
	}
	for _, accountID := range accountIDs {
		for _, collectionID := range collectionIDs {
			if operation == "add" {
				_, err = tx.ExecContext(ctx, `INSERT INTO account_collection_members (account_id, account_collection_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, accountID, collectionID)
			} else {
				_, err = tx.ExecContext(ctx, `DELETE FROM account_collection_members WHERE account_id=$1 AND account_collection_id=$2`, accountID, collectionID)
			}
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

type accountCollectionFilterKey struct{}

func WithAccountCollectionFilter(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, accountCollectionFilterKey{}, id)
}
func AccountCollectionFilterFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(accountCollectionFilterKey{}).(int64)
	return id
}
