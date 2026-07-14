package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type announcementEmailBroadcastRepository struct{ db *sql.DB }

func NewAnnouncementEmailBroadcastRepository(db *sql.DB) service.AnnouncementEmailBroadcastRepository {
	return &announcementEmailBroadcastRepository{db: db}
}

const announcementEmailBroadcastSelect = `
SELECT id, announcement_id, status, total_count, sent_count, failed_count,
       created_by, created_at, started_at, completed_at
FROM announcement_email_broadcasts`

func scanAnnouncementEmailBroadcast(row interface{ Scan(...any) error }) (*service.AnnouncementEmailBroadcast, error) {
	var b service.AnnouncementEmailBroadcast
	err := row.Scan(&b.ID, &b.AnnouncementID, &b.Status, &b.TotalCount, &b.SentCount, &b.FailedCount,
		&b.CreatedBy, &b.CreatedAt, &b.StartedAt, &b.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAnnouncementEmailNotFound
	}
	return &b, err
}

func (r *announcementEmailBroadcastRepository) GetByAnnouncementID(ctx context.Context, announcementID int64) (*service.AnnouncementEmailBroadcast, error) {
	return scanAnnouncementEmailBroadcast(r.db.QueryRowContext(ctx, announcementEmailBroadcastSelect+` WHERE announcement_id = $1`, announcementID))
}

func (r *announcementEmailBroadcastRepository) ExistsByAnnouncementID(ctx context.Context, announcementID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM announcement_email_broadcasts WHERE announcement_id = $1)`, announcementID).Scan(&exists)
	return exists, err
}

func (r *announcementEmailBroadcastRepository) Create(
	ctx context.Context,
	announcementID int64,
	subject, bodyHTML string,
	createdBy int64,
	recipients []service.AnnouncementEmailRecipient,
) (*service.AnnouncementEmailBroadcast, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var broadcastID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO announcement_email_broadcasts (announcement_id, subject, body_html, total_count, created_by)
VALUES ($1, $2, $3, $4, NULLIF($5, 0))
RETURNING id`, announcementID, subject, bodyHTML, len(recipients), createdBy).Scan(&broadcastID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, service.ErrAnnouncementEmailAlreadyExists
		}
		return nil, err
	}

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("announcement_email_deliveries", "broadcast_id", "user_id", "email"))
	if err != nil {
		return nil, err
	}
	for _, recipient := range recipients {
		if _, err = stmt.ExecContext(ctx, broadcastID, recipient.UserID, recipient.Email); err != nil {
			_ = stmt.Close()
			return nil, err
		}
	}
	if _, err = stmt.ExecContext(ctx); err != nil {
		_ = stmt.Close()
		return nil, err
	}
	if err = stmt.Close(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByAnnouncementID(ctx, announcementID)
}

