package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

const (
	conversationExportPrefix      = "conversation-exports"
	conversationExportDownloadTTL = 15 * time.Minute
)

func (s *ConversationCaptureService) CreateExportJob(ctx context.Context, req ConversationCreateExportJobRequest, createdBy int64) (*ConversationExportJob, error) {
	if s == nil || s.repo == nil {
		return nil, errors.InternalServer("CONVERSATION_EXPORT_UNAVAILABLE", "conversation export service unavailable")
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.ExportEnabled {
		return nil, errors.Forbidden("CONVERSATION_EXPORT_DISABLED", "conversation export is disabled")
	}
	req, err = normalizeConversationExportJobRequest(req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job, err := s.repo.CreateExportJob(ctx, ConversationExportJob{
		Status:    ConversationExportJobStatusPending,
		Filters:   req.Filters,
		Format:    req.Format,
		Encoding:  req.Encoding,
		ExpiresAt: now.AddDate(0, 0, cfg.RetentionDays),
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	if s.exportWorker == nil || !s.exportWorker.TrySubmit(func(taskCtx context.Context) error {
		s.runExportJob(taskCtx, job.ID)
		return nil
	}) {
		_ = s.repo.FailExportJob(context.Background(), job.ID, "export worker queue is full", time.Now())
		job.Status = ConversationExportJobStatusFailed
		msg := "export worker queue is full"
		job.ErrorMessage = &msg
	}
	return job, nil
}

func (s *ConversationCaptureService) ListExportJobs(ctx context.Context, page, pageSize int) ([]ConversationExportJob, int64, error) {
	return s.repo.ListExportJobs(ctx, page, clampPageSize(pageSize))
}

func (s *ConversationCaptureService) GetExportJob(ctx context.Context, id int64) (*ConversationExportJob, error) {
	return s.repo.GetExportJobByID(ctx, id)
}

func (s *ConversationCaptureService) CreateExportDownloadTicket(ctx context.Context, id int64) (*ConversationExportDownloadTicket, error) {
	job, err := s.repo.GetExportJobByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status != ConversationExportJobStatusCompleted || job.S3Key == nil || strings.TrimSpace(*job.S3Key) == "" {
		return nil, errors.BadRequest("CONVERSATION_EXPORT_NOT_READY", "conversation export job is not completed")
	}
	now := time.Now()
	if !job.ExpiresAt.After(now) {
		return nil, errors.BadRequest("CONVERSATION_EXPORT_EXPIRED", "conversation export job has expired")
	}
	store, err := s.conversationExportObjectStore(ctx)
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(conversationExportDownloadTTL)
	if job.ExpiresAt.Before(expiresAt) {
		expiresAt = job.ExpiresAt
	}
	url, err := store.PresignURL(ctx, *job.S3Key, time.Until(expiresAt))
	if err != nil {
		return nil, fmt.Errorf("presign conversation export: %w", err)
	}
	if err := s.repo.SetExportJobDownloadURLExpiresAt(ctx, id, expiresAt); err != nil {
		return nil, err
	}
	return &ConversationExportDownloadTicket{DownloadURL: url, ExpiresAt: expiresAt}, nil
}

func (s *ConversationCaptureService) DeleteExportJob(ctx context.Context, id int64) error {
	job, err := s.repo.GetExportJobByID(ctx, id)
	if err != nil {
		return err
	}
	if job.Status == ConversationExportJobStatusRunning {
		return errors.Conflict("CONVERSATION_EXPORT_RUNNING", "conversation export job is running")
	}
	job, err = s.repo.MarkExportJobDeleted(ctx, id, time.Now())
	if err != nil {
		return err
	}
	if job.S3Key == nil || strings.TrimSpace(*job.S3Key) == "" {
		return nil
	}
	store, err := s.conversationExportObjectStore(ctx)
	if err != nil {
		return err
	}
	return store.Delete(ctx, *job.S3Key)
}

func (s *ConversationCaptureService) CleanupExpiredExportJobs(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	now := time.Now()
	jobs, err := s.repo.MarkExpiredExportJobs(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	if len(jobs) == 0 {
		return 0, nil
	}
	store, storeErr := s.conversationExportObjectStore(ctx)
	deleted := 0
	expiredIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if storeErr != nil || job.S3Key == nil || strings.TrimSpace(*job.S3Key) == "" {
			expiredIDs = append(expiredIDs, job.ID)
			continue
		}
		if err := store.Delete(ctx, *job.S3Key); err != nil {
			logger.L().With(zap.String("component", "service.conversation_export")).
				Warn("conversation_export.delete_expired_object_failed", zap.Int64("job_id", job.ID), zap.Error(err))
			continue
		}
		expiredIDs = append(expiredIDs, job.ID)
		deleted++
	}
	if len(expiredIDs) > 0 {
		if err := s.repo.MarkExportJobsExpired(ctx, expiredIDs, now); err != nil {
			return deleted, err
		}
	}
	return deleted, nil
}

func (s *ConversationCaptureService) runExportJob(ctx context.Context, id int64) {
	now := time.Now()
	if err := s.repo.MarkExportJobRunning(ctx, id, now); err != nil {
		logger.L().With(zap.String("component", "service.conversation_export")).
			Warn("conversation_export.mark_running_failed", zap.Int64("job_id", id), zap.Error(err))
		return
	}
	if err := s.executeExportJob(ctx, id); err != nil {
		_ = s.repo.FailExportJob(context.Background(), id, safeConversationExportError(err), time.Now())
	}
}

func (s *ConversationCaptureService) executeExportJob(ctx context.Context, id int64) error {
	job, err := s.repo.GetExportJobByID(ctx, id)
	if err != nil {
		return err
	}
	exportReq := conversationExportRequestFromJobFilters(job.Filters)
	turns, err := s.repo.ListExportableTurns(ctx, exportReq)
	if err != nil {
		return err
	}
	payload, sessionCount, turnCount, err := buildConversationExportPayloadWithOptions(turns, job.Encoding, job.Filters)
	if err != nil {
		return err
	}
	store, err := s.conversationExportObjectStore(ctx)
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("conversation_export_%d_%s.jsonl", job.ID, time.Now().UTC().Format("20060102_150405"))
	contentType := "application/x-ndjson"
	if job.Encoding == ConversationExportEncodingZstd {
		fileName += ".zst"
		contentType = "application/zstd"
	}
	key := path.Join(conversationExportPrefix, time.Now().UTC().Format("2006/01/02"), fileName)
	size, err := store.Upload(ctx, key, bytes.NewReader(payload), contentType)
	if err != nil {
		return err
	}
	return s.repo.CompleteExportJob(ctx, job.ID, sessionCount, turnCount, size, key, time.Now())
}

func (s *ConversationCaptureService) conversationExportObjectStore(ctx context.Context) (BackupObjectStore, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.BadRequest("CONVERSATION_EXPORT_S3_NOT_CONFIGURED", "conversation export object storage is not configured")
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyBackupS3Config)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, errors.BadRequest("CONVERSATION_EXPORT_S3_NOT_CONFIGURED", "conversation export object storage is not configured")
	}
	var cfg BackupS3Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, errors.InternalServer("CONVERSATION_EXPORT_S3_CONFIG_CORRUPT", "conversation export object storage config is corrupted")
	}
	if cfg.SecretAccessKey != "" && s.encryptor != nil {
		if decrypted, err := s.encryptor.Decrypt(cfg.SecretAccessKey); err == nil {
			cfg.SecretAccessKey = decrypted
		}
	}
	if !cfg.IsConfigured() || s.storeFactory == nil {
		return nil, errors.BadRequest("CONVERSATION_EXPORT_S3_NOT_CONFIGURED", "conversation export object storage is not configured")
	}
	return s.storeFactory(ctx, &cfg)
}

func normalizeConversationExportJobRequest(req ConversationCreateExportJobRequest) (ConversationCreateExportJobRequest, error) {
	if strings.TrimSpace(req.Format) == "" {
		req.Format = ConversationExportFormatMessagesJSONL
	}
	if strings.TrimSpace(req.Encoding) == "" {
		req.Encoding = ConversationExportEncodingZstd
	}
	req.Format = strings.TrimSpace(req.Format)
	req.Encoding = strings.TrimSpace(req.Encoding)
	if req.Format != ConversationExportFormatMessagesJSONL {
		return req, errors.BadRequest("INVALID_CONVERSATION_EXPORT_FORMAT", "conversation export format is not supported")
	}
	if req.Encoding != ConversationExportEncodingPlain && req.Encoding != ConversationExportEncodingZstd {
		return req, errors.BadRequest("INVALID_CONVERSATION_EXPORT_ENCODING", "conversation export encoding is not supported")
	}
	if req.Filters.Limit <= 0 {
		req.Filters.Limit = 1000
	}
	if req.Filters.Limit > 10000 {
		req.Filters.Limit = 10000
	}
	if req.Filters.Dedupe {
		req.Filters.IncludeDuplicates = false
	}
	req.Filters.Dedupe = !req.Filters.IncludeDuplicates
	return req, nil
}

func conversationExportRequestFromJobFilters(filters ConversationExportJobFilters) ConversationExportMessagesJSONLRequest {
	dedupe := filters.Dedupe
	if filters.Limit <= 0 {
		filters.Limit = 1000
	}
	return ConversationExportMessagesJSONLRequest{
		ConversationSessionFilters: ConversationSessionFilters{
			UserID:        filters.UserID,
			APIKeyID:      filters.APIKeyID,
			Model:         filters.Model,
			RequestID:     filters.RequestID,
			QualityStatus: filters.QualityStatus,
			StartedAtFrom: filters.StartedAtFrom,
			StartedAtTo:   filters.StartedAtTo,
		},
		IncludeHeuristic:  filters.IncludeHeuristic,
		IncludeDuplicates: filters.IncludeDuplicates,
		RedactionEnabled:  filters.RedactionEnabled,
		Dedupe:            &dedupe,
		Limit:             filters.Limit,
	}
}

func buildConversationExportPayload(turns []ConversationTurn, encoding string, dedupe bool) ([]byte, int64, int64, error) {
	return buildConversationExportPayloadWithOptions(turns, encoding, ConversationExportJobFilters{
		Dedupe:            dedupe,
		IncludeDuplicates: !dedupe,
	})
}

func buildConversationExportPayloadWithOptions(turns []ConversationTurn, encoding string, filters ConversationExportJobFilters) ([]byte, int64, int64, error) {
	exportReq := conversationExportRequestFromJobFilters(filters)
	turns = FilterConversationExportableTurns(turns, exportReq)
	var raw bytes.Buffer
	seenSessions := map[string]struct{}{}
	seenDedupe := map[string]struct{}{}
	turnCount := int64(0)
	for _, turn := range turns {
		if !filters.IncludeDuplicates && turn.DedupeHash != "" {
			if _, ok := seenDedupe[turn.DedupeHash]; ok {
				continue
			}
			seenDedupe[turn.DedupeHash] = struct{}{}
		}
		seenSessions[turn.SessionID] = struct{}{}
		turnCount++
		messages := buildConversationExportMessages(turn, filters.RedactionEnabled)
		row := map[string]any{
			"messages": messages,
			"metadata": map[string]any{
				"session_id": turn.SessionID,
				"request_id": turn.RequestID,
				"user_id":    turn.Meta["user_id"],
				"api_key_id": turn.Meta["api_key_id"],
				"model":      turn.Model,
			},
		}
		line, err := json.Marshal(row)
		if err != nil {
			return nil, 0, 0, err
		}
		raw.Write(line)
		raw.WriteByte('\n')
	}
	if encoding != ConversationExportEncodingZstd {
		return raw.Bytes(), int64(len(seenSessions)), turnCount, nil
	}
	var compressed bytes.Buffer
	zw, err := zstd.NewWriter(&compressed)
	if err != nil {
		return nil, 0, 0, err
	}
	if _, err := io.Copy(zw, bytes.NewReader(raw.Bytes())); err != nil {
		_ = zw.Close()
		return nil, 0, 0, err
	}
	if err := zw.Close(); err != nil {
		return nil, 0, 0, err
	}
	return compressed.Bytes(), int64(len(seenSessions)), turnCount, nil
}

func safeConversationExportError(err error) string {
	if err == nil {
		return ""
	}
	return truncateConversationError(err.Error(), 500)
}
