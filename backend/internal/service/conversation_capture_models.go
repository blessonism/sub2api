package service

import (
	"context"
	"encoding/json"
	"time"
)

const (
	ConversationProviderOpenAI = "openai"

	ConversationParseStatusSuccess = "success"
	ConversationParseStatusFailed  = "failed"

	ConversationQualityStatusUnchecked   = "unchecked"
	ConversationQualityStatusClean       = "clean"
	ConversationQualityStatusNeedsReview = "needs_review"
	ConversationQualityStatusRejected    = "rejected"

	ConversationSessionSourceExplicit  = "explicit"
	ConversationSessionSourceHeuristic = "heuristic"
	ConversationSessionSourceSingle    = "single_turn"
	ConversationSessionSourceResponses = "responses"

	ConversationCaptureEndpointChatCompletions = "chat_completions"
	ConversationCaptureEndpointResponses       = "responses"

	ConversationCaptureSubjectFilterModeBlacklist = "blacklist"
	ConversationCaptureSubjectFilterModeWhitelist = "whitelist"
)

type ConversationCaptureConfig struct {
	Enabled                bool    `json:"enabled"`
	SamplePercent          int     `json:"sample_percent"`
	CaptureChatCompletions bool    `json:"capture_chat_completions"`
	CaptureResponses       bool    `json:"capture_responses"`
	RawArchiveEnabled      bool    `json:"raw_archive_enabled"`
	MaxTurnPayloadBytes    int     `json:"max_turn_payload_bytes"`
	PayloadPreviewChars    int     `json:"payload_preview_chars"`
	SessionWindowMinutes   int     `json:"session_window_minutes"`
	RetentionDays          int     `json:"retention_days"`
	ExportEnabled          bool    `json:"export_enabled"`
	SubjectFilterMode      string  `json:"subject_filter_mode"`
	ExcludedUserIDs        []int64 `json:"excluded_user_ids"`
	ExcludedAPIKeyIDs      []int64 `json:"excluded_api_key_ids"`
	IncludedUserIDs        []int64 `json:"included_user_ids"`
	IncludedAPIKeyIDs      []int64 `json:"included_api_key_ids"`
}

type ConversationCaptureDecision struct {
	Capture              bool
	EndpointKind         string
	MaxTurnPayloadBytes  int
	PayloadPreviewChars  int
	SessionWindowMinutes int
	RetentionDays        int
}

type ConversationCaptureSubject struct {
	UserID   int64
	APIKeyID int64
}

type ConversationCaptureMeta struct {
	RequestID          string
	UpstreamRequestID  string
	ClientRequestID    string
	ConversationID     string
	ResponseID         string
	PreviousResponseID string
	EndpointKind       string
	UserID             int64
	APIKeyID           int64
	AccountID          int64
	Model              string
	UpstreamModel      string
	RequestPath        string
	Stream             bool
	Usage              OpenAIUsage
	ActualCost         float64
	RequestBody        []byte
	ClientDisconnect   bool
}

type ConversationCaptureInput struct {
	Meta             ConversationCaptureMeta
	ResponseRaw      []byte
	ClientDisconnect bool
	Truncated        bool
}

