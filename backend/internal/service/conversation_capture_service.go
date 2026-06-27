package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
)

type ConversationCaptureService struct {
	repo         ConversationRepository
	settingRepo  SettingRepository
	worker       *ConversationCaptureWorkerPool
	exportWorker *ConversationExportWorkerPool
	cfg          *config.Config
	encryptor    SecretEncryptor
	storeFactory BackupObjectStoreFactory
}

func NewConversationCaptureService(repo ConversationRepository, settingRepo SettingRepository, worker *ConversationCaptureWorkerPool, cfg *config.Config) *ConversationCaptureService {
	return &ConversationCaptureService{repo: repo, settingRepo: settingRepo, worker: worker, cfg: cfg}
}

func ProvideConversationCaptureService(
	repo ConversationRepository,
	settingRepo SettingRepository,
	worker *ConversationCaptureWorkerPool,
	exportWorker *ConversationExportWorkerPool,
	cfg *config.Config,
	encryptor SecretEncryptor,
	storeFactory BackupObjectStoreFactory,
) *ConversationCaptureService {
	svc := NewConversationCaptureService(repo, settingRepo, worker, cfg)
	svc.exportWorker = exportWorker
	svc.encryptor = encryptor
	svc.storeFactory = storeFactory
	return svc
}

func (s *ConversationCaptureService) Decide(ctx context.Context, subject ConversationCaptureSubject) ConversationCaptureDecision {
	return s.DecideForEndpoint(ctx, subject, ConversationCaptureEndpointChatCompletions)
}

func (s *ConversationCaptureService) DecideForEndpoint(ctx context.Context, subject ConversationCaptureSubject, endpointKind string) ConversationCaptureDecision {
	cfg := s.defaultConfig()
	if loaded, err := s.GetConfig(ctx); err == nil {
		cfg = *loaded
	}
	normalizeConversationCaptureConfig(&cfg, s.defaultConfig())
	if !cfg.Enabled {
		return ConversationCaptureDecision{}
	}
	endpointKind = strings.TrimSpace(endpointKind)
	if endpointKind == "" {
		endpointKind = ConversationCaptureEndpointChatCompletions
	}
	if endpointKind == ConversationCaptureEndpointChatCompletions && !cfg.CaptureChatCompletions {
		return ConversationCaptureDecision{}
	}
	if endpointKind == ConversationCaptureEndpointResponses && !cfg.CaptureResponses {
		return ConversationCaptureDecision{}
	}
	if conversationCaptureContainsInt64(cfg.ExcludedUserIDs, subject.UserID) || conversationCaptureContainsInt64(cfg.ExcludedAPIKeyIDs, subject.APIKeyID) {
		return ConversationCaptureDecision{}
	}
	if cfg.SamplePercent <= 0 || (cfg.SamplePercent < 100 && rand.Intn(100) >= cfg.SamplePercent) {
		return ConversationCaptureDecision{}
	}
	return ConversationCaptureDecision{
		Capture:              true,
		EndpointKind:         endpointKind,
		MaxTurnPayloadBytes:  cfg.MaxTurnPayloadBytes,
		PayloadPreviewChars:  cfg.PayloadPreviewChars,
		SessionWindowMinutes: cfg.SessionWindowMinutes,
		RetentionDays:        cfg.RetentionDays,
	}
}

func (s *ConversationCaptureService) Submit(decision ConversationCaptureDecision, input ConversationCaptureInput) bool {
	if s == nil || s.worker == nil || s.repo == nil || !decision.Capture {
		return false
	}
	taskInput := cloneConversationCaptureInput(input)
	taskDecision := decision
	return s.worker.TrySubmit(func(ctx context.Context) error {
		return s.capture(ctx, taskDecision, taskInput)
	})
}

