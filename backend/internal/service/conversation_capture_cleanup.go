package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type ConversationCaptureCleanupService struct {
	capture *ConversationCaptureService
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewConversationCaptureCleanupService(capture *ConversationCaptureService) *ConversationCaptureCleanupService {
	return &ConversationCaptureCleanupService{
		capture: capture,
		stopCh:  make(chan struct{}),
	}
}

func (s *ConversationCaptureCleanupService) Start() {
	if s == nil || s.capture == nil {
		return
	}
	s.wg.Add(1)
	go s.loop()
}

func (s *ConversationCaptureCleanupService) Stop() {
	if s == nil {
		return
	}
	close(s.stopCh)
	s.wg.Wait()
}

func (s *ConversationCaptureCleanupService) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *ConversationCaptureCleanupService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessions, turns, err := s.capture.CleanupExpired(ctx, 1000)
	if err != nil {
		logger.L().With(zap.String("component", "service.conversation_capture_cleanup")).
			Warn("conversation_capture.cleanup_failed", zap.Error(err))
		return
	}
	expiredExports, exportErr := s.capture.CleanupExpiredExportJobs(ctx, 100)
	if exportErr != nil {
		logger.L().With(zap.String("component", "service.conversation_capture_cleanup")).
			Warn("conversation_export.cleanup_failed", zap.Error(exportErr))
	}
	if sessions > 0 || turns > 0 {
		logger.L().With(zap.String("component", "service.conversation_capture_cleanup")).
			Info("conversation_capture.cleanup_completed", zap.Int64("sessions", sessions), zap.Int64("turns", turns))
	}
	if expiredExports > 0 {
		logger.L().With(zap.String("component", "service.conversation_capture_cleanup")).
			Info("conversation_export.cleanup_completed", zap.Int("objects_deleted", expiredExports))
	}
}
