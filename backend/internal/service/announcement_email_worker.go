package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	announcementEmailWorkerCount = 3
	announcementEmailPollDelay   = time.Second
	announcementEmailLease       = 2 * time.Minute
	announcementEmailSendTimeout = 30 * time.Second
)

type AnnouncementEmailWorker struct {
	repo    AnnouncementEmailBroadcastRepository
	send    func(context.Context, string, string, string) error
	stop    chan struct{}
	stopOne sync.Once
	wg      sync.WaitGroup
}

func NewAnnouncementEmailWorker(repo AnnouncementEmailBroadcastRepository, emailService *EmailService) *AnnouncementEmailWorker {
	w := &AnnouncementEmailWorker{
		repo: repo,
		send: emailService.SendEmail,
		stop: make(chan struct{}),
	}
	for i := 0; i < announcementEmailWorkerCount; i++ {
		w.wg.Add(1)
		go w.run()
	}
	return w
}

func (w *AnnouncementEmailWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOne.Do(func() { close(w.stop) })
	w.wg.Wait()
}

func (w *AnnouncementEmailWorker) run() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stop:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		delivery, err := w.repo.ClaimNext(ctx, time.Now().Add(announcementEmailLease))
		cancel()
		if err != nil {
			logger.L().Warn("announcement_email.claim_failed", zap.Error(err))
			if !w.wait() {
				return
			}
			continue
		}
		if delivery == nil {
			if !w.wait() {
				return
			}
			continue
		}
		w.deliver(delivery)
	}
}

func (w *AnnouncementEmailWorker) wait() bool {
	timer := time.NewTimer(announcementEmailPollDelay)
	defer timer.Stop()
	select {
	case <-w.stop:
		return false
	case <-timer.C:
		return true
	}
}

func (w *AnnouncementEmailWorker) deliver(delivery *AnnouncementEmailDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), announcementEmailSendTimeout)
	err := w.send(ctx, delivery.Email, delivery.Subject, delivery.BodyHTML)
	cancel()
	resultCtx, resultCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resultCancel()
	if err == nil {
		if markErr := w.repo.MarkSent(resultCtx, delivery.ID, time.Now()); markErr != nil {
			logger.L().Warn("announcement_email.mark_sent_failed", zap.Int64("delivery_id", delivery.ID), zap.Error(markErr))
		}
		return
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	if markErr := w.repo.MarkFailed(resultCtx, delivery.ID, message, time.Now()); markErr != nil {
		logger.L().Warn("announcement_email.mark_failed_failed", zap.Int64("delivery_id", delivery.ID), zap.Error(markErr))
	}
}
