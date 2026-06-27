package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	UpstreamRelayAuthModeManualSession = "manual_session"
	UpstreamRelayAuthModePasswordLogin = "password_login"

	UpstreamRelayConnectorStatusActive      = "active"
	UpstreamRelayConnectorStatusNeedsReauth = "needs_reauth"
	UpstreamRelayConnectorStatusInvalid     = "invalid"
	UpstreamRelayConnectorStatusPaused      = "paused"

	UpstreamRelayRateSourceAvailable  = "login_available_groups"
	UpstreamRelayRateSourceOverride   = "login_user_group_rates"
	UpstreamRelayRateSourceUsageDelta = "usage_cost_delta"

	UpstreamRelayRunStatusSuccess = "success"
	UpstreamRelayRunStatusFailed  = "failed"

	upstreamRelayDefaultPriorityStart = 10
	upstreamRelayPriorityStep         = 10
	upstreamRelayHTTPTimeout          = 20 * time.Second
	upstreamRelaySnapshotFreshness    = 24 * time.Hour
	upstreamRelayUsageDeltaFreshness  = 24 * time.Hour
	upstreamRelayProbeFreshness       = 30 * time.Minute
	upstreamRelaySnapshotStatusStale  = "stale"
)

var (
	ErrUpstreamRelayConnectorNotFound = infraerrors.NotFound(
		"UPSTREAM_RELAY_CONNECTOR_NOT_FOUND", "upstream relay connector not found",
	)
	ErrUpstreamRelayCandidateNotFound = infraerrors.NotFound(
		"UPSTREAM_RELAY_CANDIDATE_NOT_FOUND", "upstream relay candidate not found",
	)
	ErrUpstreamRelayRunNotFound = infraerrors.NotFound(
		"UPSTREAM_RELAY_RECOMMENDATION_RUN_NOT_FOUND", "upstream relay recommendation run not found",
	)
	ErrUpstreamRelayInvalidManualSession = infraerrors.BadRequest(
		"UPSTREAM_RELAY_INVALID_MANUAL_SESSION", "manual_session requires base_url and authorization bearer token",
	)
	ErrUpstreamRelayInvalidPasswordLogin = infraerrors.BadRequest(
		"UPSTREAM_RELAY_INVALID_PASSWORD_LOGIN", "password_login requires base_url, login_email and login_password",
	)
	ErrUpstreamRelayPasswordLoginNeedsManualSession = infraerrors.BadRequest(
		"UPSTREAM_RELAY_PASSWORD_LOGIN_NEEDS_MANUAL_SESSION", "upstream login requires browser verification or 2FA; use manual_session instead",
	)
	ErrUpstreamRelayNoPendingSuggestions = infraerrors.BadRequest(
		"UPSTREAM_RELAY_NO_PENDING_SUGGESTIONS", "recommendation run has no pending suggestions",
	)
)

type UpstreamRelayConnector struct {
	ID                      int64      `json:"id"`
	Name                    string     `json:"name"`
	BaseURL                 string     `json:"base_url"`
	AuthMode                string     `json:"auth_mode"`
	Status                  string     `json:"status"`
	CredentialVersion       int64      `json:"credential_version"`
	BearerTokenMasked       string     `json:"bearer_token_masked,omitempty"`
	RefreshTokenMasked      string     `json:"refresh_token_masked,omitempty"`
	LoginEmailMasked        string     `json:"login_email_masked,omitempty"`
	CookieMasked            string     `json:"cookie_masked,omitempty"`
	UserAgentMasked         string     `json:"user_agent_masked,omitempty"`
	HasBearerToken          bool       `json:"has_bearer_token"`
	HasRefreshToken         bool       `json:"has_refresh_token"`
	HasLoginEmail           bool       `json:"has_login_email"`
	HasCookie               bool       `json:"has_cookie"`
	HasUserAgent            bool       `json:"has_user_agent"`
	LastVerifiedAt          *time.Time `json:"last_verified_at,omitempty"`
	LastSyncedAt            *time.Time `json:"last_synced_at,omitempty"`
	LastError               string     `json:"last_error,omitempty"`
	CreatedBy               int64      `json:"created_by,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	BearerTokenEncrypted    string     `json:"-"`
	RefreshTokenEncrypted   string     `json:"-"`
	LoginEmailEncrypted     string     `json:"-"`
	CookieEncrypted         string     `json:"-"`
	UserAgentEncrypted      string     `json:"-"`
	BearerTokenPlain        string     `json:"-"`
	RefreshTokenPlain       string     `json:"-"`
	LoginEmailPlain         string     `json:"-"`
	CookiePlain             string     `json:"-"`
	UserAgentPlain          string     `json:"-"`
	BearerTokenDecryptFail  bool       `json:"-"`
	RefreshTokenDecryptFail bool       `json:"-"`
	LoginEmailDecryptFail   bool       `json:"-"`
	CookieDecryptFail       bool       `json:"-"`
	UserAgentDecryptFail    bool       `json:"-"`
}

type UpstreamRelayConnectorInput struct {
	Name              string  `json:"name"`
	BaseURL           string  `json:"base_url"`
	AuthMode          string  `json:"auth_mode"`
	BearerToken       *string `json:"bearer_token,omitempty"`
	LoginEmail        *string `json:"login_email,omitempty"`
	LoginPassword     *string `json:"login_password,omitempty"`
	Cookie            *string `json:"cookie,omitempty"`
	UserAgent         *string `json:"user_agent,omitempty"`
	SkipImmediateSync bool    `json:"-"`
}

type UpstreamRelayGroupRateSnapshot struct {
	ID                     int64     `json:"id"`
	ConnectorID            int64     `json:"connector_id"`
	UpstreamGroupID        string    `json:"upstream_group_id"`
	Name                   string    `json:"name"`
	Platform               string    `json:"platform"`
	Status                 string    `json:"status"`
	DefaultRateMultiplier  float64   `json:"default_rate_multiplier"`
	OverrideRateMultiplier *float64  `json:"override_rate_multiplier,omitempty"`
	FinalRateMultiplier    float64   `json:"final_rate_multiplier"`
	Source                 string    `json:"source"`
	LastSeenAt             time.Time `json:"last_seen_at"`
	CreatedAt              time.Time `json:"created_at,omitempty"`
	UpdatedAt              time.Time `json:"updated_at,omitempty"`
}

type UpstreamRelayCandidate struct {
	ID                int64                           `json:"id"`
	ConnectorID       int64                           `json:"connector_id"`
	ConnectorName     string                          `json:"connector_name,omitempty"`
	ConnectorStatus   string                          `json:"connector_status,omitempty"`
	AccountID         int64                           `json:"account_id"`
	AccountName       string                          `json:"account_name,omitempty"`
	AccountPlatform   string                          `json:"account_platform,omitempty"`
	UpstreamGroupID   string                          `json:"upstream_group_id"`
	UpstreamGroupName string                          `json:"upstream_group_name,omitempty"`
	ProbeModel        string                          `json:"probe_model"`
	ProbeProtocol     string                          `json:"probe_protocol"`
	TargetGroupID     int64                           `json:"target_group_id"`
	TargetGroupName   string                          `json:"target_group_name,omitempty"`
	CurrentPriority   *int                            `json:"current_priority,omitempty"`
	Enabled           bool                            `json:"enabled"`
	Notes             string                          `json:"notes"`
	LastProbeResultID *int64                          `json:"last_probe_result_id,omitempty"`
	LatestProbe       *UpstreamRelayProbeResult       `json:"latest_probe,omitempty"`
	LatestSnapshot    *UpstreamRelayGroupRateSnapshot `json:"latest_snapshot,omitempty"`
	Health            *UpstreamRelayCandidateHealth   `json:"health,omitempty"`
	LatestUsageDelta  *UpstreamRelayUsageDeltaSample  `json:"latest_usage_delta,omitempty"`
	CreatedBy         int64                           `json:"created_by,omitempty"`
	CreatedAt         time.Time                       `json:"created_at"`
	UpdatedAt         time.Time                       `json:"updated_at"`
}

type UpstreamRelayCandidateInput struct {
	ConnectorID     int64  `json:"connector_id"`
	AccountID       int64  `json:"account_id"`
	UpstreamGroupID string `json:"upstream_group_id"`
	ProbeModel      string `json:"probe_model"`
	ProbeProtocol   string `json:"probe_protocol"`
	TargetGroupID   int64  `json:"target_group_id"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Notes           string `json:"notes"`
}