func (r *announcementEmailBroadcastRepository) ListDeliveries(
	ctx context.Context,
	broadcastID int64,
	params pagination.PaginationParams,
	status, search string,
) ([]service.AnnouncementEmailDelivery, *pagination.PaginationResult, error) {
	conditions := []string{"broadcast_id = $1"}
	args := []any{broadcastID}
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM announcement_email_deliveries WHERE `+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT id, broadcast_id, user_id, email, status, attempt_count, error_message,
       last_attempt_at, sent_at, created_at
FROM announcement_email_deliveries
WHERE `+where+fmt.Sprintf(` ORDER BY id ASC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]service.AnnouncementEmailDelivery, 0)
	for rows.Next() {
		var d service.AnnouncementEmailDelivery
		if err := rows.Scan(&d.ID, &d.BroadcastID, &d.UserID, &d.Email, &d.Status, &d.AttemptCount,
			&d.ErrorMessage, &d.LastAttemptAt, &d.SentAt, &d.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *announcementEmailBroadcastRepository) RetryFailed(ctx context.Context, broadcastID int64) (*service.AnnouncementEmailBroadcast, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
UPDATE announcement_email_deliveries
SET status = 'pending', lease_expires_at = NULL, error_message = NULL
WHERE broadcast_id = $1 AND status = 'failed'`, broadcastID)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, service.ErrAnnouncementEmailNoFailures
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE announcement_email_broadcasts
SET status = 'pending', failed_count = 0, completed_at = NULL
WHERE id = $1`, broadcastID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return scanAnnouncementEmailBroadcast(r.db.QueryRowContext(ctx, announcementEmailBroadcastSelect+` WHERE id = $1`, broadcastID))
}

func (r *announcementEmailBroadcastRepository) ClaimNext(ctx context.Context, leaseUntil time.Time) (*service.AnnouncementEmailDelivery, error) {
	now := time.Now()
	row := r.db.QueryRowContext(ctx, `
WITH candidate AS (
    SELECT id
    FROM announcement_email_deliveries
    WHERE status = 'pending'
       OR (status = 'processing' AND lease_expires_at < $1)
    ORDER BY id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
), claimed AS (
    UPDATE announcement_email_deliveries d
    SET status = 'processing', attempt_count = attempt_count + 1,
        last_attempt_at = $1, lease_expires_at = $2, error_message = NULL
    FROM candidate
    WHERE d.id = candidate.id
    RETURNING d.id, d.broadcast_id, d.user_id, d.email, d.attempt_count, d.created_at
)
UPDATE announcement_email_broadcasts b
SET status = 'running', started_at = COALESCE(started_at, $1)
FROM claimed c
WHERE b.id = c.broadcast_id
RETURNING c.id, c.broadcast_id, c.user_id, c.email, c.attempt_count, c.created_at, b.subject, b.body_html`, now, leaseUntil)
	var d service.AnnouncementEmailDelivery
	err := row.Scan(&d.ID, &d.BroadcastID, &d.UserID, &d.Email, &d.AttemptCount, &d.CreatedAt, &d.Subject, &d.BodyHTML)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Status = service.AnnouncementEmailDeliveryProcessing
	d.LastAttemptAt = &now
	return &d, nil
}

func (r *announcementEmailBroadcastRepository) MarkSent(ctx context.Context, deliveryID int64, sentAt time.Time) error {
	return r.finishDelivery(ctx, deliveryID, true, "", sentAt)
}

func (r *announcementEmailBroadcastRepository) MarkFailed(ctx context.Context, deliveryID int64, message string, failedAt time.Time) error {
	return r.finishDelivery(ctx, deliveryID, false, message, failedAt)
}

func (r *announcementEmailBroadcastRepository) finishDelivery(ctx context.Context, deliveryID int64, sent bool, message string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	status := service.AnnouncementEmailDeliveryFailed
	if sent {
		status = service.AnnouncementEmailDeliverySent
	}
	var broadcastID int64
	err = tx.QueryRowContext(ctx, `
UPDATE announcement_email_deliveries
SET status = $2::varchar, lease_expires_at = NULL, error_message = NULLIF($3, ''),
    sent_at = CASE WHEN $2::varchar = 'sent' THEN $4 ELSE sent_at END
WHERE id = $1 AND status = 'processing'
RETURNING broadcast_id`, deliveryID, status, message, now).Scan(&broadcastID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	sentDelta, failedDelta := 0, 1
	if sent {
		sentDelta, failedDelta = 1, 0
	}
	_, err = tx.ExecContext(ctx, `
UPDATE announcement_email_broadcasts
SET sent_count = sent_count + $2,
    failed_count = failed_count + $3,
    status = CASE
        WHEN sent_count + failed_count + $2 + $3 = total_count
            THEN CASE WHEN failed_count + $3 > 0 THEN 'partial_failed' ELSE 'completed' END
        ELSE 'running'
    END,
    completed_at = CASE WHEN sent_count + failed_count + $2 + $3 = total_count THEN $4::timestamptz ELSE NULL END
WHERE id = $1`, broadcastID, sentDelta, failedDelta, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}