type ConversationSession struct {
	ID                 int64          `json:"id"`
	SessionID          string         `json:"session_id"`
	UserID             int64          `json:"user_id"`
	APIKeyID           int64          `json:"api_key_id"`
	AccountID          *int64         `json:"account_id,omitempty"`
	Provider           string         `json:"provider"`
	Model              string         `json:"model"`
	RequestPath        string         `json:"request_path"`
	Status             string         `json:"status"`
	TurnCount          int            `json:"turn_count"`
	SourceRequestCount int            `json:"source_request_count"`
	InputTokens        int64          `json:"input_tokens"`
	OutputTokens       int64          `json:"output_tokens"`
	TotalTokens        int64          `json:"total_tokens"`
	ActualCost         float64        `json:"actual_cost"`
	QualityStatus      string         `json:"quality_status"`
	QualityErrors      []QualityError `json:"quality_errors"`
	Exportable         bool           `json:"exportable"`
	DuplicateTurnCount int64          `json:"duplicate_turn_count"`
	CaptureStatus      string         `json:"capture_status"`
	SessionSource      string         `json:"session_source"`
	RetentionUntil     time.Time      `json:"retention_until"`
	StartedAt          time.Time      `json:"started_at"`
	EndedAt            time.Time      `json:"ended_at"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type ConversationTurn struct {
	ID                int64             `json:"id"`
	SessionID         string            `json:"session_id"`
	RequestID         string            `json:"request_id"`
	UpstreamRequestID *string           `json:"upstream_request_id,omitempty"`
	ClientRequestID   *string           `json:"client_request_id,omitempty"`
	TurnIndex         int               `json:"turn_index"`
	Provider          string            `json:"provider"`
	Model             string            `json:"model"`
	RequestPath       string            `json:"request_path"`
	RequestMessages   []json.RawMessage `json:"request_messages"`
	ResponseMessages  []json.RawMessage `json:"response_messages"`
	Tools             []json.RawMessage `json:"tools"`
	Usage             map[string]any    `json:"usage"`
	Meta              map[string]any    `json:"meta"`
	InputTokens       int64             `json:"input_tokens"`
	OutputTokens      int64             `json:"output_tokens"`
	TotalTokens       int64             `json:"total_tokens"`
	ActualCost        float64           `json:"actual_cost"`
	Stream            bool              `json:"stream"`
	ClientDisconnect  bool              `json:"client_disconnect"`
	Truncated         bool              `json:"truncated"`
	QualityStatus     string            `json:"quality_status"`
	QualityErrors     []QualityError    `json:"quality_errors"`
	Exportable        bool              `json:"exportable"`
	ParseStatus       string            `json:"parse_status"`
	ParseError        *string           `json:"parse_error,omitempty"`
	DedupeHash        string            `json:"dedupe_hash"`
	RawArchiveKey     *string           `json:"raw_archive_key,omitempty"`
	PayloadPreview    *string           `json:"payload_preview,omitempty"`
	RetentionUntil    time.Time         `json:"retention_until"`
	CreatedAt         time.Time         `json:"created_at"`
}

type ConversationTurnSummary struct {
	ID                int64          `json:"id"`
	SessionID         string         `json:"session_id"`
	RequestID         string         `json:"request_id"`
	UpstreamRequestID *string        `json:"upstream_request_id,omitempty"`
	ClientRequestID   *string        `json:"client_request_id,omitempty"`
	TurnIndex         int            `json:"turn_index"`
	Provider          string         `json:"provider"`
	Model             string         `json:"model"`
	RequestPath       string         `json:"request_path"`
	InputTokens       int64          `json:"input_tokens"`
	OutputTokens      int64          `json:"output_tokens"`
	TotalTokens       int64          `json:"total_tokens"`
	ActualCost        float64        `json:"actual_cost"`
	Stream            bool           `json:"stream"`
	ClientDisconnect  bool           `json:"client_disconnect"`
	Truncated         bool           `json:"truncated"`
	QualityStatus     string         `json:"quality_status"`
	QualityErrors     []QualityError `json:"quality_errors"`
	Exportable        bool           `json:"exportable"`
	ParseStatus       string         `json:"parse_status"`
	ParseError        *string        `json:"parse_error,omitempty"`
	DedupeHash        string         `json:"dedupe_hash"`
	DuplicateCount    int64          `json:"duplicate_count"`
	PayloadPreview    *string        `json:"payload_preview,omitempty"`
	RetentionUntil    time.Time      `json:"retention_until"`
	CreatedAt         time.Time      `json:"created_at"`
}

type ConversationSessionFilters struct {
	UserID        int64      `json:"user_id,omitempty"`
	APIKeyID      int64      `json:"api_key_id,omitempty"`
	Model         string     `json:"model,omitempty"`
	RequestID     string     `json:"request_id,omitempty"`
	QualityStatus string     `json:"quality_status,omitempty"`
	Exportable    *bool      `json:"exportable,omitempty"`
	StartedAtFrom *time.Time `json:"started_at_from,omitempty"`
	StartedAtTo   *time.Time `json:"started_at_to,omitempty"`
}

type ConversationExportMessagesJSONLRequest struct {
	ConversationSessionFilters
	IncludeHeuristic  bool  `json:"include_heuristic"`
	IncludeDuplicates bool  `json:"include_duplicates"`
	RedactionEnabled  bool  `json:"redaction_enabled"`
	Dedupe            *bool `json:"dedupe,omitempty"`
	Limit             int   `json:"limit,omitempty"`
}

type QualityError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

type ConversationQualityUpdateRequest struct {
	QualityStatus string         `json:"quality_status"`
	QualityErrors []QualityError `json:"quality_errors"`
	Exportable    *bool          `json:"exportable,omitempty"`
}

type ConversationBulkQualityUpdateRequest struct {
	SessionIDs    []int64        `json:"session_ids,omitempty"`
	TurnIDs       []int64        `json:"turn_ids,omitempty"`
	QualityStatus string         `json:"quality_status"`
	QualityErrors []QualityError `json:"quality_errors"`
	Exportable    *bool          `json:"exportable,omitempty"`
}

const (
	ConversationExportJobStatusPending   = "pending"
	ConversationExportJobStatusRunning   = "running"
	ConversationExportJobStatusCompleted = "completed"
	ConversationExportJobStatusFailed    = "failed"
	ConversationExportJobStatusExpired   = "expired"
	ConversationExportJobStatusDeleted   = "deleted"

	ConversationExportFormatMessagesJSONL = "messages_jsonl"
	ConversationExportEncodingPlain       = "plain"
	ConversationExportEncodingZstd        = "zstd"
)

type ConversationExportJob struct {
	ID                   int64                        `json:"id"`
	Status               string                       `json:"status"`
	Filters              ConversationExportJobFilters `json:"filters"`
	Format               string                       `json:"format"`
	Encoding             string                       `json:"encoding"`
	SessionCount         int64                        `json:"session_count"`
	TurnCount            int64                        `json:"turn_count"`
	FileSize             int64                        `json:"file_size"`
	S3Key                *string                      `json:"s3_key,omitempty"`
	DownloadURLExpiresAt *time.Time                   `json:"download_url_expires_at,omitempty"`
	ExpiresAt            time.Time                    `json:"expires_at"`
	ErrorMessage         *string                      `json:"error_message,omitempty"`
	CreatedBy            int64                        `json:"created_by"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
	StartedAt            *time.Time                   `json:"started_at,omitempty"`
	CompletedAt          *time.Time                   `json:"completed_at,omitempty"`
}