type UpstreamRelayProbeResult struct {
	ID           int64     `json:"id"`
	CandidateID  int64     `json:"candidate_id"`
	Success      bool      `json:"success"`
	LatencyMs    *int      `json:"latency_ms,omitempty"`
	HTTPStatus   *int      `json:"http_status,omitempty"`
	ErrorClass   string    `json:"error_class,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ProbedAt     time.Time `json:"probed_at"`
}

type UpstreamRelayCandidateHealth struct {
	ProbeCount           int        `json:"probe_count"`
	SuccessCount         int        `json:"success_count"`
	SuccessRate          float64    `json:"success_rate"`
	AvgLatencyMs         *int       `json:"avg_latency_ms,omitempty"`
	P95LatencyMs         *int       `json:"p95_latency_ms,omitempty"`
	ConsecutiveSuccesses int        `json:"consecutive_successes"`
	ConsecutiveFailures  int        `json:"consecutive_failures"`
	LastErrorClass       string     `json:"last_error_class,omitempty"`
	LastSuccessAt        *time.Time `json:"last_success_at,omitempty"`
	WindowMinutes        int        `json:"window_minutes"`
	SampleSize           int        `json:"sample_size"`
}

type UpstreamRelayUsageDeltaSample struct {
	ID                    int64     `json:"id,omitempty"`
	CandidateID           int64     `json:"candidate_id"`
	ProbeResultID         *int64    `json:"probe_result_id,omitempty"`
	Model                 string    `json:"model"`
	Status                string    `json:"status"`
	BeforeCost            *float64  `json:"before_cost,omitempty"`
	BeforeActualCost      *float64  `json:"before_actual_cost,omitempty"`
	AfterCost             *float64  `json:"after_cost,omitempty"`
	AfterActualCost       *float64  `json:"after_actual_cost,omitempty"`
	CostDelta             *float64  `json:"cost_delta,omitempty"`
	ActualCostDelta       *float64  `json:"actual_cost_delta,omitempty"`
	DerivedRateMultiplier *float64  `json:"derived_rate_multiplier,omitempty"`
	UnreliableReason      string    `json:"unreliable_reason,omitempty"`
	SampledAt             time.Time `json:"sampled_at"`
}

type UpstreamRelayRecommendationRun struct {
	ID              int64                                   `json:"id"`
	Status          string                                  `json:"status"`
	TotalCandidates int                                     `json:"total_candidates"`
	SuggestionCount int                                     `json:"suggestion_count"`
	Applied         bool                                    `json:"applied"`
	AppliedBy       *int64                                  `json:"applied_by,omitempty"`
	AppliedAt       *time.Time                              `json:"applied_at,omitempty"`
	ErrorMessage    string                                  `json:"error_message,omitempty"`
	CreatedBy       int64                                   `json:"created_by,omitempty"`
	CreatedAt       time.Time                               `json:"created_at"`
	Suggestions     []UpstreamRelayRecommendationSuggestion `json:"suggestions,omitempty"`
}

type UpstreamRelayRecommendationSuggestion struct {
	ID                  int64      `json:"id,omitempty"`
	RunID               int64      `json:"run_id,omitempty"`
	CandidateID         int64      `json:"candidate_id"`
	ConnectorID         int64      `json:"connector_id"`
	ConnectorName       string     `json:"connector_name,omitempty"`
	AccountID           int64      `json:"account_id"`
	AccountName         string     `json:"account_name,omitempty"`
	UpstreamGroupID     string     `json:"upstream_group_id"`
	UpstreamGroupName   string     `json:"upstream_group_name,omitempty"`
	TargetGroupID       int64      `json:"target_group_id"`
	TargetGroupName     string     `json:"target_group_name,omitempty"`
	OldPriority         *int       `json:"old_priority,omitempty"`
	NewPriority         int        `json:"new_priority"`
	FinalRateMultiplier float64    `json:"final_rate_multiplier"`
	HealthStatus        string     `json:"health_status"`
	ReasonCode          string     `json:"reason_code"`
	Confidence          string     `json:"confidence"`
	HealthSummary       string     `json:"health_summary"`
	RateSource          string     `json:"rate_source"`
	Reason              string     `json:"reason"`
	Applied             bool       `json:"applied"`
	AppliedBy           *int64     `json:"applied_by,omitempty"`
	AppliedAt           *time.Time `json:"applied_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at,omitempty"`
}

type UpstreamRelayConnectorListFilters struct {
	Status string
	Search string
}

type UpstreamRelayCandidateListFilters struct {
	ConnectorID int64
	Enabled     *bool
}

type UpstreamRelayRepository interface {
	ListConnectors(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayConnectorListFilters) ([]UpstreamRelayConnector, *pagination.PaginationResult, error)
	GetConnector(ctx context.Context, id int64) (*UpstreamRelayConnector, error)
	CreateConnector(ctx context.Context, connector *UpstreamRelayConnector) (*UpstreamRelayConnector, error)
	UpdateConnector(ctx context.Context, connector *UpstreamRelayConnector, credentialsUpdated bool) (*UpstreamRelayConnector, error)
	SoftDeleteConnector(ctx context.Context, id int64) error
	UpsertSnapshots(ctx context.Context, connectorID int64, snapshots []UpstreamRelayGroupRateSnapshot) error
	ListSnapshots(ctx context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error)
	MarkConnectorSync(ctx context.Context, connectorID int64, status string, errMessage string) error

	ListCandidates(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayCandidateListFilters) ([]UpstreamRelayCandidate, *pagination.PaginationResult, error)
	GetCandidate(ctx context.Context, id int64) (*UpstreamRelayCandidate, error)
	CreateCandidate(ctx context.Context, candidate *UpstreamRelayCandidate) (*UpstreamRelayCandidate, error)
	UpdateCandidate(ctx context.Context, candidate *UpstreamRelayCandidate) (*UpstreamRelayCandidate, error)
	SoftDeleteCandidate(ctx context.Context, id int64) error
	InsertProbeResult(ctx context.Context, result UpstreamRelayProbeResult) (*UpstreamRelayProbeResult, error)
	InsertUsageDeltaSample(ctx context.Context, sample UpstreamRelayUsageDeltaSample) (*UpstreamRelayUsageDeltaSample, error)

	ListRecommendationInputs(ctx context.Context) ([]UpstreamRelayCandidate, error)
	CreateRecommendationRun(ctx context.Context, run UpstreamRelayRecommendationRun, suggestions []UpstreamRelayRecommendationSuggestion) (*UpstreamRelayRecommendationRun, error)
	GetRecommendationRun(ctx context.Context, id int64) (*UpstreamRelayRecommendationRun, error)
	ListRecommendationRuns(ctx context.Context, params pagination.PaginationParams) ([]UpstreamRelayRecommendationRun, *pagination.PaginationResult, error)
	ApplyRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error)
}

type UpstreamRelayGroupMonitoringService struct {
	repo        UpstreamRelayRepository
	accountRepo AccountRepository
	encryptor   SecretEncryptor
	httpClient  *http.Client
}

func NewUpstreamRelayGroupMonitoringService(repo UpstreamRelayRepository, accountRepo AccountRepository, encryptor SecretEncryptor) *UpstreamRelayGroupMonitoringService {
	return &UpstreamRelayGroupMonitoringService{
		repo:        repo,
		accountRepo: accountRepo,
		encryptor:   encryptor,
		httpClient:  &http.Client{Timeout: upstreamRelayHTTPTimeout},
	}
}

func (s *UpstreamRelayGroupMonitoringService) ListConnectors(ctx context.Context, page, pageSize int, filters UpstreamRelayConnectorListFilters) ([]UpstreamRelayConnector, *pagination.PaginationResult, error) {
	items, result, err := s.repo.ListConnectors(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		s.prepareConnectorResponse(&items[i])
	}
	return items, result, nil
}

func (s *UpstreamRelayGroupMonitoringService) GetConnector(ctx context.Context, id int64) (*UpstreamRelayConnector, error) {
	connector, err := s.repo.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	s.prepareConnectorResponse(connector)
	return connector, nil
}

func (s *UpstreamRelayGroupMonitoringService) CreateConnector(ctx context.Context, input UpstreamRelayConnectorInput, operatorID int64) (*UpstreamRelayConnector, error) {
	connector, credentialsUpdated, err := s.normalizeConnectorInput(ctx, input, nil, operatorID)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateConnector(ctx, connector)
	if err != nil {
		return nil, err
	}
	if !input.SkipImmediateSync {
		if _, err := s.SyncConnector(ctx, created.ID); err != nil {
			return nil, err
		}
	}
	if !credentialsUpdated {
		s.prepareConnectorResponse(created)
		return created, nil
	}
	return s.GetConnector(ctx, created.ID)
}

func (s *UpstreamRelayGroupMonitoringService) UpdateConnector(ctx context.Context, id int64, input UpstreamRelayConnectorInput) (*UpstreamRelayConnector, error) {
	existing, err := s.repo.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	connector, credentialsUpdated, err := s.normalizeConnectorInput(ctx, input, existing, existing.CreatedBy)
	if err != nil {
		return nil, err
	}
	connector.ID = id
	updated, err := s.repo.UpdateConnector(ctx, connector, credentialsUpdated)
	if err != nil {
		return nil, err
	}
	if !input.SkipImmediateSync {
		if _, err := s.SyncConnector(ctx, updated.ID); err != nil {
			return nil, err
		}
	}
	return s.GetConnector(ctx, updated.ID)
}

func (s *UpstreamRelayGroupMonitoringService) DeleteConnector(ctx context.Context, id int64) error {
	return s.repo.SoftDeleteConnector(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) SyncConnector(ctx context.Context, id int64) ([]UpstreamRelayGroupRateSnapshot, error) {
	connector, err := s.repo.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.decryptConnector(connector); err != nil {
		_ = s.repo.MarkConnectorSync(ctx, id, UpstreamRelayConnectorStatusNeedsReauth, err.Error())
		return nil, err
	}
	snapshots, err := s.fetchGroupSnapshots(ctx, connector)
	if err != nil {
		_ = s.repo.MarkConnectorSync(ctx, id, UpstreamRelayConnectorStatusNeedsReauth, sanitizeUpstreamRelayError(err.Error()))
		return nil, err
	}
	if err := s.repo.UpsertSnapshots(ctx, id, snapshots); err != nil {
		return nil, err
	}
	if err := s.repo.MarkConnectorSync(ctx, id, UpstreamRelayConnectorStatusActive, ""); err != nil {
		return nil, err
	}
	return s.repo.ListSnapshots(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) ListSnapshots(ctx context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error) {
	return s.repo.ListSnapshots(ctx, connectorID)
}

func (s *UpstreamRelayGroupMonitoringService) ListCandidates(ctx context.Context, page, pageSize int, filters UpstreamRelayCandidateListFilters) ([]UpstreamRelayCandidate, *pagination.PaginationResult, error) {
	return s.repo.ListCandidates(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
}

func (s *UpstreamRelayGroupMonitoringService) CreateCandidate(ctx context.Context, input UpstreamRelayCandidateInput, operatorID int64) (*UpstreamRelayCandidate, error) {
	candidate, err := normalizeUpstreamRelayCandidateInput(input, 0, operatorID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCandidate(ctx, candidate)
}

func (s *UpstreamRelayGroupMonitoringService) UpdateCandidate(ctx context.Context, id int64, input UpstreamRelayCandidateInput) (*UpstreamRelayCandidate, error) {
	existing, err := s.repo.GetCandidate(ctx, id)
	if err != nil {
		return nil, err
	}
	candidate, err := normalizeUpstreamRelayCandidateInput(input, id, existing.CreatedBy)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateCandidate(ctx, candidate)
}

func (s *UpstreamRelayGroupMonitoringService) DeleteCandidate(ctx context.Context, id int64) error {
	return s.repo.SoftDeleteCandidate(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) ProbeCandidate(ctx context.Context, id int64) (*UpstreamRelayProbeResult, error) {
	candidate, err := s.repo.GetCandidate(ctx, id)
	if err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, candidate.AccountID)
	if err != nil {
		return nil, err
	}
	sampleUsage := shouldSampleUsageDelta(*candidate, time.Now())
	var beforeUsage *upstreamRelayUsageSnapshot
	var beforeErr error
	if sampleUsage {
		beforeUsage, beforeErr = s.fetchUsageSnapshotForAccount(ctx, account)
	}
	probe := s.runCandidateProbe(ctx, *candidate, account)
	inserted, err := s.repo.InsertProbeResult(ctx, probe)
	if err != nil {
		return nil, err
	}
	if sampleUsage {
		afterUsage, afterErr := s.fetchUsageSnapshotForAccount(ctx, account)
		sample := buildUsageDeltaSample(*candidate, inserted, beforeUsage, beforeErr, afterUsage, afterErr)
		if sample != nil {
			_, _ = s.repo.InsertUsageDeltaSample(ctx, *sample)
		}
	}
	return inserted, nil
}

func (s *UpstreamRelayGroupMonitoringService) GenerateRecommendations(ctx context.Context, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	candidates, err := s.repo.ListRecommendationInputs(ctx)
	if err != nil {
		return nil, err
	}
	suggestions := buildUpstreamRelaySuggestions(candidates)
	run := UpstreamRelayRecommendationRun{
		Status:          UpstreamRelayRunStatusSuccess,
		TotalCandidates: len(candidates),
		SuggestionCount: len(suggestions),
		CreatedBy:       operatorID,
	}
	return s.repo.CreateRecommendationRun(ctx, run, suggestions)
}

func (s *UpstreamRelayGroupMonitoringService) GetRecommendationRun(ctx context.Context, id int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.GetRecommendationRun(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) ListRecommendationRuns(ctx context.Context, page, pageSize int) ([]UpstreamRelayRecommendationRun, *pagination.PaginationResult, error) {
	return s.repo.ListRecommendationRuns(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize})
}

func (s *UpstreamRelayGroupMonitoringService) ApplyRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.ApplyRecommendationRun(ctx, runID, operatorID)
}

func (s *UpstreamRelayGroupMonitoringService) normalizeConnectorInput(ctx context.Context, input UpstreamRelayConnectorInput, existing *UpstreamRelayConnector, operatorID int64) (*UpstreamRelayConnector, bool, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, false, infraerrors.BadRequest("UPSTREAM_RELAY_CONNECTOR_NAME_REQUIRED", "connector name is required")
	}
	baseURL := normalizeUpstreamRelayBaseURL(input.BaseURL)
	if baseURL == "" {
		return nil, false, ErrUpstreamRelayInvalidManualSession
	}
	authMode := strings.TrimSpace(input.AuthMode)
	if authMode == "" {
		authMode = UpstreamRelayAuthModeManualSession
	}
	if authMode != UpstreamRelayAuthModeManualSession && authMode != UpstreamRelayAuthModePasswordLogin {
		return nil, false, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_AUTH_MODE", "auth_mode must be manual_session or password_login")
	}
	connector := &UpstreamRelayConnector{
		Name:      name,
		BaseURL:   baseURL,
		AuthMode:  authMode,
		Status:    UpstreamRelayConnectorStatusInvalid,
		CreatedBy: operatorID,
	}
	if existing != nil {
		*connector = *existing
		connector.Name = name
		connector.BaseURL = baseURL
		connector.AuthMode = authMode
	}
	updated := false
	previousAuthMode := ""
	baseURLChanged := false
	if existing != nil {
		previousAuthMode = existing.AuthMode
		baseURLChanged = existing.BaseURL != baseURL
	}
	if authMode == UpstreamRelayAuthModePasswordLogin {
		loginEmail := strings.TrimSpace(stringValue(input.LoginEmail))
		loginPassword := strings.TrimSpace(stringValue(input.LoginPassword))
		inputEmailProvided := input.LoginEmail != nil && loginEmail != ""
		if loginEmail == "" && connector.LoginEmailEncrypted != "" && !baseURLChanged {
			loginEmail = strings.TrimSpace(s.decryptForDisplay(connector.LoginEmailEncrypted))
		}

		if loginPassword == "" && (baseURLChanged || existing == nil || existing.AuthMode != UpstreamRelayAuthModePasswordLogin || connector.BearerTokenEncrypted == "") {
			return nil, false, ErrUpstreamRelayInvalidPasswordLogin
		}
		if loginPassword == "" && inputEmailProvided && strings.TrimSpace(s.decryptForDisplay(connector.LoginEmailEncrypted)) != loginEmail {
			return nil, false, ErrUpstreamRelayInvalidPasswordLogin
		}

		if loginPassword != "" {
			if baseURLChanged && !inputEmailProvided {
				return nil, false, ErrUpstreamRelayInvalidPasswordLogin
			}
			token, err := s.loginUpstreamRelay(ctx, baseURL, loginEmail, loginPassword)
			if err != nil {
				return nil, false, err
			}
			if err := s.applyPasswordLoginToken(connector, token); err != nil {
				return nil, false, err
			}
			encrypted, err := s.encryptor.Encrypt(loginEmail)
			if err != nil {
				return nil, false, fmt.Errorf("encrypt login email: %w", err)
			}
			connector.LoginEmailEncrypted = encrypted
			updated = true
		}
		if connector.BearerTokenEncrypted == "" {
			return nil, false, ErrUpstreamRelayInvalidPasswordLogin
		}
		return connector, updated, nil
	}
	explicitBearerToken := strings.TrimSpace(stringValue(input.BearerToken)) != ""
	if (baseURLChanged || previousAuthMode == UpstreamRelayAuthModePasswordLogin) && !explicitBearerToken {
		return nil, false, ErrUpstreamRelayInvalidManualSession
	}
	connector.RefreshTokenEncrypted = ""
	connector.LoginEmailEncrypted = ""
	if baseURLChanged {
		connector.CookieEncrypted = ""
		connector.UserAgentEncrypted = ""
	}
	if explicitBearerToken {
		encrypted, err := s.encryptor.Encrypt(normalizeBearerToken(*input.BearerToken))
		if err != nil {
			return nil, false, fmt.Errorf("encrypt bearer token: %w", err)
		}
		connector.BearerTokenEncrypted = encrypted
		updated = true
	}
	if input.Cookie != nil && strings.TrimSpace(*input.Cookie) != "" {
		encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*input.Cookie))
		if err != nil {
			return nil, false, fmt.Errorf("encrypt cookie: %w", err)
		}
		connector.CookieEncrypted = encrypted
		updated = true
	}
	if input.UserAgent != nil && strings.TrimSpace(*input.UserAgent) != "" {
		encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*input.UserAgent))
		if err != nil {
			return nil, false, fmt.Errorf("encrypt user agent: %w", err)
		}
		connector.UserAgentEncrypted = encrypted
		updated = true
	}
	if connector.BearerTokenEncrypted == "" {
		return nil, false, ErrUpstreamRelayInvalidManualSession
	}
	return connector, updated, nil
}

func (s *UpstreamRelayGroupMonitoringService) prepareConnectorResponse(connector *UpstreamRelayConnector) {
	if connector == nil {
		return
	}
	connector.HasBearerToken = connector.BearerTokenEncrypted != ""
	connector.HasRefreshToken = connector.RefreshTokenEncrypted != ""
	connector.HasLoginEmail = connector.LoginEmailEncrypted != ""
	connector.HasCookie = connector.CookieEncrypted != ""
	connector.HasUserAgent = connector.UserAgentEncrypted != ""
	if connector.BearerTokenPlain == "" {
		connector.BearerTokenPlain = s.decryptForDisplay(connector.BearerTokenEncrypted)
	}
	if connector.RefreshTokenPlain == "" {
		connector.RefreshTokenPlain = s.decryptForDisplay(connector.RefreshTokenEncrypted)
	}
	if connector.LoginEmailPlain == "" {
		connector.LoginEmailPlain = s.decryptForDisplay(connector.LoginEmailEncrypted)
	}
	if connector.CookiePlain == "" {
		connector.CookiePlain = s.decryptForDisplay(connector.CookieEncrypted)
	}
	if connector.UserAgentPlain == "" {
		connector.UserAgentPlain = s.decryptForDisplay(connector.UserAgentEncrypted)
	}
	connector.BearerTokenMasked = maskSecret(connector.BearerTokenPlain)
	connector.RefreshTokenMasked = maskSecret(connector.RefreshTokenPlain)
	connector.LoginEmailMasked = maskEmail(connector.LoginEmailPlain)
	connector.CookieMasked = maskSecret(connector.CookiePlain)
	connector.UserAgentMasked = maskUserAgent(connector.UserAgentPlain)
	connector.BearerTokenPlain = ""
	connector.RefreshTokenPlain = ""
	connector.LoginEmailPlain = ""
	connector.CookiePlain = ""
	connector.UserAgentPlain = ""
}

func (s *UpstreamRelayGroupMonitoringService) decryptForDisplay(encrypted string) string {
	if s == nil || s.encryptor == nil || encrypted == "" {
		return ""
	}
	plain, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return ""
	}
	return plain
}

func (s *UpstreamRelayGroupMonitoringService) decryptConnector(connector *UpstreamRelayConnector) error {
	if connector == nil {
		return ErrUpstreamRelayConnectorNotFound
	}
	if connector.BearerTokenEncrypted != "" {
		plain, err := s.encryptor.Decrypt(connector.BearerTokenEncrypted)
		if err != nil {
			connector.BearerTokenDecryptFail = true
			return fmt.Errorf("decrypt bearer token: %w", err)
		}
		connector.BearerTokenPlain = plain
	}
	if connector.RefreshTokenEncrypted != "" {
		plain, err := s.encryptor.Decrypt(connector.RefreshTokenEncrypted)
		if err != nil {
			connector.RefreshTokenDecryptFail = true
			return fmt.Errorf("decrypt refresh token: %w", err)
		}
		connector.RefreshTokenPlain = plain
	}
	if connector.LoginEmailEncrypted != "" {
		plain, err := s.encryptor.Decrypt(connector.LoginEmailEncrypted)
		if err != nil {
			connector.LoginEmailDecryptFail = true
			return fmt.Errorf("decrypt login email: %w", err)
		}
		connector.LoginEmailPlain = plain
	}
	if connector.CookieEncrypted != "" {
		plain, err := s.encryptor.Decrypt(connector.CookieEncrypted)
		if err != nil {
			connector.CookieDecryptFail = true
			return fmt.Errorf("decrypt cookie: %w", err)
		}
		connector.CookiePlain = plain
	}
	if connector.UserAgentEncrypted != "" {
		plain, err := s.encryptor.Decrypt(connector.UserAgentEncrypted)
		if err != nil {
			connector.UserAgentDecryptFail = true
			return fmt.Errorf("decrypt user agent: %w", err)
		}
		connector.UserAgentPlain = plain
	}
	return nil
}

type upstreamRelayLoginToken struct {
	AccessToken  string
	RefreshToken string
}

func (s *UpstreamRelayGroupMonitoringService) applyPasswordLoginToken(connector *UpstreamRelayConnector, token upstreamRelayLoginToken) error {
	accessToken := normalizeBearerToken(token.AccessToken)
	if accessToken == "" {
		return ErrUpstreamRelayPasswordLoginNeedsManualSession
	}
	encryptedAccess, err := s.encryptor.Encrypt(accessToken)
	if err != nil {
		return fmt.Errorf("encrypt bearer token: %w", err)
	}
	connector.BearerTokenEncrypted = encryptedAccess
	connector.RefreshTokenEncrypted = ""
	if strings.TrimSpace(token.RefreshToken) != "" {
		encryptedRefresh, err := s.encryptor.Encrypt(strings.TrimSpace(token.RefreshToken))
		if err != nil {
			return fmt.Errorf("encrypt refresh token: %w", err)
		}
		connector.RefreshTokenEncrypted = encryptedRefresh
	}
	connector.CookieEncrypted = ""
	connector.UserAgentEncrypted = ""
	return nil
}

func (s *UpstreamRelayGroupMonitoringService) loginUpstreamRelay(ctx context.Context, baseURL, email, password string) (upstreamRelayLoginToken, error) {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return upstreamRelayLoginToken{}, ErrUpstreamRelayInvalidPasswordLogin
	}
	payload, err := json.Marshal(map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	})
	if err != nil {
		return upstreamRelayLoginToken{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		return upstreamRelayLoginToken{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return upstreamRelayLoginToken{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || looksLikeBrowserChallenge(body) {
		return upstreamRelayLoginToken{}, ErrUpstreamRelayPasswordLoginNeedsManualSession
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamRelayLoginToken{}, infraerrors.BadRequest("UPSTREAM_RELAY_PASSWORD_LOGIN_FAILED", sanitizeUpstreamRelayError(string(body)))
	}
	token, requires2FA, err := parseUpstreamRelayLoginToken(body)
	if err != nil {
		return upstreamRelayLoginToken{}, err
	}
	if requires2FA || token.AccessToken == "" {
		return upstreamRelayLoginToken{}, ErrUpstreamRelayPasswordLoginNeedsManualSession
	}
	return token, nil
}

func (s *UpstreamRelayGroupMonitoringService) fetchGroupSnapshots(ctx context.Context, connector *UpstreamRelayConnector) ([]UpstreamRelayGroupRateSnapshot, error) {
	availableBytes, err := s.getUpstreamJSON(ctx, connector, "/api/v1/groups/available")
	if err != nil {
		return nil, fmt.Errorf("fetch available groups: %w", err)
	}
	ratesBytes, err := s.getUpstreamJSON(ctx, connector, "/api/v1/groups/rates")
	if err != nil {
		return nil, fmt.Errorf("fetch group rates: %w", err)
	}
	available, err := parseAvailableGroups(availableBytes)
	if err != nil {
		return nil, err
	}
	rates, err := parseGroupRates(ratesBytes)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	snapshots := make([]UpstreamRelayGroupRateSnapshot, 0, len(available))
	for _, group := range available {
		finalRate := group.RateMultiplier
		source := UpstreamRelayRateSourceAvailable
		var override *float64
		if rate, ok := rates[group.ID]; ok {
			rateCopy := rate
			override = &rateCopy
			finalRate = rate
			source = UpstreamRelayRateSourceOverride
		}
		snapshots = append(snapshots, UpstreamRelayGroupRateSnapshot{
			ConnectorID:            connector.ID,
			UpstreamGroupID:        group.ID,
			Name:                   group.Name,
			Platform:               group.Platform,
			Status:                 group.Status,
			DefaultRateMultiplier:  group.RateMultiplier,
			OverrideRateMultiplier: override,
			FinalRateMultiplier:    finalRate,
			Source:                 source,
			LastSeenAt:             now,
		})
	}
	return snapshots, nil
}

func (s *UpstreamRelayGroupMonitoringService) getUpstreamJSON(ctx context.Context, connector *UpstreamRelayConnector, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(connector.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(connector.BearerTokenPlain))
	if strings.TrimSpace(connector.CookiePlain) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(connector.CookiePlain))
	}
	if strings.TrimSpace(connector.UserAgentPlain) != "" {
		req.Header.Set("User-Agent", strings.TrimSpace(connector.UserAgentPlain))
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, sanitizeUpstreamRelayError(string(body)))
	}
	return body, nil
}

func (s *UpstreamRelayGroupMonitoringService) runCandidateProbe(ctx context.Context, candidate UpstreamRelayCandidate, account *Account) UpstreamRelayProbeResult {
	result := UpstreamRelayProbeResult{CandidateID: candidate.ID, ProbedAt: time.Now(), ErrorClass: "unknown"}
	if account == nil {
		result.ErrorClass = "account_missing"
		result.ErrorMessage = "account not found"
		return result
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" && account.IsOpenAI() {
		apiKey = strings.TrimSpace(account.GetOpenAIAccessToken())
	}
	if apiKey == "" {
		result.ErrorClass = "missing_api_key"
		result.ErrorMessage = "bound account has no usable api key"
		return result
	}
	endpoint := resolveProbeEndpoint(account)
	if endpoint == "" {
		result.ErrorClass = "missing_endpoint"
		result.ErrorMessage = "bound account has no supported endpoint"
		return result
	}
	provider := upstreamRelayProviderFromAccount(account)
	if provider == "" {
		result.ErrorClass = "unsupported_platform"
		result.ErrorMessage = "bound account platform is not supported by phase 1 probe"
		return result
	}
	opts := &CheckOptions{APIMode: candidate.ProbeProtocol}
	start := time.Now()
	_, rawBody, status, err := callProvider(ctx, provider, endpoint, apiKey, candidate.ProbeModel, "answer: 2", opts)
	latency := int(time.Since(start) / time.Millisecond)
	result.LatencyMs = &latency
	if status > 0 {
		result.HTTPStatus = &status
	}
	if err != nil {
		result.ErrorClass = classifyProbeError(status, err.Error())
		result.ErrorMessage = truncateRelayMessage(sanitizeUpstreamRelayError(err.Error()))
		return result
	}
	if status < 200 || status >= 300 {
		result.ErrorClass = classifyProbeError(status, rawBody)
		result.ErrorMessage = truncateRelayMessage(sanitizeUpstreamRelayError(rawBody))
		return result
	}
	result.Success = true
	result.ErrorClass = ""
	result.ErrorMessage = ""
	return result
}

func normalizeUpstreamRelayCandidateInput(input UpstreamRelayCandidateInput, id int64, operatorID int64) (*UpstreamRelayCandidate, error) {
	upstreamGroupID := strings.TrimSpace(input.UpstreamGroupID)
	probeModel := strings.TrimSpace(input.ProbeModel)
	protocol := strings.TrimSpace(input.ProbeProtocol)
	if protocol == "" {
		protocol = MonitorAPIModeChatCompletions
	}
	if input.ConnectorID <= 0 || input.AccountID <= 0 || input.TargetGroupID <= 0 || upstreamGroupID == "" || probeModel == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_CANDIDATE", "connector_id, account_id, upstream_group_id, probe_model and target_group_id are required")
	}
	if protocol != MonitorAPIModeChatCompletions && protocol != MonitorAPIModeResponses {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_PROBE_PROTOCOL", "probe_protocol must be chat_completions or responses")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return &UpstreamRelayCandidate{
		ID:              id,
		ConnectorID:     input.ConnectorID,
		AccountID:       input.AccountID,
		UpstreamGroupID: upstreamGroupID,
		ProbeModel:      probeModel,
		ProbeProtocol:   protocol,
		TargetGroupID:   input.TargetGroupID,
		Enabled:         enabled,
		Notes:           strings.TrimSpace(input.Notes),
		CreatedBy:       operatorID,
	}, nil
}

func buildUpstreamRelaySuggestions(candidates []UpstreamRelayCandidate) []UpstreamRelayRecommendationSuggestion {
	now := time.Now()
	eligible := make([]UpstreamRelayCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !isUpstreamRelayCandidateEligible(candidate, now) {
			continue
		}
		eligible = append(eligible, candidate)
	}
	sortRelayCandidates(eligible)
	suggestions := make([]UpstreamRelayRecommendationSuggestion, 0, len(eligible))
	for i, candidate := range eligible {
		newPriority := upstreamRelayDefaultPriorityStart + i*upstreamRelayPriorityStep
		if candidate.CurrentPriority != nil && *candidate.CurrentPriority == newPriority {
			continue
		}
		rate, rateSource, _ := effectiveRelayRate(candidate, now)
		healthSummary := relayHealthSummary(candidate)
		suggestions = append(suggestions, UpstreamRelayRecommendationSuggestion{
			CandidateID:         candidate.ID,
			ConnectorID:         candidate.ConnectorID,
			AccountID:           candidate.AccountID,
			UpstreamGroupID:     candidate.UpstreamGroupID,
			TargetGroupID:       candidate.TargetGroupID,
			OldPriority:         candidate.CurrentPriority,
			NewPriority:         newPriority,
			FinalRateMultiplier: rate,
			HealthStatus:        relayHealthStatus(candidate),
			ReasonCode:          "rate_health_priority",
			Confidence:          relayRateConfidence(rateSource),
			HealthSummary:       healthSummary,
			RateSource:          rateSource,
			Reason:              fmt.Sprintf("上游倍率 %.4g，来源 %s，%s，按倍率、成功率与延迟建议 priority=%d", rate, rateSource, healthSummary, newPriority),
		})
	}
	return suggestions
}

func isUpstreamRelayCandidateEligible(candidate UpstreamRelayCandidate, now time.Time) bool {
	if !candidate.Enabled || candidate.ConnectorStatus != UpstreamRelayConnectorStatusActive {
		return false
	}
	if _, _, ok := effectiveRelayRate(candidate, now); !ok {
		return false
	}
	if candidate.LatestProbe == nil || !candidate.LatestProbe.Success {
		return false
	}
	if now.Sub(candidate.LatestProbe.ProbedAt) > upstreamRelayProbeFreshness {
		return false
	}
	if candidate.Health != nil {
		if candidate.Health.ConsecutiveFailures > 0 {
			return false
		}
		if candidate.Health.ProbeCount >= 3 && candidate.Health.SuccessRate < 0.5 {
			return false
		}
	}
	return true
}

func sortRelayCandidates(candidates []UpstreamRelayCandidate) {
	now := time.Now()
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftRate, rightRate := math.MaxFloat64, math.MaxFloat64
		if rate, _, ok := effectiveRelayRate(left, now); ok {
			leftRate = rate
		}
		if rate, _, ok := effectiveRelayRate(right, now); ok {
			rightRate = rate
		}
		if leftRate != rightRate {
			return leftRate < rightRate
		}
		leftSuccess, rightSuccess := relaySuccessRate(left), relaySuccessRate(right)
		if leftSuccess != rightSuccess {
			return leftSuccess > rightSuccess
		}
		leftLatency, rightLatency := relayP95Latency(left), relayP95Latency(right)
		if leftLatency != rightLatency {
			return leftLatency < rightLatency
		}
		return left.ID < right.ID
	})
}

func effectiveRelayRate(candidate UpstreamRelayCandidate, now time.Time) (float64, string, bool) {
	if hasFreshLoginRelayRate(candidate, now) {
		source := candidate.LatestSnapshot.Source
		if source == "" {
			source = UpstreamRelayRateSourceAvailable
		}
		return candidate.LatestSnapshot.FinalRateMultiplier, source, true
	}
	if hasFreshReliableUsageDelta(candidate, now) {
		return *candidate.LatestUsageDelta.DerivedRateMultiplier, UpstreamRelayRateSourceUsageDelta, true
	}
	return 0, "", false
}

func shouldSampleUsageDelta(candidate UpstreamRelayCandidate, now time.Time) bool {
	if hasFreshLoginRelayRate(candidate, now) {
		return false
	}
	return !hasFreshReliableUsageDelta(candidate, now)
}

func hasFreshLoginRelayRate(candidate UpstreamRelayCandidate, now time.Time) bool {
	return candidate.LatestSnapshot != nil &&
		candidate.LatestSnapshot.Status != upstreamRelaySnapshotStatusStale &&
		now.Sub(candidate.LatestSnapshot.LastSeenAt) <= upstreamRelaySnapshotFreshness
}

func hasFreshReliableUsageDelta(candidate UpstreamRelayCandidate, now time.Time) bool {
	return candidate.LatestUsageDelta != nil &&
		candidate.LatestUsageDelta.Status == "reliable" &&
		candidate.LatestUsageDelta.DerivedRateMultiplier != nil &&
		now.Sub(candidate.LatestUsageDelta.SampledAt) <= upstreamRelayUsageDeltaFreshness
}

func relayRateConfidence(source string) string {
	switch source {
	case UpstreamRelayRateSourceOverride:
		return "high"
	case UpstreamRelayRateSourceAvailable:
		return "medium"
	case UpstreamRelayRateSourceUsageDelta:
		return "low"
	default:
		return "unknown"
	}
}

func relayHealthStatus(candidate UpstreamRelayCandidate) string {
	if candidate.Health == nil {
		return "latest_success"
	}
	if candidate.Health.ConsecutiveFailures > 0 {
		return "degraded"
	}
	return "healthy"
}

func relayHealthSummary(candidate UpstreamRelayCandidate) string {
	if candidate.Health == nil || candidate.Health.ProbeCount == 0 {
		latency := 0
		if candidate.LatestProbe != nil && candidate.LatestProbe.LatencyMs != nil {
			latency = *candidate.LatestProbe.LatencyMs
		}
		return fmt.Sprintf("最近探测成功，延迟 %dms", latency)
	}
	latency := "-"
	if candidate.Health.P95LatencyMs != nil {
		latency = fmt.Sprintf("p95 %dms", *candidate.Health.P95LatencyMs)
	}
	return fmt.Sprintf("最近%d次成功率 %.0f%%，连续成功 %d，连续失败 %d，%s",
		candidate.Health.ProbeCount,
		candidate.Health.SuccessRate*100,
		candidate.Health.ConsecutiveSuccesses,
		candidate.Health.ConsecutiveFailures,
		latency,
	)
}

func relaySuccessRate(candidate UpstreamRelayCandidate) float64 {
	if candidate.Health == nil || candidate.Health.ProbeCount == 0 {
		if candidate.LatestProbe != nil && candidate.LatestProbe.Success {
			return 1
		}
		return 0
	}
	return candidate.Health.SuccessRate
}

func relayP95Latency(candidate UpstreamRelayCandidate) int {
	if candidate.Health != nil && candidate.Health.P95LatencyMs != nil {
		return *candidate.Health.P95LatencyMs
	}
	if candidate.LatestProbe != nil && candidate.LatestProbe.LatencyMs != nil {
		return *candidate.LatestProbe.LatencyMs
	}
	return math.MaxInt
}

type upstreamRelayAvailableGroup struct {
	ID             string
	Name           string
	Platform       string
	Status         string
	RateMultiplier float64
}

func parseAvailableGroups(body []byte) ([]upstreamRelayAvailableGroup, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse available groups: %w", err)
	}
	items := extractRelayArray(raw)
	groups := make([]upstreamRelayAvailableGroup, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := relayString(m["id"])
		if id == "" {
			id = relayString(m["group_id"])
		}
		if id == "" {
			continue
		}
		rate := relayFloat(m["rate_multiplier"], 1)
		groups = append(groups, upstreamRelayAvailableGroup{
			ID:             id,
			Name:           relayString(m["name"]),
			Platform:       relayString(m["platform"]),
			Status:         relayString(m["status"]),
			RateMultiplier: rate,
		})
	}
	return groups, nil
}

func parseGroupRates(body []byte) (map[string]float64, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse group rates: %w", err)
	}
	root, ok := raw.(map[string]any)
	if !ok {
		return map[string]float64{}, nil
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	out := make(map[string]float64, len(root))
	for key, value := range root {
		if nested, ok := value.(map[string]any); ok {
			out[key] = relayFloat(firstPresent(nested, "rate_multiplier", "rate", "multiplier"), 1)
			continue
		}
		out[key] = relayFloat(value, 1)
	}
	return out, nil
}

type upstreamRelayUsageSnapshot struct {
	Cost       float64
	ActualCost float64
}

func (s *UpstreamRelayGroupMonitoringService) fetchUsageSnapshotForAccount(ctx context.Context, account *Account) (*upstreamRelayUsageSnapshot, error) {
	if account == nil {
		return nil, fmt.Errorf("account not found")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" && account.IsOpenAI() {
		apiKey = strings.TrimSpace(account.GetOpenAIAccessToken())
	}
	if apiKey == "" {
		return nil, fmt.Errorf("bound account has no usable api key")
	}
	endpoint := resolveUsageEndpoint(account)
	if endpoint == "" {
		return nil, fmt.Errorf("bound account has no supported usage endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("usage HTTP %d: %s", resp.StatusCode, sanitizeUpstreamRelayError(string(body)))
	}
	return parseUsageSnapshot(body)
}

func parseUsageSnapshot(body []byte) (*upstreamRelayUsageSnapshot, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse usage: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return nil, fmt.Errorf("usage response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	cost, costOK := relayFloatOK(firstPresent(root, "cost", "total_cost"))
	actualCost, actualOK := relayFloatOK(firstPresent(root, "actual_cost", "total_actual_cost"))
	if !costOK || !actualOK {
		return nil, fmt.Errorf("usage response missing cost or actual_cost")
	}
	return &upstreamRelayUsageSnapshot{Cost: cost, ActualCost: actualCost}, nil
}

func buildUsageDeltaSample(candidate UpstreamRelayCandidate, probe *UpstreamRelayProbeResult, before *upstreamRelayUsageSnapshot, beforeErr error, after *upstreamRelayUsageSnapshot, afterErr error) *UpstreamRelayUsageDeltaSample {
	if before == nil && beforeErr == nil && after == nil && afterErr == nil {
		return nil
	}
	sample := &UpstreamRelayUsageDeltaSample{
		CandidateID: candidate.ID,
		Model:       candidate.ProbeModel,
		Status:      "insufficient",
		SampledAt:   time.Now(),
	}
	if probe != nil {
		sample.ProbeResultID = &probe.ID
	}
	if beforeErr != nil {
		sample.Status = "unavailable"
		sample.UnreliableReason = truncateRelayMessage(sanitizeUpstreamRelayError(beforeErr.Error()))
		return sample
	}
	if before != nil {
		sample.BeforeCost = &before.Cost
		sample.BeforeActualCost = &before.ActualCost
	}
	if afterErr != nil {
		sample.Status = "unavailable"
		sample.UnreliableReason = truncateRelayMessage(sanitizeUpstreamRelayError(afterErr.Error()))
		return sample
	}
	if after != nil {
		sample.AfterCost = &after.Cost
		sample.AfterActualCost = &after.ActualCost
	}
	if before == nil || after == nil {
		sample.UnreliableReason = "usage sample missing before or after snapshot"
		return sample
	}
	costDelta := after.Cost - before.Cost
	actualCostDelta := after.ActualCost - before.ActualCost
	sample.CostDelta = &costDelta
	sample.ActualCostDelta = &actualCostDelta
	if probe == nil || !probe.Success {
		sample.UnreliableReason = "probe did not succeed"
		return sample
	}
	if costDelta <= 0 || actualCostDelta <= 0 {
		sample.UnreliableReason = "usage delta is not positive"
		return sample
	}
	derived := costDelta / actualCostDelta
	sample.DerivedRateMultiplier = &derived
	sample.Status = "reliable"
	return sample
}

func parseUpstreamRelayLoginToken(body []byte) (upstreamRelayLoginToken, bool, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return upstreamRelayLoginToken{}, false, fmt.Errorf("parse upstream login response: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return upstreamRelayLoginToken{}, false, ErrUpstreamRelayPasswordLoginNeedsManualSession
	}
	if requires2FA(root) {
		return upstreamRelayLoginToken{}, true, nil
	}
	if data, ok := root["data"].(map[string]any); ok {
		if requires2FA(data) {
			return upstreamRelayLoginToken{}, true, nil
		}
		root = data
	}
	return upstreamRelayLoginToken{
		AccessToken:  relayString(firstPresent(root, "access_token", "token")),
		RefreshToken: relayString(root["refresh_token"]),
	}, false, nil
}

func requires2FA(m map[string]any) bool {
	if m == nil {
		return false
	}
	if v, ok := m["requires_2fa"].(bool); ok && v {
		return true
	}
	if v, ok := m["requires2fa"].(bool); ok && v {
		return true
	}
	return false
}

func looksLikeBrowserChallenge(body []byte) bool {
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "turnstile") ||
		strings.Contains(lower, "cf-challenge") ||
		strings.Contains(lower, "cloudflare") ||
		strings.Contains(lower, "<html")
}

func extractRelayArray(raw any) []any {
	if arr, ok := raw.([]any); ok {
		return arr
	}
	if root, ok := raw.(map[string]any); ok {
		for _, key := range []string{"data", "groups", "items"} {
			if arr, ok := root[key].([]any); ok {
				return arr
			}
		}
	}
	return nil
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func relayString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func relayFloat(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return fallback
}

func relayFloatOK(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func normalizeBearerToken(raw string) string {
	token := strings.TrimSpace(raw)
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	return strings.TrimSpace(token)
}

func normalizeUpstreamRelayBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/")
}

func maskSecret(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= 8 {
		return "****"
	}
	return raw[:4] + "..." + raw[len(raw)-4:]
}

func maskUserAgent(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 32 {
		return raw
	}
	return raw[:32] + "..."
}

func resolveProbeEndpoint(account *Account) string {
	if account == nil {
		return ""
	}
	switch {
	case account.IsOpenAI():
		return account.GetOpenAIBaseURL()
	case account.IsGemini():
		return account.GetGeminiBaseURL("https://generativelanguage.googleapis.com")
	default:
		return account.GetBaseURL()
	}
}

func resolveUsageEndpoint(account *Account) string {
	base := strings.TrimRight(resolveProbeEndpoint(account), "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/usage"
	}
	return base + "/v1/usage"
}

func upstreamRelayProviderFromAccount(account *Account) string {
	if account == nil {
		return ""
	}
	switch {
	case account.IsOpenAI():
		return MonitorProviderOpenAI
	case account.IsGemini():
		return MonitorProviderGemini
	case account.Platform == PlatformAnthropic:
		return MonitorProviderAnthropic
	default:
		return ""
	}
}

func classifyProbeError(status int, message string) string {
	lower := strings.ToLower(message)
	switch {
	case looksLikeBrowserChallenge([]byte(message)):
		return "browser_challenge"
	case status == http.StatusUnauthorized || status == http.StatusForbidden || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden"):
		return "auth_failed"
	case status == http.StatusTooManyRequests || strings.Contains(lower, "rate limit"):
		return "rate_limited"
	case status >= 500:
		return "upstream_5xx"
	case strings.Contains(lower, "insufficient") || strings.Contains(lower, "quota") || strings.Contains(lower, "balance"):
		return "insufficient_quota"
	case strings.Contains(lower, "context window") || strings.Contains(lower, "context length") || strings.Contains(lower, "maximum context"):
		return "context_window_exceeded"
	case status == http.StatusBadRequest || strings.Contains(lower, "invalid request") || strings.Contains(lower, "bad request"):
		return "invalid_request"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return "timeout"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "network"):
		return "network_error"
	case strings.Contains(lower, "model"):
		return "model_unavailable"
	default:
		return "request_failed"
	}
}

var (
	upstreamRelaySensitiveJSONFieldRE  = regexp.MustCompile(`(?i)("(?:password|passcode|access_token|refresh_token|id_token|bearer_token|api_key|authorization|cookie|secret|token|[a-z0-9_]*_token|[a-z0-9_]*_secret)"\s*:\s*)(?:"[^"]*"|[^,}\]]+)`)
	upstreamRelaySensitiveAssignmentRE = regexp.MustCompile(`(?i)(\b(?:password|access_token|refresh_token|id_token|bearer_token|api_key|authorization|cookie|token|secret)\b\s*[:=]\s*)(?:\[REDACTED\]|"[^"]*"|'[^']*'|[^\s,;&}\]]+)`)
	upstreamRelayAuthorizationRE       = regexp.MustCompile(`(?i)(\bAuthorization\s*:\s*)[^\r\n]+`)
	upstreamRelayCookieHeaderRE        = regexp.MustCompile(`(?i)(\bCookie\s*:\s*)[^\r\n]+`)
	upstreamRelayBearerRE              = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
)

