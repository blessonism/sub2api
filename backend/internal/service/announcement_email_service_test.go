package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestRenderAnnouncementEmailHTMLIsSafe(t *testing.T) {
	body, err := renderAnnouncementEmailHTML(`<Site>`, `<Title>`, "# Hello\n\n<script>alert(1)</script>\n\n**safe**")
	require.NoError(t, err)
	require.Contains(t, body, "&lt;Site&gt;")
	require.Contains(t, body, "&lt;Title&gt;")
	require.Contains(t, body, "<strong>safe</strong>")
	require.NotContains(t, body, "<script>")
}

func TestIsDeliverableAnnouncementEmail(t *testing.T) {
	require.True(t, isDeliverableAnnouncementEmail("user@example.com"))
	require.False(t, isDeliverableAnnouncementEmail("bad-address"))
	require.False(t, isDeliverableAnnouncementEmail("user"+OIDCConnectSyntheticEmailDomain))
}

type announcementEmailWorkerRepoStub struct {
	sentID   int64
	failedID int64
	message  string
}

func (*announcementEmailWorkerRepoStub) GetByAnnouncementID(context.Context, int64) (*AnnouncementEmailBroadcast, error) {
	return nil, ErrAnnouncementEmailNotFound
}
func (*announcementEmailWorkerRepoStub) ExistsByAnnouncementID(context.Context, int64) (bool, error) {
	return false, nil
}
func (*announcementEmailWorkerRepoStub) Create(context.Context, int64, string, string, int64, []AnnouncementEmailRecipient) (*AnnouncementEmailBroadcast, error) {
	return nil, nil
}
func (*announcementEmailWorkerRepoStub) ListDeliveries(context.Context, int64, pagination.PaginationParams, string, string) ([]AnnouncementEmailDelivery, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*announcementEmailWorkerRepoStub) RetryFailed(context.Context, int64) (*AnnouncementEmailBroadcast, error) {
	return nil, nil
}
func (*announcementEmailWorkerRepoStub) ClaimNext(context.Context, time.Time) (*AnnouncementEmailDelivery, error) {
	return nil, nil
}
func (s *announcementEmailWorkerRepoStub) MarkSent(_ context.Context, id int64, _ time.Time) error {
	s.sentID = id
	return nil
}
func (s *announcementEmailWorkerRepoStub) MarkFailed(_ context.Context, id int64, message string, _ time.Time) error {
	s.failedID = id
	s.message = message
	return nil
}

func TestAnnouncementEmailWorkerRecordsDeliveryResult(t *testing.T) {
	t.Run("sent", func(t *testing.T) {
		repo := &announcementEmailWorkerRepoStub{}
		worker := &AnnouncementEmailWorker{repo: repo, send: func(context.Context, string, string, string) error { return nil }}
		worker.deliver(&AnnouncementEmailDelivery{ID: 7, Email: "user@example.com"})
		require.Equal(t, int64(7), repo.sentID)
	})

	t.Run("failed and truncated", func(t *testing.T) {
		repo := &announcementEmailWorkerRepoStub{}
		worker := &AnnouncementEmailWorker{repo: repo, send: func(context.Context, string, string, string) error {
			return errors.New(strings.Repeat("x", 600))
		}}
		worker.deliver(&AnnouncementEmailDelivery{ID: 8, Email: "user@example.com"})
		require.Equal(t, int64(8), repo.failedID)
		require.Len(t, repo.message, 500)
	})
}
