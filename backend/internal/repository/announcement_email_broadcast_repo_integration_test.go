//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementEmailBroadcastRepositoryDeliveryLifecycle(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("announcement-email-%d@example.com", suffix)).
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	announcement, err := integrationEntClient.Announcement.Create().
		SetTitle("Maintenance").
		SetContent("Details").
		SetStatus(service.AnnouncementStatusActive).
		SetNotifyMode(service.AnnouncementNotifyModeSilent).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM announcement_email_broadcasts WHERE announcement_id = $1`, announcement.ID)
		_ = integrationEntClient.Announcement.DeleteOneID(announcement.ID).Exec(ctx)
		_ = integrationEntClient.User.DeleteOneID(user.ID).Exec(ctx)
	})

	repo := NewAnnouncementEmailBroadcastRepository(integrationDB)
	broadcast, err := repo.Create(ctx, announcement.ID, "Subject", "<p>Body</p>", user.ID, []service.AnnouncementEmailRecipient{{
		UserID: user.ID,
		Email:  user.Email,
	}})
	require.NoError(t, err)
	require.Equal(t, 1, broadcast.TotalCount)

	delivery, err := repo.ClaimNext(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, user.Email, delivery.Email)
	require.Equal(t, 1, delivery.AttemptCount)
	_, err = integrationDB.ExecContext(ctx, `UPDATE announcement_email_deliveries SET lease_expires_at = NOW() - INTERVAL '1 second' WHERE id = $1`, delivery.ID)
	require.NoError(t, err)
	delivery, err = repo.ClaimNext(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, 2, delivery.AttemptCount)
	require.NoError(t, repo.MarkFailed(ctx, delivery.ID, "temporary failure", time.Now()))

	broadcast, err = repo.GetByAnnouncementID(ctx, announcement.ID)
	require.NoError(t, err)
	require.Equal(t, service.AnnouncementEmailBroadcastPartialFailed, broadcast.Status)
	require.Equal(t, 1, broadcast.FailedCount)

	broadcast, err = repo.RetryFailed(ctx, broadcast.ID)
	require.NoError(t, err)
	require.Equal(t, service.AnnouncementEmailBroadcastPending, broadcast.Status)
	delivery, err = repo.ClaimNext(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, 3, delivery.AttemptCount)
	require.NoError(t, repo.MarkSent(ctx, delivery.ID, time.Now()))

	broadcast, err = repo.GetByAnnouncementID(ctx, announcement.ID)
	require.NoError(t, err)
	require.Equal(t, service.AnnouncementEmailBroadcastCompleted, broadcast.Status)
	require.Equal(t, 1, broadcast.SentCount)
	require.Equal(t, 0, broadcast.FailedCount)

	err = NewAnnouncementRepository(integrationEntClient).Delete(ctx, announcement.ID)
	require.ErrorIs(t, err, service.ErrAnnouncementDeleteBroadcast)
}