func sanitizeUpstreamRelayError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = redactUpstreamRelayJSON(message)
	message = redactUpstreamRelayText(message)
	return truncateRelayMessage(message)
}

func redactUpstreamRelayJSON(message string) string {
	var raw any
	if err := json.Unmarshal([]byte(message), &raw); err != nil {
		return message
	}
	redacted := redactUpstreamRelayJSONValue(raw)
	out, err := json.Marshal(redacted)
	if err != nil {
		return message
	}
	return string(out)
}

func redactUpstreamRelayJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isUpstreamRelaySensitiveKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactUpstreamRelayJSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactUpstreamRelayJSONValue(child)
		}
		return out
	default:
		return value
	}
}

func isUpstreamRelaySensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"), " ", "_"))
	switch normalized {
	case "password", "passcode", "access_token", "refresh_token", "id_token", "bearer_token", "api_key", "authorization", "cookie", "secret", "token":
		return true
	}
	return strings.HasSuffix(normalized, "_token") || strings.HasSuffix(normalized, "_secret")
}

func redactUpstreamRelayText(message string) string {
	message = upstreamRelaySensitiveJSONFieldRE.ReplaceAllString(message, `${1}"[REDACTED]"`)
	message = upstreamRelayAuthorizationRE.ReplaceAllString(message, "${1}[REDACTED]")
	message = upstreamRelayCookieHeaderRE.ReplaceAllString(message, "${1}[REDACTED]")
	message = upstreamRelayBearerRE.ReplaceAllString(message, "Bearer [REDACTED]")
	message = upstreamRelaySensitiveAssignmentRE.ReplaceAllString(message, "${1}[REDACTED]")
	return message
}

func truncateRelayMessage(message string) string {
	const max = 500
	message = strings.TrimSpace(message)
	if len(message) <= max {
		return message
	}
	return message[:max]
}