func (s *ConversationCaptureService) GetConfig(ctx context.Context) (*ConversationCaptureConfig, error) {
	cfg := s.defaultConfig()
	if s.settingRepo == nil {
		normalizeConversationCaptureConfig(&cfg, cfg)
		return &cfg, nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyConversationCaptureConfig)
	if err != nil {
		if err == ErrSettingNotFound {
			normalizeConversationCaptureConfig(&cfg, cfg)
			return &cfg, nil
		}
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		normalizeConversationCaptureConfig(&cfg, cfg)
		return &cfg, nil
	}
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		cfg = s.defaultConfig()
	}
	normalizeConversationCaptureConfig(&cfg, s.defaultConfig())
	return &cfg, nil
}

func (s *ConversationCaptureService) UpdateConfig(ctx context.Context, cfg ConversationCaptureConfig) (*ConversationCaptureConfig, error) {
	defaultCfg := s.defaultConfig()
	if err := validateConversationCaptureConfig(cfg); err != nil {
		return nil, err
	}
	normalizeConversationCaptureConfig(&cfg, defaultCfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if s.settingRepo == nil {
		return nil, infraerrors.InternalServer("CONVERSATION_CAPTURE_SETTINGS_UNAVAILABLE", "settings repository unavailable")
	}
	if err := s.settingRepo.Set(ctx, SettingKeyConversationCaptureConfig, string(raw)); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *ConversationCaptureService) ListSessions(ctx context.Context, filters ConversationSessionFilters, page, pageSize int) ([]ConversationSession, int64, error) {
	return s.repo.ListSessions(ctx, filters, page, clampPageSize(pageSize))
}

func (s *ConversationCaptureService) GetSession(ctx context.Context, id int64) (*ConversationSession, error) {
	return s.repo.GetSessionByID(ctx, id)
}

func (s *ConversationCaptureService) ListSessionTurns(ctx context.Context, sessionID string, page, pageSize int) ([]ConversationTurnSummary, int64, error) {
	return s.repo.ListTurnsBySessionID(ctx, sessionID, page, clampPageSize(pageSize))
}

func (s *ConversationCaptureService) GetTurn(ctx context.Context, id int64) (*ConversationTurn, error) {
	return s.repo.GetTurnByID(ctx, id)
}

func (s *ConversationCaptureService) SetSessionExportable(ctx context.Context, id int64, exportable bool) error {
	return s.repo.SetSessionExportable(ctx, id, exportable)
}

func (s *ConversationCaptureService) SetTurnExportable(ctx context.Context, id int64, exportable bool) error {
	return s.repo.SetTurnExportable(ctx, id, exportable)
}

func (s *ConversationCaptureService) SetSessionQuality(ctx context.Context, id int64, req ConversationQualityUpdateRequest) error {
	req = normalizeConversationQualityUpdate(req)
	if err := validateConversationQualityStatus(req.QualityStatus); err != nil {
		return err
	}
	return s.repo.SetSessionQuality(ctx, id, req.QualityStatus, req.QualityErrors, req.Exportable)
}

func (s *ConversationCaptureService) SetTurnQuality(ctx context.Context, id int64, req ConversationQualityUpdateRequest) error {
	req = normalizeConversationQualityUpdate(req)
	if err := validateConversationQualityStatus(req.QualityStatus); err != nil {
		return err
	}
	return s.repo.SetTurnQuality(ctx, id, req.QualityStatus, req.QualityErrors, req.Exportable)
}

func (s *ConversationCaptureService) BulkSetQuality(ctx context.Context, req ConversationBulkQualityUpdateRequest) error {
	req.QualityStatus = strings.TrimSpace(req.QualityStatus)
	normalized := normalizeConversationQualityUpdate(ConversationQualityUpdateRequest{
		QualityStatus: req.QualityStatus,
		QualityErrors: req.QualityErrors,
		Exportable:    req.Exportable,
	})
	req.QualityStatus = normalized.QualityStatus
	req.QualityErrors = normalized.QualityErrors
	req.Exportable = normalized.Exportable
	if err := validateConversationQualityStatus(req.QualityStatus); err != nil {
		return err
	}
	if len(req.SessionIDs) == 0 && len(req.TurnIDs) == 0 {
		return infraerrors.BadRequest("EMPTY_CONVERSATION_QUALITY_TARGETS", "session_ids or turn_ids is required")
	}
	if len(req.SessionIDs)+len(req.TurnIDs) > 200 {
		return infraerrors.BadRequest("TOO_MANY_CONVERSATION_QUALITY_TARGETS", "too many quality targets")
	}
	req.SessionIDs = positiveUniqueInt64s(req.SessionIDs)
	req.TurnIDs = positiveUniqueInt64s(req.TurnIDs)
	return s.repo.BulkSetQuality(ctx, req)
}

func (s *ConversationCaptureService) MergeSessions(ctx context.Context, req ConversationMergeSessionsRequest) error {
	if req.TargetSessionID <= 0 {
		return infraerrors.BadRequest("INVALID_TARGET_SESSION_ID", "target_session_id must be positive")
	}
	req.SourceSessionIDs = positiveUniqueInt64s(req.SourceSessionIDs)
	if len(req.SourceSessionIDs) == 0 {
		return infraerrors.BadRequest("EMPTY_SOURCE_SESSION_IDS", "source_session_ids is required")
	}
	return s.repo.MergeSessions(ctx, req.TargetSessionID, req.SourceSessionIDs)
}

func (s *ConversationCaptureService) SplitSession(ctx context.Context, sessionID int64, req ConversationSplitSessionRequest) (*ConversationSession, error) {
	if sessionID <= 0 || req.TurnID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CONVERSATION_SPLIT_TARGET", "session id and turn_id must be positive")
	}
	now := time.Now()
	newSessionID := "manual_split_" + hashHex([]byte(fmt.Sprintf("%d:%d:%s", sessionID, req.TurnID, now.Format(time.RFC3339Nano))))[:24]
	return s.repo.SplitSessionFromTurn(ctx, sessionID, req.TurnID, newSessionID, now)
}