type ConversationExportJobFilters struct {
	UserID            int64      `json:"user_id,omitempty"`
	APIKeyID          int64      `json:"api_key_id,omitempty"`
	Model             string     `json:"model,omitempty"`
	RequestID         string     `json:"request_id,omitempty"`
	QualityStatus     string     `json:"quality_status,omitempty"`
	StartedAtFrom     *time.Time `json:"started_at_from,omitempty"`
	StartedAtTo       *time.Time `json:"started_at_to,omitempty"`
	IncludeHeuristic  bool       `json:"include_heuristic"`
	IncludeDuplicates bool       `json:"include_duplicates"`
	RedactionEnabled  bool       `json:"redaction_enabled"`
	Dedupe            bool       `json:"dedupe"`
	Limit             int        `json:"limit,omitempty"`
}

type ConversationCreateExportJobRequest struct {
	Filters  ConversationExportJobFilters `json:"filters"`
	Format   string                       `json:"format"`
	Encoding string                       `json:"encoding"`
}

type ConversationExportDownloadTicket struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ConversationMergeSessionsRequest struct {
	SourceSessionIDs []int64 `json:"source_session_ids"`
	TargetSessionID  int64   `json:"target_session_id"`
}

type ConversationSplitSessionRequest struct {
	TurnID int64 `json:"turn_id"`
}

