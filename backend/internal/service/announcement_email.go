package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	AnnouncementEmailBroadcastPending       = "pending"
	AnnouncementEmailBroadcastRunning       = "running"
	AnnouncementEmailBroadcastCompleted     = "completed"
	AnnouncementEmailBroadcastPartialFailed = "partial_failed"

	AnnouncementEmailDeliveryPending    = "pending"
	AnnouncementEmailDeliveryProcessing = "processing"
	AnnouncementEmailDeliverySent       = "sent"
	AnnouncementEmailDeliveryFailed     = "failed"
)

var (
	ErrAnnouncementEmailAlreadyExists = infraerrors.Conflict("ANNOUNCEMENT_EMAIL_BROADCAST_EXISTS", "announcement email broadcast already exists")
	ErrAnnouncementEmailNotActive     = infraerrors.BadRequest("ANNOUNCEMENT_EMAIL_NOT_ACTIVE", "announcement is not currently active")
	ErrAnnouncementEmailNoRecipients  = infraerrors.BadRequest("ANNOUNCEMENT_EMAIL_NO_RECIPIENTS", "announcement has no eligible email recipients")
	ErrAnnouncementEmailNotFound      = infraerrors.NotFound("ANNOUNCEMENT_EMAIL_BROADCAST_NOT_FOUND", "announcement email broadcast not found")
	ErrAnnouncementEmailNoFailures    = infraerrors.BadRequest("ANNOUNCEMENT_EMAIL_NO_FAILURES", "announcement email broadcast has no failed deliveries")
	ErrAnnouncementEmailTooMany       = infraerrors.BadRequest("ANNOUNCEMENT_EMAIL_TOO_MANY_RECIPIENTS", "announcement email broadcast exceeds 10000 recipients")
	ErrAnnouncementDeleteBroadcast    = infraerrors.Conflict("ANNOUNCEMENT_EMAIL_BROADCAST_DELETE_BLOCKED", "announcement with an email broadcast cannot be deleted")
)

type AnnouncementEmailBroadcast struct {
	ID             int64      `json:"id"`
	AnnouncementID int64      `json:"announcement_id"`
	Status         string     `json:"status"`
	TotalCount     int        `json:"total_count"`
	SentCount      int        `json:"sent_count"`
	FailedCount    int        `json:"failed_count"`
	CreatedBy      *int64     `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

func (b AnnouncementEmailBroadcast) PendingCount() int {
	pending := b.TotalCount - b.SentCount - b.FailedCount
	if pending < 0 {
		return 0
	}
	return pending
}

type AnnouncementEmailDelivery struct {
	ID            int64      `json:"id"`
	BroadcastID   int64      `json:"broadcast_id"`
	UserID        *int64     `json:"user_id,omitempty"`
	Email         string     `json:"email"`
	Status        string     `json:"status"`
	AttemptCount  int        `json:"attempt_count"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	Subject       string     `json:"-"`
	BodyHTML      string     `json:"-"`
}

type AnnouncementEmailRecipient struct {
	UserID int64
	Email  string
}

type AnnouncementEmailOverview struct {
	Broadcast     *AnnouncementEmailBroadcast `json:"broadcast"`
	EligibleCount int                         `json:"eligible_count"`
	CanSend       bool                        `json:"can_send"`
}

type AnnouncementEmailBroadcastRepository interface {
	GetByAnnouncementID(ctx context.Context, announcementID int64) (*AnnouncementEmailBroadcast, error)
	ExistsByAnnouncementID(ctx context.Context, announcementID int64) (bool, error)
	Create(ctx context.Context, announcementID int64, subject, bodyHTML string, createdBy int64, recipients []AnnouncementEmailRecipient) (*AnnouncementEmailBroadcast, error)
	ListDeliveries(ctx context.Context, broadcastID int64, params pagination.PaginationParams, status, search string) ([]AnnouncementEmailDelivery, *pagination.PaginationResult, error)
	RetryFailed(ctx context.Context, broadcastID int64) (*AnnouncementEmailBroadcast, error)
	ClaimNext(ctx context.Context, leaseUntil time.Time) (*AnnouncementEmailDelivery, error)
	MarkSent(ctx context.Context, deliveryID int64, sentAt time.Time) error
	MarkFailed(ctx context.Context, deliveryID int64, message string, failedAt time.Time) error
}