func (s *ConversationCaptureService) MoveTurn(ctx context.Context, turnID int64, req ConversationMoveTurnRequest) error {
	if turnID <= 0 || req.TargetSessionID <= 0 {
		return infraerrors.BadRequest("INVALID_CONVERSATION_MOVE_TARGET", "turn id and target_session_id must be positive")
	}
	return s.repo.MoveTurn(ctx, turnID, req.TargetSessionID)
}

func (s *ConversationCaptureService) ExportMessagesJSONL(ctx context.Context, req ConversationExportMessagesJSONLRequest) ([]byte, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.ExportEnabled {
		return nil, infraerrors.Forbidden("CONVERSATION_EXPORT_DISABLED", "conversation export is disabled")
	}
	if req.Limit <= 0 {
		req.Limit = 200
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}
	req = normalizeConversationExportMessagesRequest(req)
	turns, err := s.repo.ListExportableTurns(ctx, req)
	if err != nil {
		return nil, err
	}
	turns = FilterConversationExportableTurns(turns, req)
	var buf bytes.Buffer
	seen := map[string]struct{}{}
	for _, turn := range turns {
		if !req.IncludeDuplicates {
			if _, ok := seen[turn.DedupeHash]; ok {
				continue
			}
			seen[turn.DedupeHash] = struct{}{}
		}
		messages := buildConversationExportMessages(turn, req.RedactionEnabled)
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
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		buf.Write(raw)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func (s *ConversationCaptureService) CleanupExpired(ctx context.Context, limit int) (int64, int64, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	return s.repo.CleanupExpired(ctx, time.Now(), limit)
}

func (s *ConversationCaptureService) capture(ctx context.Context, decision ConversationCaptureDecision, input ConversationCaptureInput) error {
	now := time.Now()
	meta := input.Meta
	requestBody, responseRaw, truncated := enforceTurnPayloadLimit(meta.RequestBody, input.ResponseRaw, decision.MaxTurnPayloadBytes)
	preview := buildPayloadPreview(requestBody, responseRaw, decision.PayloadPreviewChars)
	endpointKind := conversationCaptureFirstNonEmpty(meta.EndpointKind, decision.EndpointKind, ConversationCaptureEndpointChatCompletions)
	parsed := parseOpenAIChatCompletionsTurn(requestBody, responseRaw, meta.Stream)
	if endpointKind == ConversationCaptureEndpointResponses {
		parsed = parseOpenAIResponsesTurn(requestBody, responseRaw, meta.Stream)
	}
	if truncated || input.Truncated || input.ClientDisconnect || meta.ClientDisconnect {
		parsed.ParseStatus = parsed.ParseStatus
	}
	sessionID, sessionSource := resolveConversationSessionID(meta, decision.SessionWindowMinutes)
	if endpointKind == ConversationCaptureEndpointResponses && sessionID == "" {
		sessionID, sessionSource = s.resolveResponsesSessionID(ctx, meta, parsed, now)
	}
	if sessionID == "" {
		sessionID = "cc_" + hashHex([]byte(fmt.Sprintf("%d:%d:%s:%s", meta.UserID, meta.APIKeyID, meta.RequestID, now.Format(time.RFC3339Nano))))[:24]
		sessionSource = ConversationSessionSourceSingle
	}
	record := ConversationTurnRecord{
		SessionID:         sessionID,
		SessionSource:     sessionSource,
		UserID:            meta.UserID,
		APIKeyID:          meta.APIKeyID,
		AccountID:         meta.AccountID,
		Provider:          ConversationProviderOpenAI,
		Model:             conversationCaptureFirstNonEmpty(meta.Model, meta.UpstreamModel),
		RequestPath:       meta.RequestPath,
		RequestID:         conversationCaptureFirstNonEmpty(meta.RequestID, meta.ClientRequestID),
		UpstreamRequestID: meta.UpstreamRequestID,
		ClientRequestID:   meta.ClientRequestID,
		TurnIndex:         0,
		RequestMessages:   parsed.RequestMessages,
		ResponseMessages:  parsed.ResponseMessages,
		Tools:             parsed.Tools,
		Usage: map[string]any{
			"input_tokens":                meta.Usage.InputTokens,
			"output_tokens":               meta.Usage.OutputTokens,
			"cache_creation_input_tokens": meta.Usage.CacheCreationInputTokens,
			"cache_read_input_tokens":     meta.Usage.CacheReadInputTokens,
		},
		Meta: map[string]any{
			"user_id":        meta.UserID,
			"api_key_id":     meta.APIKeyID,
			"account_id":     meta.AccountID,
			"session_source": sessionSource,
			"endpoint_kind":  endpointKind,
		},
		InputTokens:      int64(meta.Usage.InputTokens),
		OutputTokens:     int64(meta.Usage.OutputTokens),
		TotalTokens:      int64(meta.Usage.InputTokens + meta.Usage.OutputTokens),
		ActualCost:       meta.ActualCost,
		Stream:           meta.Stream,
		ClientDisconnect: input.ClientDisconnect || meta.ClientDisconnect,
		Truncated:        truncated || input.Truncated,
		QualityStatus:    ConversationQualityStatusUnchecked,
		Exportable:       false,
		ParseStatus:      parsed.ParseStatus,
		ParseError:       parsed.ParseError,
		DedupeHash:       buildConversationDedupeHash(parsed.RequestMessages, parsed.ResponseMessages, parsed.Tools, meta, requestBody, responseRaw),
		PayloadPreview:   preview,
		RetentionUntil:   now.AddDate(0, 0, decision.RetentionDays),
		CreatedAt:        now,
	}
	if record.ParseStatus == "" {
		record.ParseStatus = ConversationParseStatusFailed
	}
	record.ParseError = truncateConversationError(record.ParseError, 500)
	responseID := conversationCaptureFirstNonEmpty(parsed.ResponseID, meta.ResponseID)
	if responseID != "" {
		record.Meta["response_id"] = responseID
	}
	previousResponseID := conversationCaptureFirstNonEmpty(parsed.PreviousResponseID, meta.PreviousResponseID)
	if previousResponseID != "" {
		record.Meta["previous_response_id"] = previousResponseID
	}
	if record.Model == "" {
		record.Model = "unknown"
	}
	if record.RequestPath == "" {
		record.RequestPath = "/v1/chat/completions"
	}
	assessment := AssessConversationTurnQuality(record)
	record.QualityStatus = assessment.QualityStatus
	record.QualityErrors = assessment.QualityErrors
	record.Exportable = assessment.Exportable
	return s.repo.UpsertTurn(ctx, record)
}

func (s *ConversationCaptureService) resolveResponsesSessionID(ctx context.Context, meta ConversationCaptureMeta, parsed conversationParseResult, now time.Time) (string, string) {
	previousResponseID := conversationCaptureFirstNonEmpty(parsed.PreviousResponseID, meta.PreviousResponseID)
	if previousResponseID != "" && s != nil && s.repo != nil {
		if sessionID, err := s.repo.FindSessionIDByResponseID(ctx, previousResponseID); err == nil && sessionID != "" {
			return sessionID, ConversationSessionSourceResponses
		}
	}
	responseID := conversationCaptureFirstNonEmpty(parsed.ResponseID, meta.ResponseID, meta.RequestID, meta.ClientRequestID)
	if responseID == "" {
		responseID = fmt.Sprintf("%d:%d:%s", meta.UserID, meta.APIKeyID, now.Format(time.RFC3339Nano))
	}
	return "resp_" + safeSessionHash(responseID), ConversationSessionSourceResponses
}

func (s *ConversationCaptureService) defaultConfig() ConversationCaptureConfig {
	cfg := ConversationCaptureConfig{
		Enabled:                false,
		SamplePercent:          100,
		CaptureChatCompletions: true,
		CaptureResponses:       false,
		RawArchiveEnabled:      false,
		MaxTurnPayloadBytes:    1048576,
		PayloadPreviewChars:    8000,
		SessionWindowMinutes:   30,
		RetentionDays:          30,
		ExportEnabled:          true,
	}
	if s != nil && s.cfg != nil {
		cc := s.cfg.Gateway.ConversationCapture
		cfg.Enabled = cc.Enabled
		cfg.SamplePercent = cc.SamplePercent
		cfg.CaptureChatCompletions = cc.CaptureChatCompletions
		cfg.CaptureResponses = cc.CaptureResponses
		cfg.RawArchiveEnabled = cc.RawArchiveEnabled
		cfg.MaxTurnPayloadBytes = cc.MaxTurnPayloadBytes
		cfg.PayloadPreviewChars = cc.PayloadPreviewChars
		cfg.SessionWindowMinutes = cc.SessionWindowMinutes
		cfg.RetentionDays = cc.RetentionDays
		cfg.ExportEnabled = cc.ExportEnabled
	}
	normalizeConversationCaptureConfig(&cfg, cfg)
	return cfg
}

func validateConversationCaptureConfig(cfg ConversationCaptureConfig) error {
	if cfg.SamplePercent < 0 || cfg.SamplePercent > 100 {
		return infraerrors.BadRequest("INVALID_SAMPLE_PERCENT", "sample_percent must be between 0 and 100")
	}
	if cfg.MaxTurnPayloadBytes <= 0 {
		return infraerrors.BadRequest("INVALID_MAX_TURN_PAYLOAD_BYTES", "max_turn_payload_bytes must be positive")
	}
	if cfg.PayloadPreviewChars <= 0 {
		return infraerrors.BadRequest("INVALID_PAYLOAD_PREVIEW_CHARS", "payload_preview_chars must be positive")
	}
	if cfg.SessionWindowMinutes <= 0 {
		return infraerrors.BadRequest("INVALID_SESSION_WINDOW_MINUTES", "session_window_minutes must be positive")
	}
	if cfg.RetentionDays < 1 {
		return infraerrors.BadRequest("INVALID_RETENTION_DAYS", "retention_days must be at least 1")
	}
	for _, id := range cfg.ExcludedUserIDs {
		if id <= 0 {
			return infraerrors.BadRequest("INVALID_EXCLUDED_USER_ID", "excluded_user_ids must contain positive ids")
		}
	}
	for _, id := range cfg.ExcludedAPIKeyIDs {
		if id <= 0 {
			return infraerrors.BadRequest("INVALID_EXCLUDED_API_KEY_ID", "excluded_api_key_ids must contain positive ids")
		}
	}
	return nil
}

func normalizeConversationCaptureConfig(cfg *ConversationCaptureConfig, defaults ConversationCaptureConfig) {
	if cfg.SamplePercent < 0 || cfg.SamplePercent > 100 {
		cfg.SamplePercent = defaults.SamplePercent
	}
	if cfg.MaxTurnPayloadBytes <= 0 {
		cfg.MaxTurnPayloadBytes = defaults.MaxTurnPayloadBytes
	}
	if cfg.PayloadPreviewChars <= 0 {
		cfg.PayloadPreviewChars = defaults.PayloadPreviewChars
	}
	if cfg.SessionWindowMinutes <= 0 {
		cfg.SessionWindowMinutes = defaults.SessionWindowMinutes
	}
	if cfg.RetentionDays < 1 {
		cfg.RetentionDays = defaults.RetentionDays
	}
	cfg.ExcludedUserIDs = positiveUniqueInt64s(cfg.ExcludedUserIDs)
	cfg.ExcludedAPIKeyIDs = positiveUniqueInt64s(cfg.ExcludedAPIKeyIDs)
}

func cloneConversationCaptureInput(input ConversationCaptureInput) ConversationCaptureInput {
	out := input
	out.Meta.RequestBody = append([]byte(nil), input.Meta.RequestBody...)
	out.ResponseRaw = append([]byte(nil), input.ResponseRaw...)
	return out
}

func enforceTurnPayloadLimit(requestBody, responseRaw []byte, maxBytes int) ([]byte, []byte, bool) {
	req := append([]byte(nil), requestBody...)
	resp := append([]byte(nil), responseRaw...)
	if maxBytes <= 0 || len(req)+len(resp) <= maxBytes {
		return req, resp, false
	}
	if len(req) >= maxBytes/2 {
		req = req[:maxBytes/2]
	}
	remain := maxBytes - len(req)
	if remain < 0 {
		remain = 0
	}
	if len(resp) > remain {
		resp = resp[:remain]
	}
	return req, resp, true
}

func buildPayloadPreview(requestBody, responseRaw []byte, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	preview := "request:\n" + string(requestBody) + "\nresponse:\n" + string(responseRaw)
	rs := []rune(preview)
	if len(rs) > maxChars {
		rs = rs[:maxChars]
	}
	return string(rs)
}

func resolveConversationSessionID(meta ConversationCaptureMeta, windowMinutes int) (string, string) {
	if explicit := strings.TrimSpace(meta.ConversationID); explicit != "" {
		return "explicit_" + safeSessionHash(explicit), ConversationSessionSourceExplicit
	}
	if explicit := strings.TrimSpace(gjson.GetBytes(meta.RequestBody, "metadata.conversation_id").String()); explicit != "" {
		return "explicit_" + safeSessionHash(explicit), ConversationSessionSourceExplicit
	}
	if explicit := strings.TrimSpace(gjson.GetBytes(meta.RequestBody, "conversation_id").String()); explicit != "" {
		return "explicit_" + safeSessionHash(explicit), ConversationSessionSourceExplicit
	}
	if windowMinutes <= 0 {
		windowMinutes = 30
	}
	prefix := gjson.GetBytes(meta.RequestBody, "messages.0").Raw
	if prefix == "" {
		return "", ConversationSessionSourceSingle
	}
	bucket := time.Now().Unix() / int64(windowMinutes*60)
	seed := fmt.Sprintf("%d:%d:%s:%s:%d", meta.UserID, meta.APIKeyID, meta.Model, prefix, bucket)
	return "heur_" + hashHex([]byte(seed))[:24], ConversationSessionSourceHeuristic
}

func buildConversationDedupeHash(requestMessages, responseMessages, tools []json.RawMessage, meta ConversationCaptureMeta, requestBody, responseRaw []byte) string {
	if len(requestMessages)+len(responseMessages)+len(tools) > 0 {
		payload := map[string]any{
			"model":             conversationCaptureFirstNonEmpty(meta.Model, meta.UpstreamModel),
			"request_messages":  canonicalConversationRawMessages(requestMessages),
			"response_messages": canonicalConversationRawMessages(responseMessages),
			"tools":             canonicalConversationRawMessages(tools),
		}
		if raw, err := json.Marshal(payload); err == nil {
			return hashHex(raw)
		}
	}
	h := sha256.New()
	h.Write([]byte(meta.RequestID))
	h.Write([]byte(meta.ClientRequestID))
	h.Write(requestBody)
	h.Write(responseRaw)
	return hex.EncodeToString(h.Sum(nil))
}

func canonicalConversationRawMessages(values []json.RawMessage) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			out = append(out, strings.TrimSpace(string(value)))
			continue
		}
		out = append(out, decoded)
	}
	return out
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func safeSessionHash(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return hashHex([]byte(v))[:32]
}

func conversationCaptureFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func conversationCaptureContainsInt64(values []int64, target int64) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func positiveUniqueInt64s(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func truncateConversationError(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len([]rune(value)) <= maxLen {
		return value
	}
	rs := []rune(value)
	return string(rs[:maxLen])
}

func validateConversationQualityStatus(status string) error {
	switch strings.TrimSpace(status) {
	case ConversationQualityStatusUnchecked, ConversationQualityStatusClean, ConversationQualityStatusNeedsReview, ConversationQualityStatusRejected:
		return nil
	default:
		return infraerrors.BadRequest("INVALID_CONVERSATION_QUALITY_STATUS", "quality_status must be one of unchecked/clean/needs_review/rejected")
	}
}

func normalizeConversationQualityUpdate(req ConversationQualityUpdateRequest) ConversationQualityUpdateRequest {
	req.QualityStatus = strings.TrimSpace(req.QualityStatus)
	req.QualityErrors = normalizeConversationQualityErrors(req.QualityErrors)
	if req.QualityStatus == ConversationQualityStatusRejected || req.QualityStatus == ConversationQualityStatusNeedsReview {
		value := false
		req.Exportable = &value
	}
	return req
}

func normalizeConversationQualityErrors(values []QualityError) []QualityError {
	out := make([]QualityError, 0, len(values))
	for _, value := range values {
		code := strings.TrimSpace(value.Code)
		message := strings.TrimSpace(value.Message)
		source := strings.TrimSpace(value.Source)
		if code == "" && message == "" {
			continue
		}
		if len([]rune(code)) > 64 {
			code = string([]rune(code)[:64])
		}
		if len([]rune(message)) > 200 {
			message = string([]rune(message)[:200])
		}
		if len([]rune(source)) > 64 {
			source = string([]rune(source)[:64])
		}
		out = append(out, QualityError{Code: code, Message: message, Source: source})
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func normalizeConversationExportMessagesRequest(req ConversationExportMessagesJSONLRequest) ConversationExportMessagesJSONLRequest {
	if req.Dedupe != nil {
		req.IncludeDuplicates = !*req.Dedupe
	}
	return req
}

func buildConversationExportMessages(turn ConversationTurn, redactionEnabled bool) []json.RawMessage {
	messages := append([]json.RawMessage{}, turn.RequestMessages...)
	messages = append(messages, turn.ResponseMessages...)
	if !redactionEnabled {
		return messages
	}
	return redactConversationMessages(messages)
}

func clampPageSize(pageSize int) int {
	if pageSize <= 0 {
		return 20
	}
	if pageSize > 200 {
		return 200
	}
	return pageSize
}