type ConversationMoveTurnRequest struct {
	TargetSessionID int64 `json:"target_session_id"`
}

type ConversationRepository interface {
	UpsertTurn(ctx context.Context, record ConversationTurnRecord) error
	ListSessions(ctx context.Context, filters ConversationSessionFilters, page, pageSize int) ([]ConversationSession, int64, error)
	GetSessionByID(ctx context.Context, id int64) (*ConversationSession, error)
	ListTurnsBySessionID(ctx context.Context, sessionID string, page, pageSize int) ([]ConversationTurnSummary, int64, error)
	GetTurnByID(ctx context.Context, id int64) (*ConversationTurn, error)
	SetSessionExportable(ctx context.Context, id int64, exportable bool) error
	SetTurnExportable(ctx context.Context, id int64, exportable bool) error
	SetSessionQuality(ctx context.Context, id int64, status string, errors []QualityError, exportable *bool) error
	SetTurnQuality(ctx context.Context, id int64, status string, errors []QualityError, exportable *bool) error
	BulkSetQuality(ctx context.Context, req ConversationBulkQualityUpdateRequest) error
	MergeSessions(ctx context.Context, targetID int64, sourceIDs []int64) error
	SplitSessionFromTurn(ctx context.Context, sessionID int64, turnID int64, newSessionID string, now time.Time) (*ConversationSession, error)
	MoveTurn(ctx context.Context, turnID int64, targetSessionID int64) error
	ListExportableTurns(ctx context.Context, req ConversationExportMessagesJSONLRequest) ([]ConversationTurn, error)
	FindSessionIDByResponseID(ctx context.Context, responseID string) (string, error)
	CreateExportJob(ctx context.Context, job ConversationExportJob) (*ConversationExportJob, error)
	ListExportJobs(ctx context.Context, page, pageSize int) ([]ConversationExportJob, int64, error)
	GetExportJobByID(ctx context.Context, id int64) (*ConversationExportJob, error)
	MarkExportJobRunning(ctx context.Context, id int64, startedAt time.Time) error
	CompleteExportJob(ctx context.Context, id int64, sessionCount, turnCount, fileSize int64, s3Key string, completedAt time.Time) error
	FailExportJob(ctx context.Context, id int64, errorMessage string, failedAt time.Time) error
	SetExportJobDownloadURLExpiresAt(ctx context.Context, id int64, expiresAt time.Time) error
	MarkExportJobDeleted(ctx context.Context, id int64, deletedAt time.Time) (*ConversationExportJob, error)
	MarkExpiredExportJobs(ctx context.Context, now time.Time, limit int) ([]ConversationExportJob, error)
	MarkExportJobsExpired(ctx context.Context, ids []int64, now time.Time) error
	CleanupExpired(ctx context.Context, now time.Time, limit int) (int64, int64, error)
}

type ConversationTurnRecord struct {
	SessionID         string
	SessionSource     string
	UserID            int64
	APIKeyID          int64
	AccountID         int64
	Provider          string
	Model             string
	RequestPath       string
	RequestID         string
	UpstreamRequestID string
	ClientRequestID   string
	TurnIndex         int
	RequestMessages   []json.RawMessage
	ResponseMessages  []json.RawMessage
	Tools             []json.RawMessage
	Usage             map[string]any
	Meta              map[string]any
	InputTokens       int64
	OutputTokens      int64
	TotalTokens       int64
	ActualCost        float64
	Stream            bool
	ClientDisconnect  bool
	Truncated         bool
	QualityStatus     string
	QualityErrors     []QualityError
	Exportable        bool
	ParseStatus       string
	ParseError        string
	DedupeHash        string
	RawArchiveKey     string
	PayloadPreview    string
	RetentionUntil    time.Time
	CreatedAt         time.Time
}
