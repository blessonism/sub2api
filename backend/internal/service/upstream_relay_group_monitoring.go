package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
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

	UpstreamRelaySuggestionActionPriorityUpdate = "priority_update"
	UpstreamRelaySuggestionActionAccountPause   = "account_pause"
	UpstreamRelaySuggestionActionAccountResume  = "account_resume"

	UpstreamRelayProbeProtocolAnthropic = MonitorProviderAnthropic

	upstreamRelayDefaultPriorityStart            = 10
	upstreamRelayPriorityStep                    = 10
	upstreamRelayDefaultSyncInterval             = 480
	upstreamRelayDefaultProbeInterval            = 10
	upstreamRelayDefaultRecommendationInterval   = 60
	upstreamRelayDefaultRetryInterval            = 15
	upstreamRelayDefaultSyncLimit                = 2
	upstreamRelayDefaultProbeLimit               = 5
	upstreamRelayDefaultAutoApplySuggestionLimit = 20
	upstreamRelayDefaultAutoApplyPriorityDelta   = 100
	upstreamRelayHTTPTimeout                     = 20 * time.Second
	upstreamRelaySnapshotFreshness               = 24 * time.Hour
	upstreamRelayUsageDeltaFreshness             = 24 * time.Hour
	upstreamRelayProbeFreshness                  = 30 * time.Minute
	upstreamRelayUsageFetchPageSize              = 1000
	upstreamRelayUsageFetchMaxPages              = 50
	upstreamRelaySnapshotStatusStale             = "stale"

	UpstreamRelaySortRateAsc         = "rate_asc"
	UpstreamRelaySortSuccessRateDesc = "success_rate_desc"
	UpstreamRelaySortLatencyAsc      = "latency_asc"

	upstreamRelayConfidenceHigh    = "high"
	upstreamRelayConfidenceMedium  = "medium"
	upstreamRelayConfidenceLow     = "low"
	upstreamRelayConfidenceUnknown = "unknown"

	upstreamRelayHealthDegraded = "degraded"
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
	ErrUpstreamRelayAppliedRunDelete = infraerrors.Conflict(
		"UPSTREAM_RELAY_APPLIED_RUN_DELETE", "applied recommendation run cannot be deleted",
	)
	ErrUpstreamRelayCredentialVersionConflict = infraerrors.Conflict(
		"UPSTREAM_RELAY_CREDENTIAL_VERSION_CONFLICT", "upstream relay connector credentials were updated concurrently",
	)
)

type UpstreamRelayConnector struct {
	ID                              int64      `json:"id"`
	Name                            string     `json:"name"`
	BaseURL                         string     `json:"base_url"`
	AuthMode                        string     `json:"auth_mode"`
	Status                          string     `json:"status"`
	CredentialVersion               int64      `json:"credential_version"`
	UpstreamAccountBalance          *float64   `json:"upstream_account_balance,omitempty"`
	UpstreamAccountBalanceCheckedAt *time.Time `json:"upstream_account_balance_checked_at,omitempty"`
	BearerTokenMasked               string     `json:"bearer_token_masked,omitempty"`
	RefreshTokenMasked              string     `json:"refresh_token_masked,omitempty"`
	LoginEmailMasked                string     `json:"login_email_masked,omitempty"`
	CookieMasked                    string     `json:"cookie_masked,omitempty"`
	UserAgentMasked                 string     `json:"user_agent_masked,omitempty"`
	HasBearerToken                  bool       `json:"has_bearer_token"`
	HasRefreshToken                 bool       `json:"has_refresh_token"`
	HasLoginEmail                   bool       `json:"has_login_email"`
	HasCookie                       bool       `json:"has_cookie"`
	HasUserAgent                    bool       `json:"has_user_agent"`
	LastVerifiedAt                  *time.Time `json:"last_verified_at,omitempty"`
	LastSyncedAt                    *time.Time `json:"last_synced_at,omitempty"`
	LastError                       string     `json:"last_error,omitempty"`
	CreatedBy                       int64      `json:"created_by,omitempty"`
	CreatedAt                       time.Time  `json:"created_at"`
	UpdatedAt                       time.Time  `json:"updated_at"`
	BearerTokenEncrypted            string     `json:"-"`
	RefreshTokenEncrypted           string     `json:"-"`
	LoginEmailEncrypted             string     `json:"-"`
	CookieEncrypted                 string     `json:"-"`
	UserAgentEncrypted              string     `json:"-"`
	BearerTokenPlain                string     `json:"-"`
	RefreshTokenPlain               string     `json:"-"`
	LoginEmailPlain                 string     `json:"-"`
	CookiePlain                     string     `json:"-"`
	UserAgentPlain                  string     `json:"-"`
	BearerTokenDecryptFail          bool       `json:"-"`
	RefreshTokenDecryptFail         bool       `json:"-"`
	LoginEmailDecryptFail           bool       `json:"-"`
	CookieDecryptFail               bool       `json:"-"`
	UserAgentDecryptFail            bool       `json:"-"`
}

type UpstreamRelayConnectorInput struct {
	Name              string  `json:"name"`
	BaseURL           string  `json:"base_url"`
	AuthMode          string  `json:"auth_mode"`
	BearerToken       *string `json:"bearer_token,omitempty"`
	RefreshToken      *string `json:"refresh_token,omitempty"`
	LoginEmail        *string `json:"login_email,omitempty"`
	LoginPassword     *string `json:"login_password,omitempty"`
	Cookie            *string `json:"cookie,omitempty"`
	UserAgent         *string `json:"user_agent,omitempty"`
	SkipImmediateSync bool    `json:"-"`
}

type UpstreamRelayGroupRateSnapshot struct {
	ID                     int64      `json:"id"`
	ConnectorID            int64      `json:"connector_id"`
	UpstreamGroupID        string     `json:"upstream_group_id"`
	Name                   string     `json:"name"`
	Platform               string     `json:"platform"`
	Status                 string     `json:"status"`
	DefaultRateMultiplier  float64    `json:"default_rate_multiplier"`
	OverrideRateMultiplier *float64   `json:"override_rate_multiplier,omitempty"`
	FinalRateMultiplier    float64    `json:"final_rate_multiplier"`
	TodayActualCost        *float64   `json:"today_actual_cost,omitempty"`
	TodayTotalTokens       *int64     `json:"today_total_tokens,omitempty"`
	TodayUsageCheckedAt    *time.Time `json:"today_usage_checked_at,omitempty"`
	Source                 string     `json:"source"`
	LastSeenAt             time.Time  `json:"last_seen_at"`
	CreatedAt              time.Time  `json:"created_at,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at,omitempty"`
}

type UpstreamRelayGroupTodayUsage struct {
	ActualCost  float64
	TotalTokens int64
}

type UpstreamRelayGroupUsageHistory struct {
	ID              int64      `json:"id"`
	UsageDate       string     `json:"usage_date"`
	ConnectorID     int64      `json:"connector_id"`
	ConnectorName   string     `json:"connector_name,omitempty"`
	UpstreamGroupID string     `json:"upstream_group_id"`
	GroupName       string     `json:"group_name"`
	Platform        string     `json:"platform"`
	ActualCost      float64    `json:"actual_cost"`
	TotalTokens     int64      `json:"total_tokens"`
	CheckedAt       time.Time  `json:"checked_at"`
	FinalizedAt     *time.Time `json:"finalized_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at,omitempty"`
}

type UpstreamRelayGroupUsageHistoryUpsert struct {
	UsageDate       string
	ConnectorID     int64
	UpstreamGroupID string
	GroupName       string
	Platform        string
	ActualCost      float64
	TotalTokens     int64
	CheckedAt       time.Time
	Finalized       bool
}

type UpstreamRelayUsageHistorySummary struct {
	TotalCost       float64    `json:"total_cost"`
	TotalTokens     int64      `json:"total_tokens"`
	ConnectorCount  int        `json:"connector_count"`
	GroupCount      int        `json:"group_count"`
	LatestCheckedAt *time.Time `json:"latest_checked_at,omitempty"`
	PendingFinalize int        `json:"pending_finalize"`
	RowCount        int64      `json:"-"`
}

type UpstreamRelayCandidateUsageBinding struct {
	CandidateID        int64
	ConnectorID        int64
	AccountID          int64
	UpstreamGroupID    string
	UpstreamAPIKeyID   int64
	UpstreamAPIKeyName string
}

type UpstreamRelayMetricsIssueDetail struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	CandidateID     int64  `json:"candidate_id,omitempty"`
	AccountID       int64  `json:"account_id,omitempty"`
	UpstreamGroupID string `json:"upstream_group_id,omitempty"`
}

type upstreamRelayMetricsIssueError struct {
	issue UpstreamRelayMetricsIssueDetail
}

func (e *upstreamRelayMetricsIssueError) Error() string {
	return e.issue.Message
}

type UpstreamRelayConnectorMetricsRefreshResult struct {
	Connector        *UpstreamRelayConnector           `json:"connector"`
	Snapshots        []UpstreamRelayGroupRateSnapshot  `json:"snapshots"`
	Status           string                            `json:"status"`
	BalanceDetail    UpstreamRelayMetricsBalanceDetail `json:"balance_detail"`
	UsageDetail      UpstreamRelayMetricsUsageDetail   `json:"usage_detail"`
	BalanceAvailable bool                              `json:"balance_available"`
	BalanceError     string                            `json:"balance_error,omitempty"`
	UsageAvailable   bool                              `json:"usage_available"`
	UsageError       string                            `json:"usage_error,omitempty"`
	RefreshedAt      time.Time                         `json:"refreshed_at"`
}

type UpstreamRelayMonitoringRefreshResult struct {
	Status      string                               `json:"status"`
	Total       int                                  `json:"total"`
	Success     int                                  `json:"success"`
	Partial     int                                  `json:"partial"`
	Failed      int                                  `json:"failed"`
	Items       []UpstreamRelayMonitoringRefreshItem `json:"items"`
	RefreshedAt time.Time                            `json:"refreshed_at"`
}

type UpstreamRelayMonitoringRefreshItem struct {
	ConnectorID    int64                                       `json:"connector_id"`
	ConnectorName  string                                      `json:"connector_name,omitempty"`
	Connector      *UpstreamRelayConnector                     `json:"connector,omitempty"`
	Status         string                                      `json:"status"`
	SnapshotStatus string                                      `json:"snapshot_status"`
	SnapshotCount  int                                         `json:"snapshot_count"`
	SnapshotError  string                                      `json:"snapshot_error,omitempty"`
	Snapshots      []UpstreamRelayGroupRateSnapshot            `json:"snapshots"`
	Metrics        *UpstreamRelayConnectorMetricsRefreshResult `json:"metrics,omitempty"`
	ErrorReason    string                                      `json:"error_reason,omitempty"`
}

type UpstreamRelayMetricsBalanceDetail struct {
	Status    string     `json:"status"`
	Value     *float64   `json:"value,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type UpstreamRelayMetricsUsageDetail struct {
	Status        string                                   `json:"status"`
	TotalGroups   int                                      `json:"total_groups"`
	UpdatedGroups int                                      `json:"updated_groups"`
	MissingGroups []UpstreamRelayMetricsMissingGroupDetail `json:"missing_groups"`
	Error         string                                   `json:"error,omitempty"`
	Issue         *UpstreamRelayMetricsIssueDetail         `json:"issue,omitempty"`
	Issues        []UpstreamRelayMetricsIssueDetail        `json:"issues,omitempty"`
	CheckedAt     *time.Time                               `json:"checked_at,omitempty"`
}

type UpstreamRelayMetricsMissingGroupDetail struct {
	UpstreamGroupID string `json:"upstream_group_id"`
	Name            string `json:"name,omitempty"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
}

const (
	upstreamRelayMetricsRefreshStatusSuccess = "success"
	upstreamRelayMetricsRefreshStatusPartial = "partial"
	upstreamRelayMetricsRefreshStatusFailed  = "failed"
	upstreamRelayMetricsRefreshStatusSkipped = "skipped"

	upstreamRelayMetricsIssueNoCandidateBindings    = "no_candidate_bindings"
	upstreamRelayMetricsIssueMissingAPIKeyBinding   = "missing_upstream_api_key_binding"
	upstreamRelayMetricsIssueAPIKeyNotVisible       = "upstream_api_key_not_visible"
	upstreamRelayMetricsIssueAPIKeyGroupUnavailable = "upstream_api_key_group_unavailable"
	upstreamRelayMetricsIssueUpstreamUsageRequest   = "upstream_usage_request_failed"
	upstreamRelayMetricsIssueCandidateBindingsLoad  = "candidate_bindings_load_failed"
	upstreamRelayMetricsIssueUsageRefreshFailed     = "usage_refresh_failed"
	upstreamRelayMetricsIssueUsageRefreshAborted    = "usage_refresh_aborted"

	UpstreamRelaySnapshotChangeAdded       = "added"
	UpstreamRelaySnapshotChangeRemoved     = "removed"
	UpstreamRelaySnapshotChangeRateChanged = "rate_changed"
)

type UpstreamRelayGroupRateSnapshotChange struct {
	ID                     int64     `json:"id"`
	ConnectorID            int64     `json:"connector_id"`
	ConnectorName          string    `json:"connector_name,omitempty"`
	UpstreamGroupID        string    `json:"upstream_group_id"`
	GroupName              string    `json:"group_name"`
	Platform               string    `json:"platform"`
	ChangeType             string    `json:"change_type"`
	OldFinalRateMultiplier *float64  `json:"old_final_rate_multiplier,omitempty"`
	NewFinalRateMultiplier *float64  `json:"new_final_rate_multiplier,omitempty"`
	OldStatus              string    `json:"old_status"`
	NewStatus              string    `json:"new_status"`
	Source                 string    `json:"source"`
	ChangedAt              time.Time `json:"changed_at"`
}

type UpstreamRelayCandidate struct {
	ID                   int64                           `json:"id"`
	ConnectorID          int64                           `json:"connector_id"`
	ConnectorName        string                          `json:"connector_name,omitempty"`
	ConnectorStatus      string                          `json:"connector_status,omitempty"`
	TodayActualCost      *float64                        `json:"today_actual_cost,omitempty"`
	TodayTotalTokens     *int64                          `json:"today_total_tokens,omitempty"`
	TodayUsageCheckedAt  *time.Time                      `json:"today_usage_checked_at,omitempty"`
	AccountID            int64                           `json:"account_id"`
	AccountName          string                          `json:"account_name,omitempty"`
	AccountPlatform      string                          `json:"account_platform,omitempty"`
	AccountSchedulable   bool                            `json:"account_schedulable"`
	AccountGateActive    bool                            `json:"account_gate_active,omitempty"`
	UpstreamGroupID      string                          `json:"upstream_group_id"`
	UpstreamGroupName    string                          `json:"upstream_group_name,omitempty"`
	UpstreamAPIKeyID     *int64                          `json:"upstream_api_key_id,omitempty"`
	UpstreamAPIKeyName   string                          `json:"upstream_api_key_name,omitempty"`
	UpstreamAPIKeyMasked string                          `json:"upstream_api_key_masked,omitempty"`
	ProbeModel           string                          `json:"probe_model"`
	ProbeProtocol        string                          `json:"probe_protocol"`
	CurrentPriority      *int                            `json:"current_priority,omitempty"`
	Enabled              bool                            `json:"enabled"`
	Notes                string                          `json:"notes"`
	LastProbeResultID    *int64                          `json:"last_probe_result_id,omitempty"`
	LatestProbe          *UpstreamRelayProbeResult       `json:"latest_probe,omitempty"`
	LatestSnapshot       *UpstreamRelayGroupRateSnapshot `json:"latest_snapshot,omitempty"`
	Health               *UpstreamRelayCandidateHealth   `json:"health,omitempty"`
	LatestUsageDelta     *UpstreamRelayUsageDeltaSample  `json:"latest_usage_delta,omitempty"`
	CreatedBy            int64                           `json:"created_by,omitempty"`
	CreatedAt            time.Time                       `json:"created_at"`
	UpdatedAt            time.Time                       `json:"updated_at"`
}

type UpstreamRelayCandidateInput struct {
	ConnectorID          int64  `json:"connector_id"`
	AccountID            int64  `json:"account_id"`
	UpstreamGroupID      string `json:"upstream_group_id"`
	UpstreamAPIKeyID     *int64 `json:"upstream_api_key_id,omitempty"`
	UpstreamAPIKeyName   string `json:"upstream_api_key_name"`
	UpstreamAPIKeyMasked string `json:"upstream_api_key_masked"`
	ProbeModel           string `json:"probe_model"`
	ProbeProtocol        string `json:"probe_protocol"`
	Enabled              *bool  `json:"enabled,omitempty"`
	Notes                string `json:"notes"`
}

type UpstreamRelayAPIKeyOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name,omitempty"`
	MaskedKey string `json:"masked_key,omitempty"`
	GroupID   string `json:"group_id,omitempty"`
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
	CalculatedAt         time.Time  `json:"calculated_at,omitempty"`
	Stale                bool       `json:"stale"`
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
	Closed          bool                                    `json:"closed"`
	ClosedBy        *int64                                  `json:"closed_by,omitempty"`
	ClosedAt        *time.Time                              `json:"closed_at,omitempty"`
	ErrorMessage    string                                  `json:"error_message,omitempty"`
	CreatedBy       int64                                   `json:"created_by,omitempty"`
	CreatedAt       time.Time                               `json:"created_at"`
	Suggestions     []UpstreamRelayRecommendationSuggestion `json:"suggestions,omitempty"`
}

type UpstreamRelayRecommendationRunListFilters struct {
	HasSuggestions *bool
}

type UpstreamRelayRecommendationSuggestion struct {
	ID                  int64      `json:"id,omitempty"`
	RunID               int64      `json:"run_id,omitempty"`
	ActionType          string     `json:"action_type"`
	CandidateID         int64      `json:"candidate_id"`
	ConnectorID         int64      `json:"connector_id"`
	ConnectorName       string     `json:"connector_name,omitempty"`
	AccountID           int64      `json:"account_id"`
	AccountName         string     `json:"account_name,omitempty"`
	UpstreamGroupID     string     `json:"upstream_group_id"`
	UpstreamGroupName   string     `json:"upstream_group_name,omitempty"`
	OldPriority         *int       `json:"old_priority,omitempty"`
	NewPriority         *int       `json:"new_priority,omitempty"`
	OldSchedulable      *bool      `json:"old_schedulable,omitempty"`
	NewSchedulable      *bool      `json:"new_schedulable,omitempty"`
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

type UpstreamRelayRecommendationPolicy struct {
	SnapshotFreshnessMinutes          int       `json:"snapshot_freshness_minutes"`
	UsageDeltaFreshnessMinutes        int       `json:"usage_delta_freshness_minutes"`
	ProbeFreshnessMinutes             int       `json:"probe_freshness_minutes"`
	MinSuccessRate                    float64   `json:"min_success_rate"`
	MinSampleSize                     int       `json:"min_sample_size"`
	ExcludeConsecutiveFailures        bool      `json:"exclude_consecutive_failures"`
	PriorityStart                     int       `json:"priority_start"`
	PriorityStep                      int       `json:"priority_step"`
	SortFields                        []string  `json:"sort_fields"`
	PauseRateGapEnabled               bool      `json:"pause_rate_gap_enabled"`
	PauseRateGapThreshold             float64   `json:"pause_rate_gap_threshold"`
	PauseConsecutiveFailuresEnabled   bool      `json:"pause_consecutive_failures_enabled"`
	PauseConsecutiveFailuresThreshold int       `json:"pause_consecutive_failures_threshold"`
	PauseSuccessRateEnabled           bool      `json:"pause_success_rate_enabled"`
	UpdatedBy                         int64     `json:"updated_by,omitempty"`
	CreatedAt                         time.Time `json:"created_at,omitempty"`
	UpdatedAt                         time.Time `json:"updated_at,omitempty"`
}

type UpstreamRelayMonitoringPolicy struct {
	AutoSyncEnabled                 bool      `json:"auto_sync_enabled"`
	SyncIntervalMinutes             int       `json:"sync_interval_minutes"`
	AutoProbeEnabled                bool      `json:"auto_probe_enabled"`
	ProbeIntervalMinutes            int       `json:"probe_interval_minutes"`
	AutoRecommendationEnabled       bool      `json:"auto_recommendation_enabled"`
	RecommendationIntervalMinutes   int       `json:"recommendation_interval_minutes"`
	AutoApplyRecommendationsEnabled bool      `json:"auto_apply_recommendations_enabled"`
	MaxAutoApplySuggestions         int       `json:"max_auto_apply_suggestions"`
	MaxAutoApplyPriorityDelta       int       `json:"max_auto_apply_priority_delta"`
	MinAutoApplyConfidence          string    `json:"min_auto_apply_confidence"`
	AllowAutoApplyDegradedHealth    bool      `json:"allow_auto_apply_degraded_health"`
	FailureRetryIntervalMinutes     int       `json:"failure_retry_interval_minutes"`
	SyncConcurrency                 int       `json:"sync_concurrency"`
	ProbeConcurrency                int       `json:"probe_concurrency"`
	SnapshotStaleAfterMinutes       int       `json:"snapshot_stale_after_minutes"`
	UsageDeltaStaleAfterMinutes     int       `json:"usage_delta_stale_after_minutes"`
	ProbeStaleAfterMinutes          int       `json:"probe_stale_after_minutes"`
	UpdatedBy                       int64     `json:"updated_by,omitempty"`
	CreatedAt                       time.Time `json:"created_at,omitempty"`
	UpdatedAt                       time.Time `json:"updated_at,omitempty"`
}

type UpstreamRelayRecommendationAutomationResult struct {
	Run           *UpstreamRelayRecommendationRun `json:"run,omitempty"`
	Applied       bool                            `json:"applied"`
	SkippedReason string                          `json:"skipped_reason,omitempty"`
}

type UpstreamRelayBulkOperationResult struct {
	Total   int                              `json:"total"`
	Success int                              `json:"success"`
	Failed  int                              `json:"failed"`
	Items   []UpstreamRelayBulkOperationItem `json:"items"`
}

type UpstreamRelayBulkOperationItem struct {
	ID            int64      `json:"id"`
	ConnectorID   int64      `json:"connector_id,omitempty"`
	ConnectorName string     `json:"connector_name,omitempty"`
	CandidateID   int64      `json:"candidate_id,omitempty"`
	AccountID     int64      `json:"account_id,omitempty"`
	AccountName   string     `json:"account_name,omitempty"`
	Success       bool       `json:"success"`
	Count         int        `json:"count,omitempty"`
	ErrorReason   string     `json:"error_reason,omitempty"`
	ProbeResultID *int64     `json:"probe_result_id,omitempty"`
	LatencyMs     *int       `json:"latency_ms,omitempty"`
	HTTPStatus    *int       `json:"http_status,omitempty"`
	ErrorClass    string     `json:"error_class,omitempty"`
	ProbedAt      *time.Time `json:"probed_at,omitempty"`
}

type UpstreamRelayRecommendationPreview struct {
	Policy          UpstreamRelayRecommendationPolicy       `json:"policy"`
	TotalCandidates int                                     `json:"total_candidates"`
	SuggestionCount int                                     `json:"suggestion_count"`
	ExcludedCount   int                                     `json:"excluded_count"`
	Suggestions     []UpstreamRelayRecommendationSuggestion `json:"suggestions"`
	Exclusions      []UpstreamRelayRecommendationExclusion  `json:"exclusions"`
}

type UpstreamRelayRecommendationExclusion struct {
	CandidateID         int64    `json:"candidate_id"`
	ConnectorID         int64    `json:"connector_id"`
	ConnectorName       string   `json:"connector_name,omitempty"`
	AccountID           int64    `json:"account_id"`
	AccountName         string   `json:"account_name,omitempty"`
	UpstreamGroupID     string   `json:"upstream_group_id"`
	UpstreamGroupName   string   `json:"upstream_group_name,omitempty"`
	OldPriority         *int     `json:"old_priority,omitempty"`
	ExpectedPriority    *int     `json:"expected_priority,omitempty"`
	FinalRateMultiplier *float64 `json:"final_rate_multiplier,omitempty"`
	RateSource          string   `json:"rate_source,omitempty"`
	Confidence          string   `json:"confidence,omitempty"`
	HealthStatus        string   `json:"health_status"`
	HealthSummary       string   `json:"health_summary"`
	ReasonCode          string   `json:"reason_code"`
	Reason              string   `json:"reason"`
}

type UpstreamRelayConnectorListFilters struct {
	Status string
	Search string
}

type UpstreamRelayCandidateListFilters struct {
	ConnectorID int64
	Enabled     *bool
}

type UpstreamRelaySnapshotChangeListFilters struct {
	ConnectorID int64
	ChangeType  string
	Search      string
}

type UpstreamRelayUsageHistoryListFilters struct {
	StartDate        string
	EndDate          string
	ConnectorID      int64
	UpstreamGroupID  string
	Search           string
	IncludeZeroUsage bool
}

type UpstreamRelayRepository interface {
	ListConnectors(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayConnectorListFilters) ([]UpstreamRelayConnector, *pagination.PaginationResult, error)
	GetConnector(ctx context.Context, id int64) (*UpstreamRelayConnector, error)
	CreateConnector(ctx context.Context, connector *UpstreamRelayConnector) (*UpstreamRelayConnector, error)
	UpdateConnector(ctx context.Context, connector *UpstreamRelayConnector, credentialsUpdated bool) (*UpstreamRelayConnector, error)
	UpdateConnectorTokens(ctx context.Context, connectorID int64, expectedCredentialVersion int64, bearerTokenEncrypted, refreshTokenEncrypted string) (*UpstreamRelayConnector, error)
	SoftDeleteConnector(ctx context.Context, id int64) error
	UpsertSnapshots(ctx context.Context, connectorID int64, snapshots []UpstreamRelayGroupRateSnapshot) error
	ListSnapshots(ctx context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error)
	ListSnapshotChanges(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelaySnapshotChangeListFilters) ([]UpstreamRelayGroupRateSnapshotChange, *pagination.PaginationResult, error)
	MarkConnectorSync(ctx context.Context, connectorID int64, status string, errMessage string) error
	UpdateConnectorAccountBalance(ctx context.Context, connectorID int64, balance *float64, checkedAt *time.Time) error
	UpdateSnapshotTodayUsage(ctx context.Context, connectorID int64, usageByGroup map[string]UpstreamRelayGroupTodayUsage, checkedAt *time.Time, complete bool) error
	UpsertUsageHistory(ctx context.Context, rows []UpstreamRelayGroupUsageHistoryUpsert) error
	ListUsageHistory(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayUsageHistoryListFilters) ([]UpstreamRelayGroupUsageHistory, *pagination.PaginationResult, error)
	SummarizeUsageHistory(ctx context.Context, filters UpstreamRelayUsageHistoryListFilters) (*UpstreamRelayUsageHistorySummary, error)
	ListCandidateUsageBindings(ctx context.Context, connectorID int64) ([]UpstreamRelayCandidateUsageBinding, error)

	ListCandidates(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayCandidateListFilters) ([]UpstreamRelayCandidate, *pagination.PaginationResult, error)
	GetCandidate(ctx context.Context, id int64) (*UpstreamRelayCandidate, error)
	CreateCandidate(ctx context.Context, candidate *UpstreamRelayCandidate) (*UpstreamRelayCandidate, error)
	UpdateCandidate(ctx context.Context, candidate *UpstreamRelayCandidate) (*UpstreamRelayCandidate, error)
	SoftDeleteCandidate(ctx context.Context, id int64) error
	InsertProbeResult(ctx context.Context, result UpstreamRelayProbeResult) (*UpstreamRelayProbeResult, error)
	InsertUsageDeltaSample(ctx context.Context, sample UpstreamRelayUsageDeltaSample) (*UpstreamRelayUsageDeltaSample, error)

	ListRecommendationInputs(ctx context.Context) ([]UpstreamRelayCandidate, error)
	GetMonitoringPolicy(ctx context.Context) (*UpstreamRelayMonitoringPolicy, error)
	UpsertMonitoringPolicy(ctx context.Context, policy UpstreamRelayMonitoringPolicy, operatorID int64) (*UpstreamRelayMonitoringPolicy, error)
	GetRecommendationPolicy(ctx context.Context) (*UpstreamRelayRecommendationPolicy, error)
	UpsertRecommendationPolicy(ctx context.Context, policy UpstreamRelayRecommendationPolicy, operatorID int64) (*UpstreamRelayRecommendationPolicy, error)
	CreateRecommendationRun(ctx context.Context, run UpstreamRelayRecommendationRun, suggestions []UpstreamRelayRecommendationSuggestion) (*UpstreamRelayRecommendationRun, error)
	GetRecommendationRun(ctx context.Context, id int64) (*UpstreamRelayRecommendationRun, error)
	ListRecommendationRuns(ctx context.Context, params pagination.PaginationParams, filters UpstreamRelayRecommendationRunListFilters) ([]UpstreamRelayRecommendationRun, *pagination.PaginationResult, error)
	ApplyRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error)
	CloseRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error)
	RestoreRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error)
	DeleteRecommendationRun(ctx context.Context, runID int64) error
}

type UpstreamRelayGroupMonitoringService struct {
	repo               UpstreamRelayRepository
	accountRepo        AccountRepository
	accountTestService *AccountTestService
	encryptor          SecretEncryptor
	httpClient         *http.Client
}

func NewUpstreamRelayGroupMonitoringService(repo UpstreamRelayRepository, accountRepo AccountRepository, encryptor SecretEncryptor) *UpstreamRelayGroupMonitoringService {
	return &UpstreamRelayGroupMonitoringService{
		repo:        repo,
		accountRepo: accountRepo,
		encryptor:   encryptor,
		httpClient:  &http.Client{Timeout: upstreamRelayHTTPTimeout},
	}
}

// ProvideUpstreamRelayGroupMonitoringService 注入账号测试服务，让监控探测复用账号管理的测试路径。
func ProvideUpstreamRelayGroupMonitoringService(repo UpstreamRelayRepository, accountRepo AccountRepository, accountTestService *AccountTestService, encryptor SecretEncryptor) *UpstreamRelayGroupMonitoringService {
	svc := NewUpstreamRelayGroupMonitoringService(repo, accountRepo, encryptor)
	svc.accountTestService = accountTestService
	return svc
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
		if err := s.syncConnectorAfterSave(ctx, created.ID); err != nil {
			return nil, err
		}
		return s.GetConnector(ctx, created.ID)
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
		if err := s.syncConnectorAfterSave(ctx, updated.ID); err != nil {
			return nil, err
		}
	}
	return s.GetConnector(ctx, updated.ID)
}

func (s *UpstreamRelayGroupMonitoringService) syncConnectorAfterSave(ctx context.Context, connectorID int64) error {
	if _, err := s.SyncConnector(ctx, connectorID); err != nil {
		if _, getErr := s.repo.GetConnector(ctx, connectorID); getErr != nil {
			return err
		}
	}
	return nil
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
	if err := s.persistKnownUsageHistory(ctx, id, snapshots); err != nil {
		return nil, err
	}
	s.finalizePendingUsageForConnector(ctx, id)
	if err := s.refreshConnectorAccountBalance(ctx, connector); err != nil {
		return nil, err
	}
	if err := s.repo.MarkConnectorSync(ctx, id, UpstreamRelayConnectorStatusActive, ""); err != nil {
		return nil, err
	}
	return s.repo.ListSnapshots(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) SyncAllConnectors(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
	policy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return nil, err
	}
	connectors, err := s.listAllConnectors(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpstreamRelayBulkOperationResult{
		Total: len(connectors),
		Items: make([]UpstreamRelayBulkOperationItem, len(connectors)),
	}
	limit := positiveOrDefault(policy.SyncConcurrency, upstreamRelayDefaultSyncLimit)
	s.runLimited(len(connectors), limit, func(index int) {
		connector := connectors[index]
		item := UpstreamRelayBulkOperationItem{
			ID:            connector.ID,
			ConnectorID:   connector.ID,
			ConnectorName: connector.Name,
		}
		snapshots, err := s.SyncConnector(ctx, connector.ID)
		if err != nil {
			item.ErrorReason = sanitizeUpstreamRelayError(err.Error())
		} else {
			item.Success = true
			item.Count = len(snapshots)
		}
		result.Items[index] = item
	})
	result.summarize()
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) RefreshConnectorMetrics(ctx context.Context, id int64) (*UpstreamRelayConnectorMetricsRefreshResult, error) {
	connector, err := s.repo.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.decryptConnector(connector); err != nil {
		_ = s.repo.MarkConnectorSync(ctx, id, UpstreamRelayConnectorStatusNeedsReauth, err.Error())
		return nil, err
	}

	result := &UpstreamRelayConnectorMetricsRefreshResult{RefreshedAt: time.Now()}
	bindings, bindingsErr := s.repo.ListCandidateUsageBindings(ctx, connector.ID)
	balance, balanceErr := s.fetchUpstreamAccountBalance(ctx, connector)
	balanceCheckedAt := time.Now()
	if balanceErr != nil {
		result.BalanceError = sanitizeUpstreamRelayError(balanceErr.Error())
		if err := s.repo.UpdateConnectorAccountBalance(ctx, connector.ID, nil, nil); err != nil {
			return nil, err
		}
	} else {
		result.BalanceAvailable = true
		if err := s.repo.UpdateConnectorAccountBalance(ctx, connector.ID, balance, &balanceCheckedAt); err != nil {
			return nil, err
		}
	}
	result.BalanceDetail = buildUpstreamRelayMetricsBalanceDetail(result.BalanceAvailable, balance, &balanceCheckedAt, result.BalanceError)

	usageByGroup, usageCheckedAt, usageIssues, usageErr := s.fetchUpstreamGroupTodayUsage(ctx, connector, result.RefreshedAt)
	if usageErr != nil {
		result.UsageError = sanitizeUpstreamRelayError(usageErr.Error())
	} else {
		result.UsageAvailable = len(usageIssues) == 0
		if len(usageIssues) > 0 {
			result.UsageError = usageIssues[0].Message
		}
		complete := len(usageIssues) == 0
		if err := s.repo.UpdateSnapshotTodayUsage(ctx, connector.ID, usageByGroup, usageCheckedAt, complete); err != nil {
			return nil, err
		}
		if err := s.persistKnownUsageHistoryFromRefresh(ctx, connector.ID, usageByGroup, usageCheckedAt, complete); err != nil {
			return nil, err
		}
	}
	s.finalizePendingUsageForConnector(ctx, connector.ID)

	refreshedConnector, err := s.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.repo.ListSnapshots(ctx, id)
	if err != nil {
		return nil, err
	}
	result.Connector = refreshedConnector
	result.Snapshots = snapshots
	if usageErr != nil {
		usageIssues = []UpstreamRelayMetricsIssueDetail{*upstreamRelayMetricsIssueFromError(usageErr)}
	}
	result.UsageDetail = buildUpstreamRelayMetricsUsageDetail(bindings, bindingsErr, snapshots, usageByGroup, usageCheckedAt, result.UsageError, usageIssues)
	result.Status = buildUpstreamRelayMetricsRefreshStatus(result.BalanceDetail.Status, result.UsageDetail.Status)
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) RefreshMonitoringData(ctx context.Context) (*UpstreamRelayMonitoringRefreshResult, error) {
	policy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return nil, err
	}
	connectors, err := s.listAllConnectors(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpstreamRelayMonitoringRefreshResult{
		Total:       len(connectors),
		Items:       make([]UpstreamRelayMonitoringRefreshItem, len(connectors)),
		RefreshedAt: time.Now(),
	}
	limit := positiveOrDefault(policy.SyncConcurrency, upstreamRelayDefaultSyncLimit)
	s.runLimited(len(connectors), limit, func(index int) {
		result.Items[index] = s.refreshMonitoringConnector(ctx, connectors[index])
	})
	result.summarize()
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) refreshMonitoringConnector(ctx context.Context, connector UpstreamRelayConnector) UpstreamRelayMonitoringRefreshItem {
	item := UpstreamRelayMonitoringRefreshItem{
		ConnectorID:    connector.ID,
		ConnectorName:  connector.Name,
		Connector:      &connector,
		Status:         upstreamRelayMetricsRefreshStatusFailed,
		SnapshotStatus: upstreamRelayMetricsRefreshStatusFailed,
		Snapshots:      []UpstreamRelayGroupRateSnapshot{},
	}

	snapshots, syncErr := s.SyncConnector(ctx, connector.ID)
	if syncErr != nil {
		item.SnapshotError = sanitizeUpstreamRelayError(syncErr.Error())
	} else {
		item.SnapshotStatus = upstreamRelayMetricsRefreshStatusSuccess
		item.SnapshotCount = len(snapshots)
		item.Snapshots = snapshots
	}

	metrics, metricsErr := s.RefreshConnectorMetrics(ctx, connector.ID)
	if metricsErr == nil {
		item.Metrics = metrics
		if metrics.Connector != nil {
			item.Connector = metrics.Connector
			item.ConnectorName = metrics.Connector.Name
		}
		if len(metrics.Snapshots) > 0 {
			item.Snapshots = metrics.Snapshots
		}
	}

	item.Status = buildUpstreamRelayMonitoringRefreshItemStatus(item.SnapshotStatus, item.Metrics)
	item.ErrorReason = firstNonEmpty(item.SnapshotError, monitoringMetricsError(metricsErr, item.Metrics))
	return item
}

func buildUpstreamRelayMonitoringRefreshItemStatus(snapshotStatus string, metrics *UpstreamRelayConnectorMetricsRefreshResult) string {
	metricsStatus := upstreamRelayMetricsRefreshStatusFailed
	if metrics != nil {
		metricsStatus = metrics.Status
	}
	if snapshotStatus == upstreamRelayMetricsRefreshStatusSuccess && metricsStatus == upstreamRelayMetricsRefreshStatusSuccess {
		return upstreamRelayMetricsRefreshStatusSuccess
	}
	if snapshotStatus == upstreamRelayMetricsRefreshStatusFailed && metricsStatus == upstreamRelayMetricsRefreshStatusFailed {
		return upstreamRelayMetricsRefreshStatusFailed
	}
	return upstreamRelayMetricsRefreshStatusPartial
}

func monitoringMetricsError(err error, metrics *UpstreamRelayConnectorMetricsRefreshResult) string {
	if err != nil {
		return sanitizeUpstreamRelayError(err.Error())
	}
	if metrics == nil {
		return ""
	}
	return firstNonEmpty(metrics.BalanceError, metrics.UsageError)
}

func buildUpstreamRelayMetricsBalanceDetail(available bool, value *float64, checkedAt *time.Time, errText string) UpstreamRelayMetricsBalanceDetail {
	if !available {
		return UpstreamRelayMetricsBalanceDetail{
			Status: upstreamRelayMetricsRefreshStatusFailed,
			Error:  errText,
		}
	}
	return UpstreamRelayMetricsBalanceDetail{
		Status:    upstreamRelayMetricsRefreshStatusSuccess,
		Value:     value,
		CheckedAt: checkedAt,
	}
}

func buildUpstreamRelayMetricsUsageDetail(
	bindings []UpstreamRelayCandidateUsageBinding,
	bindingsErr error,
	snapshots []UpstreamRelayGroupRateSnapshot,
	usageByGroup map[string]UpstreamRelayGroupTodayUsage,
	checkedAt *time.Time,
	errText string,
	issues []UpstreamRelayMetricsIssueDetail,
) UpstreamRelayMetricsUsageDetail {
	if bindingsErr != nil {
		message := sanitizeUpstreamRelayError(bindingsErr.Error())
		issue := UpstreamRelayMetricsIssueDetail{
			Code:    upstreamRelayMetricsIssueCandidateBindingsLoad,
			Message: message,
		}
		return UpstreamRelayMetricsUsageDetail{
			Status: upstreamRelayMetricsRefreshStatusFailed,
			Error:  message,
			Issue:  &issue,
			Issues: []UpstreamRelayMetricsIssueDetail{issue},
		}
	}

	groupIDs := distinctUpstreamRelayUsageGroups(bindings, usageByGroup, issues)
	detail := UpstreamRelayMetricsUsageDetail{
		TotalGroups: len(groupIDs),
		CheckedAt:   checkedAt,
		Issues:      issues,
	}
	if len(issues) > 0 {
		detail.Issue = &detail.Issues[0]
		detail.Error = errText
	}
	if len(groupIDs) == 0 {
		detail.Status = upstreamRelayMetricsRefreshStatusSkipped
		return detail
	}

	snapshotNames := map[string]string{}
	for _, snapshot := range snapshots {
		if snapshot.UpstreamGroupID == "" {
			continue
		}
		snapshotNames[snapshot.UpstreamGroupID] = snapshot.Name
	}

	if len(snapshots) == 0 {
		detail.Status = upstreamRelayMetricsRefreshStatusSkipped
		detail.Error = errText
		for _, groupID := range groupIDs {
			detail.MissingGroups = append(detail.MissingGroups, UpstreamRelayMetricsMissingGroupDetail{
				UpstreamGroupID: groupID,
				Reason:          "no_snapshot",
				Message:         "full connector sync is required before today usage can be refreshed",
			})
		}
		return detail
	}

	issueByGroup := make(map[string]UpstreamRelayMetricsIssueDetail, len(issues))
	var globalIssue *UpstreamRelayMetricsIssueDetail
	for i := range issues {
		issue := issues[i]
		if issue.UpstreamGroupID == "" {
			if globalIssue == nil {
				globalIssue = &issues[i]
			}
			continue
		}
		if _, exists := issueByGroup[issue.UpstreamGroupID]; !exists {
			issueByGroup[issue.UpstreamGroupID] = issue
		}
	}

	for _, groupID := range groupIDs {
		if _, ok := snapshotNames[groupID]; !ok {
			detail.MissingGroups = append(detail.MissingGroups, UpstreamRelayMetricsMissingGroupDetail{
				UpstreamGroupID: groupID,
				Reason:          "no_snapshot",
				Message:         "full connector sync is required before today usage can be refreshed",
			})
			continue
		}
		if issue, exists := issueByGroup[groupID]; exists {
			detail.MissingGroups = append(detail.MissingGroups, UpstreamRelayMetricsMissingGroupDetail{
				UpstreamGroupID: groupID,
				Name:            snapshotNames[groupID],
				Reason:          issue.Code,
				Message:         issue.Message,
			})
			continue
		}
		if _, known := usageByGroup[groupID]; !known {
			reason := upstreamRelayMetricsIssueUsageRefreshAborted
			message := errText
			if globalIssue != nil {
				reason = globalIssue.Code
				message = globalIssue.Message
			}
			if message == "" {
				message = "usage was not refreshed for this group"
			}
			detail.MissingGroups = append(detail.MissingGroups, UpstreamRelayMetricsMissingGroupDetail{
				UpstreamGroupID: groupID,
				Name:            snapshotNames[groupID],
				Reason:          reason,
				Message:         message,
			})
			continue
		}
		detail.UpdatedGroups++
	}
	if len(detail.MissingGroups) > 0 {
		if detail.UpdatedGroups > 0 {
			detail.Status = upstreamRelayMetricsRefreshStatusPartial
		} else {
			detail.Status = upstreamRelayMetricsRefreshStatusFailed
		}
		return detail
	}
	detail.Status = upstreamRelayMetricsRefreshStatusSuccess
	return detail
}

func upstreamRelayMetricsIssueFromError(err error) *UpstreamRelayMetricsIssueDetail {
	if err == nil {
		return nil
	}
	var issueErr *upstreamRelayMetricsIssueError
	if errors.As(err, &issueErr) {
		issue := issueErr.issue
		issue.Message = sanitizeUpstreamRelayError(issue.Message)
		return &issue
	}
	return &UpstreamRelayMetricsIssueDetail{
		Code:    upstreamRelayMetricsIssueUsageRefreshFailed,
		Message: sanitizeUpstreamRelayError(err.Error()),
	}
}

func distinctUpstreamRelayUsageGroups(bindings []UpstreamRelayCandidateUsageBinding, usageByGroup map[string]UpstreamRelayGroupTodayUsage, issues []UpstreamRelayMetricsIssueDetail) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(groupID string) {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			return
		}
		if _, ok := seen[groupID]; ok {
			return
		}
		seen[groupID] = struct{}{}
		out = append(out, groupID)
	}
	for groupID := range usageByGroup {
		add(groupID)
	}
	for _, issue := range issues {
		add(issue.UpstreamGroupID)
	}
	if len(out) == 0 {
		for _, binding := range bindings {
			add(binding.UpstreamGroupID)
		}
	}
	sort.Strings(out)
	return out
}

func buildUpstreamRelayMetricsRefreshStatus(balanceStatus, usageStatus string) string {
	if balanceStatus == upstreamRelayMetricsRefreshStatusSuccess && usageStatus == upstreamRelayMetricsRefreshStatusSuccess {
		return upstreamRelayMetricsRefreshStatusSuccess
	}
	if balanceStatus == upstreamRelayMetricsRefreshStatusFailed && usageStatus == upstreamRelayMetricsRefreshStatusFailed {
		return upstreamRelayMetricsRefreshStatusFailed
	}
	return upstreamRelayMetricsRefreshStatusPartial
}

func (s *UpstreamRelayGroupMonitoringService) ListSnapshots(ctx context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error) {
	return s.repo.ListSnapshots(ctx, connectorID)
}

func (s *UpstreamRelayGroupMonitoringService) ListSnapshotChanges(ctx context.Context, page, pageSize int, filters UpstreamRelaySnapshotChangeListFilters) ([]UpstreamRelayGroupRateSnapshotChange, *pagination.PaginationResult, error) {
	return s.repo.ListSnapshotChanges(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
}

func (s *UpstreamRelayGroupMonitoringService) ListUsageHistory(ctx context.Context, page, pageSize int, filters UpstreamRelayUsageHistoryListFilters) ([]UpstreamRelayGroupUsageHistory, *UpstreamRelayUsageHistorySummary, *pagination.PaginationResult, error) {
	if strings.TrimSpace(filters.EndDate) == "" {
		filters.EndDate = upstreamRelayUsageDate(time.Now())
	}
	if strings.TrimSpace(filters.StartDate) == "" {
		filters.StartDate = filters.EndDate
	}
	if !isRelayUsageDate(filters.StartDate) || !isRelayUsageDate(filters.EndDate) {
		return nil, nil, nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_USAGE_DATE", "usage date must use YYYY-MM-DD")
	}
	if filters.StartDate > filters.EndDate {
		return nil, nil, nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_USAGE_DATE_RANGE", "start_date must be before or equal to end_date")
	}
	items, pageResult, err := s.repo.ListUsageHistory(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
	if err != nil {
		return nil, nil, nil, err
	}
	summary, err := s.repo.SummarizeUsageHistory(ctx, filters)
	if err != nil {
		return nil, nil, nil, err
	}
	return items, summary, pageResult, nil
}

func (s *UpstreamRelayGroupMonitoringService) ListCandidates(ctx context.Context, page, pageSize int, filters UpstreamRelayCandidateListFilters) ([]UpstreamRelayCandidate, *pagination.PaginationResult, error) {
	items, pageResult, err := s.repo.ListCandidates(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
	if err != nil {
		return nil, nil, err
	}
	s.decorateCandidateCurrentGroups(ctx, items, false)
	if err := s.decorateCandidateHealthFreshness(ctx, items); err != nil {
		return nil, nil, err
	}
	sanitizeUpstreamRelayCandidateAPIKeyDisplays(items)
	return items, pageResult, nil
}

func (s *UpstreamRelayGroupMonitoringService) decorateCandidateCurrentGroups(ctx context.Context, items []UpstreamRelayCandidate, clearUnresolved bool) {
	indicesByConnector := map[int64][]int{}
	for i := range items {
		if items[i].ConnectorID <= 0 {
			continue
		}
		if items[i].UpstreamAPIKeyID == nil || *items[i].UpstreamAPIKeyID <= 0 {
			continue
		}
		indicesByConnector[items[i].ConnectorID] = append(indicesByConnector[items[i].ConnectorID], i)
	}
	for connectorID, indices := range indicesByConnector {
		connector, err := s.repo.GetConnector(ctx, connectorID)
		if err == nil {
			err = s.decryptConnector(connector)
		}
		var apiKeys []UpstreamRelayAPIKeyOption
		if err == nil {
			apiKeys, err = s.fetchUpstreamAPIKeyOptions(ctx, connector)
		}
		var snapshots []UpstreamRelayGroupRateSnapshot
		if err == nil {
			snapshots, err = s.repo.ListSnapshots(ctx, connectorID)
		}
		if err != nil {
			if clearUnresolved {
				for _, index := range indices {
					items[index].TodayActualCost = nil
					items[index].TodayTotalTokens = nil
					items[index].TodayUsageCheckedAt = nil
					items[index].LatestSnapshot = nil
				}
			}
			continue
		}
		groupByKeyID := make(map[int64]string, len(apiKeys))
		for _, apiKey := range apiKeys {
			groupByKeyID[apiKey.ID] = strings.TrimSpace(apiKey.GroupID)
		}
		snapshotByGroupID := make(map[string]UpstreamRelayGroupRateSnapshot, len(snapshots))
		for _, snapshot := range snapshots {
			snapshotByGroupID[snapshot.UpstreamGroupID] = snapshot
		}
		for _, index := range indices {
			groupID := groupByKeyID[*items[index].UpstreamAPIKeyID]
			if groupID == "" {
				if clearUnresolved {
					items[index].TodayActualCost = nil
					items[index].TodayTotalTokens = nil
					items[index].TodayUsageCheckedAt = nil
					items[index].LatestSnapshot = nil
				}
				continue
			}
			items[index].UpstreamGroupID = groupID
			items[index].UpstreamGroupName = ""
			items[index].TodayActualCost = nil
			items[index].TodayTotalTokens = nil
			items[index].TodayUsageCheckedAt = nil
			items[index].LatestSnapshot = nil
			if snapshot, ok := snapshotByGroupID[groupID]; ok {
				snapshotCopy := snapshot
				items[index].UpstreamGroupName = snapshot.Name
				items[index].TodayActualCost = snapshot.TodayActualCost
				items[index].TodayTotalTokens = snapshot.TodayTotalTokens
				items[index].TodayUsageCheckedAt = snapshot.TodayUsageCheckedAt
				items[index].LatestSnapshot = &snapshotCopy
			}
		}
	}
}

func (s *UpstreamRelayGroupMonitoringService) ListConnectorAPIKeys(ctx context.Context, connectorID int64) ([]UpstreamRelayAPIKeyOption, error) {
	connector, err := s.repo.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	if err := s.decryptConnector(connector); err != nil {
		_ = s.repo.MarkConnectorSync(ctx, connectorID, UpstreamRelayConnectorStatusNeedsReauth, err.Error())
		return nil, err
	}
	items, err := s.fetchUpstreamAPIKeyOptions(ctx, connector)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *UpstreamRelayGroupMonitoringService) CreateCandidate(ctx context.Context, input UpstreamRelayCandidateInput, operatorID int64) (*UpstreamRelayCandidate, error) {
	candidate, err := normalizeUpstreamRelayCandidateInput(input, 0, operatorID)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateCandidate(ctx, candidate)
	if err != nil {
		return nil, err
	}
	createdItems := []UpstreamRelayCandidate{*created}
	if err := s.decorateCandidateHealthFreshness(ctx, createdItems); err != nil {
		return nil, err
	}
	*created = createdItems[0]
	sanitizeUpstreamRelayCandidateAPIKeyDisplay(created)
	return created, nil
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
	updated, err := s.repo.UpdateCandidate(ctx, candidate)
	if err != nil {
		return nil, err
	}
	updatedItems := []UpstreamRelayCandidate{*updated}
	if err := s.decorateCandidateHealthFreshness(ctx, updatedItems); err != nil {
		return nil, err
	}
	*updated = updatedItems[0]
	sanitizeUpstreamRelayCandidateAPIKeyDisplay(updated)
	return updated, nil
}

func (s *UpstreamRelayGroupMonitoringService) decorateCandidateHealthFreshness(ctx context.Context, items []UpstreamRelayCandidate) error {
	if len(items) == 0 {
		return nil
	}
	policy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	staleAfter := time.Duration(policy.ProbeStaleAfterMinutes) * time.Minute
	for i := range items {
		if items[i].Health == nil {
			continue
		}
		items[i].Health.Stale = items[i].Health.CalculatedAt.IsZero() || now.Sub(items[i].Health.CalculatedAt) > staleAfter
	}
	return nil
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

func (s *UpstreamRelayGroupMonitoringService) ProbeAllCandidates(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
	policy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return nil, err
	}
	candidates, err := s.listAllEnabledCandidates(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpstreamRelayBulkOperationResult{
		Total: len(candidates),
		Items: make([]UpstreamRelayBulkOperationItem, len(candidates)),
	}
	limit := positiveOrDefault(policy.ProbeConcurrency, upstreamRelayDefaultProbeLimit)
	s.runLimited(len(candidates), limit, func(index int) {
		candidate := candidates[index]
		item := UpstreamRelayBulkOperationItem{
			ID:            candidate.ID,
			ConnectorID:   candidate.ConnectorID,
			ConnectorName: candidate.ConnectorName,
			CandidateID:   candidate.ID,
			AccountID:     candidate.AccountID,
			AccountName:   candidate.AccountName,
		}
		probe, err := s.ProbeCandidate(ctx, candidate.ID)
		if err != nil {
			item.ErrorReason = sanitizeUpstreamRelayError(err.Error())
		} else if probe == nil || !probe.Success {
			if probe != nil {
				item.ProbeResultID = &probe.ID
				item.LatencyMs = probe.LatencyMs
				item.HTTPStatus = probe.HTTPStatus
				item.ErrorClass = strings.TrimSpace(probe.ErrorClass)
				item.ProbedAt = &probe.ProbedAt
				item.ErrorReason = strings.TrimSpace(probe.ErrorMessage)
				if item.ErrorReason == "" {
					item.ErrorReason = item.ErrorClass
				}
			}
			if item.ErrorReason == "" {
				item.ErrorReason = "probe failed"
			}
		} else {
			item.Success = true
			item.Count = 1
			item.ProbeResultID = &probe.ID
			item.LatencyMs = probe.LatencyMs
			item.HTTPStatus = probe.HTTPStatus
			item.ErrorClass = strings.TrimSpace(probe.ErrorClass)
			item.ProbedAt = &probe.ProbedAt
		}
		result.Items[index] = item
	})
	result.summarize()
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) listAllConnectors(ctx context.Context) ([]UpstreamRelayConnector, error) {
	const pageSize = 100
	out := []UpstreamRelayConnector{}
	for page := 1; ; page++ {
		items, pageResult, err := s.ListConnectors(ctx, page, pageSize, UpstreamRelayConnectorListFilters{})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if pageResult == nil || page >= pageResult.Pages {
			break
		}
	}
	return out, nil
}

func (s *UpstreamRelayGroupMonitoringService) listAllEnabledCandidates(ctx context.Context) ([]UpstreamRelayCandidate, error) {
	const pageSize = 100
	enabled := true
	out := []UpstreamRelayCandidate{}
	for page := 1; ; page++ {
		items, pageResult, err := s.ListCandidates(ctx, page, pageSize, UpstreamRelayCandidateListFilters{Enabled: &enabled})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if pageResult == nil || page >= pageResult.Pages {
			break
		}
	}
	return out, nil
}

func (s *UpstreamRelayGroupMonitoringService) runLimited(total int, limit int, fn func(index int)) {
	if total <= 0 {
		return
	}
	limit = positiveOrDefault(limit, 1)
	if limit > total {
		limit = total
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(index)
		}(i)
	}
	wg.Wait()
}

func (r *UpstreamRelayBulkOperationResult) summarize() {
	if r == nil {
		return
	}
	r.Success = 0
	r.Failed = 0
	for _, item := range r.Items {
		if item.Success {
			r.Success++
		} else {
			r.Failed++
		}
	}
}

func (r *UpstreamRelayMonitoringRefreshResult) summarize() {
	if r == nil {
		return
	}
	r.Success = 0
	r.Partial = 0
	r.Failed = 0
	for _, item := range r.Items {
		switch item.Status {
		case upstreamRelayMetricsRefreshStatusSuccess:
			r.Success++
		case upstreamRelayMetricsRefreshStatusPartial, upstreamRelayMetricsRefreshStatusSkipped:
			r.Partial++
		default:
			r.Failed++
		}
	}
	if r.Total == 0 || (r.Success == r.Total && r.Partial == 0 && r.Failed == 0) {
		r.Status = upstreamRelayMetricsRefreshStatusSuccess
		return
	}
	if r.Failed == r.Total {
		r.Status = upstreamRelayMetricsRefreshStatusFailed
		return
	}
	r.Status = upstreamRelayMetricsRefreshStatusPartial
}

func positiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func (s *UpstreamRelayGroupMonitoringService) applyMonitoringPolicyFreshness(ctx context.Context, policy UpstreamRelayRecommendationPolicy) (UpstreamRelayRecommendationPolicy, error) {
	monitoringPolicy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return policy, err
	}
	policy.SnapshotFreshnessMinutes = monitoringPolicy.SnapshotStaleAfterMinutes
	policy.UsageDeltaFreshnessMinutes = monitoringPolicy.UsageDeltaStaleAfterMinutes
	policy.ProbeFreshnessMinutes = monitoringPolicy.ProbeStaleAfterMinutes
	return policy, nil
}

func (s *UpstreamRelayGroupMonitoringService) GenerateRecommendations(ctx context.Context, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	candidates, err := s.repo.ListRecommendationInputs(ctx)
	if err != nil {
		return nil, err
	}
	s.decorateCandidateCurrentGroups(ctx, candidates, true)
	policy, err := s.GetRecommendationPolicy(ctx)
	if err != nil {
		return nil, err
	}
	preview := buildUpstreamRelayRecommendationPreview(candidates, *policy)
	run := UpstreamRelayRecommendationRun{
		Status:          UpstreamRelayRunStatusSuccess,
		TotalCandidates: len(candidates),
		SuggestionCount: len(preview.Suggestions),
		CreatedBy:       operatorID,
	}
	return s.repo.CreateRecommendationRun(ctx, run, preview.Suggestions)
}

func (s *UpstreamRelayGroupMonitoringService) GenerateAndMaybeApplyRecommendations(ctx context.Context) (*UpstreamRelayRecommendationAutomationResult, error) {
	policy, err := s.GetMonitoringPolicy(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.GenerateRecommendations(ctx, 0)
	if err != nil {
		return nil, err
	}
	result := &UpstreamRelayRecommendationAutomationResult{Run: run}
	if !policy.AutoApplyRecommendationsEnabled {
		result.SkippedReason = "auto_apply_disabled"
		return result, nil
	}
	if reason := validateUpstreamRelayAutoApplyRun(run, *policy); reason != "" {
		result.SkippedReason = reason
		slog.Info("upstream relay recommendation auto-apply skipped", "run_id", run.ID, "reason", reason)
		return result, nil
	}
	applied, err := s.ApplyRecommendationRun(ctx, run.ID, 0)
	if err != nil {
		return result, err
	}
	result.Run = applied
	result.Applied = true
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) GetMonitoringPolicy(ctx context.Context) (*UpstreamRelayMonitoringPolicy, error) {
	policy, err := s.repo.GetMonitoringPolicy(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeUpstreamRelayMonitoringPolicy(monitoringPolicyOrDefault(policy))
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *UpstreamRelayGroupMonitoringService) UpdateMonitoringPolicy(ctx context.Context, input UpstreamRelayMonitoringPolicy, operatorID int64) (*UpstreamRelayMonitoringPolicy, error) {
	policy, err := normalizeUpstreamRelayMonitoringPolicy(input)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpsertMonitoringPolicy(ctx, policy, operatorID)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeUpstreamRelayMonitoringPolicy(monitoringPolicyOrDefault(saved))
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *UpstreamRelayGroupMonitoringService) GetRecommendationPolicy(ctx context.Context) (*UpstreamRelayRecommendationPolicy, error) {
	policy, err := s.repo.GetRecommendationPolicy(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeUpstreamRelayRecommendationPolicy(policyOrDefault(policy))
	if err != nil {
		return nil, err
	}
	normalized, err = s.applyMonitoringPolicyFreshness(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *UpstreamRelayGroupMonitoringService) UpdateRecommendationPolicy(ctx context.Context, input UpstreamRelayRecommendationPolicy, operatorID int64) (*UpstreamRelayRecommendationPolicy, error) {
	policy, err := normalizeUpstreamRelayRecommendationPolicy(input)
	if err != nil {
		return nil, err
	}
	policy, err = s.applyMonitoringPolicyFreshness(ctx, policy)
	if err != nil {
		return nil, err
	}
	return s.repo.UpsertRecommendationPolicy(ctx, policy, operatorID)
}

func (s *UpstreamRelayGroupMonitoringService) PreviewRecommendations(ctx context.Context, input *UpstreamRelayRecommendationPolicy) (*UpstreamRelayRecommendationPreview, error) {
	var policy UpstreamRelayRecommendationPolicy
	var err error
	if input == nil {
		var saved *UpstreamRelayRecommendationPolicy
		saved, err = s.GetRecommendationPolicy(ctx)
		if err != nil {
			return nil, err
		}
		policy = *saved
	} else {
		policy, err = normalizeUpstreamRelayRecommendationPolicy(*input)
		if err != nil {
			return nil, err
		}
	}
	policy, err = s.applyMonitoringPolicyFreshness(ctx, policy)
	if err != nil {
		return nil, err
	}
	candidates, err := s.repo.ListRecommendationInputs(ctx)
	if err != nil {
		return nil, err
	}
	s.decorateCandidateCurrentGroups(ctx, candidates, true)
	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)
	return &preview, nil
}

func (s *UpstreamRelayGroupMonitoringService) GetRecommendationRun(ctx context.Context, id int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.GetRecommendationRun(ctx, id)
}

func (s *UpstreamRelayGroupMonitoringService) ListRecommendationRuns(ctx context.Context, page, pageSize int, filters UpstreamRelayRecommendationRunListFilters) ([]UpstreamRelayRecommendationRun, *pagination.PaginationResult, error) {
	return s.repo.ListRecommendationRuns(ctx, pagination.PaginationParams{Page: page, PageSize: pageSize}, filters)
}

func (s *UpstreamRelayGroupMonitoringService) ApplyRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.ApplyRecommendationRun(ctx, runID, operatorID)
}

func (s *UpstreamRelayGroupMonitoringService) CloseRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.CloseRecommendationRun(ctx, runID, operatorID)
}

func (s *UpstreamRelayGroupMonitoringService) RestoreRecommendationRun(ctx context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	return s.repo.RestoreRecommendationRun(ctx, runID, operatorID)
}

func (s *UpstreamRelayGroupMonitoringService) DeleteRecommendationRun(ctx context.Context, runID int64) error {
	return s.repo.DeleteRecommendationRun(ctx, runID)
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
		loginEmail := strings.TrimSpace(stringPointerValue(input.LoginEmail))
		loginPassword := strings.TrimSpace(stringPointerValue(input.LoginPassword))
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
		explicitBearerToken := strings.TrimSpace(stringPointerValue(input.BearerToken)) != ""
	if (baseURLChanged || previousAuthMode == UpstreamRelayAuthModePasswordLogin) && !explicitBearerToken {
		return nil, false, ErrUpstreamRelayInvalidManualSession
	}
	connector.LoginEmailEncrypted = ""
	if (baseURLChanged || previousAuthMode == UpstreamRelayAuthModePasswordLogin) && input.RefreshToken == nil {
		connector.RefreshTokenEncrypted = ""
	}
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
	if input.RefreshToken != nil {
		refreshToken := strings.TrimSpace(*input.RefreshToken)
		if refreshToken == "" {
			connector.RefreshTokenEncrypted = ""
		} else {
			encrypted, err := s.encryptor.Encrypt(refreshToken)
			if err != nil {
				return nil, false, fmt.Errorf("encrypt refresh token: %w", err)
			}
			connector.RefreshTokenEncrypted = encrypted
		}
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
	usageByGroup, usageCheckedAt, _, _ := s.fetchUpstreamGroupTodayUsage(ctx, connector, now)
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
		var todayActualCost *float64
		var todayTotalTokens *int64
		var todayUsageCheckedAt *time.Time
		if usage, known := usageByGroup[group.ID]; usageCheckedAt != nil && known {
			actualCost := usage.ActualCost
			totalTokens := usage.TotalTokens
			todayActualCost = &actualCost
			todayTotalTokens = &totalTokens
			todayUsageCheckedAt = usageCheckedAt
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
			TodayActualCost:        todayActualCost,
			TodayTotalTokens:       todayTotalTokens,
			TodayUsageCheckedAt:    todayUsageCheckedAt,
			Source:                 source,
			LastSeenAt:             now,
		})
	}
	return snapshots, nil
}

func (s *UpstreamRelayGroupMonitoringService) persistKnownUsageHistory(ctx context.Context, connectorID int64, snapshots []UpstreamRelayGroupRateSnapshot) error {
	if s == nil || s.repo == nil || len(snapshots) == 0 {
		return nil
	}
	rows := make([]UpstreamRelayGroupUsageHistoryUpsert, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.TodayActualCost == nil || snapshot.TodayTotalTokens == nil || snapshot.TodayUsageCheckedAt == nil {
			continue
		}
		rows = append(rows, UpstreamRelayGroupUsageHistoryUpsert{
			UsageDate:       upstreamRelayUsageDate(*snapshot.TodayUsageCheckedAt),
			ConnectorID:     connectorID,
			UpstreamGroupID: snapshot.UpstreamGroupID,
			GroupName:       snapshot.Name,
			Platform:        snapshot.Platform,
			ActualCost:      *snapshot.TodayActualCost,
			TotalTokens:     *snapshot.TodayTotalTokens,
			CheckedAt:       *snapshot.TodayUsageCheckedAt,
		})
	}
	return s.repo.UpsertUsageHistory(ctx, rows)
}

func (s *UpstreamRelayGroupMonitoringService) persistKnownUsageHistoryFromRefresh(ctx context.Context, connectorID int64, usageByGroup map[string]UpstreamRelayGroupTodayUsage, checkedAt *time.Time, complete bool) error {
	if s == nil || s.repo == nil || usageByGroup == nil || checkedAt == nil {
		return nil
	}
	snapshots, err := s.repo.ListSnapshots(ctx, connectorID)
	if err != nil {
		return err
	}
	rows := make([]UpstreamRelayGroupUsageHistoryUpsert, 0, len(snapshots))
	usageDate := upstreamRelayUsageDate(*checkedAt)
	for _, snapshot := range snapshots {
		usage, known := usageByGroup[snapshot.UpstreamGroupID]
		if !complete && !known {
			continue
		}
		rows = append(rows, UpstreamRelayGroupUsageHistoryUpsert{
			UsageDate:       usageDate,
			ConnectorID:     connectorID,
			UpstreamGroupID: snapshot.UpstreamGroupID,
			GroupName:       snapshot.Name,
			Platform:        snapshot.Platform,
			ActualCost:      usage.ActualCost,
			TotalTokens:     usage.TotalTokens,
			CheckedAt:       *checkedAt,
		})
	}
	return s.repo.UpsertUsageHistory(ctx, rows)
}

func (s *UpstreamRelayGroupMonitoringService) fetchUpstreamGroupTodayUsage(ctx context.Context, connector *UpstreamRelayConnector, now time.Time) (map[string]UpstreamRelayGroupTodayUsage, *time.Time, []UpstreamRelayMetricsIssueDetail, error) {
	return s.fetchUpstreamGroupUsageForDate(ctx, connector, upstreamRelayUsageDate(now))
}

func (s *UpstreamRelayGroupMonitoringService) fetchUpstreamGroupUsageForDate(ctx context.Context, connector *UpstreamRelayConnector, date string) (map[string]UpstreamRelayGroupTodayUsage, *time.Time, []UpstreamRelayMetricsIssueDetail, error) {
	if s == nil || s.repo == nil {
		return nil, nil, nil, fmt.Errorf("usage refresh requires repository")
	}
	if connector == nil {
		return nil, nil, nil, fmt.Errorf("connector not found")
	}
	if !isRelayUsageDate(date) {
		return nil, nil, nil, fmt.Errorf("usage date must use YYYY-MM-DD")
	}
	checkedAt := time.Now()
	out := map[string]UpstreamRelayGroupTodayUsage{}
	bindings, err := s.repo.ListCandidateUsageBindings(ctx, connector.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(bindings) == 0 {
		return out, &checkedAt, []UpstreamRelayMetricsIssueDetail{{
			Code:    upstreamRelayMetricsIssueNoCandidateBindings,
			Message: "connector has no local account bindings",
		}}, nil
	}
	issues := make([]UpstreamRelayMetricsIssueDetail, 0)
	usageByKey := map[int64]UpstreamRelayGroupTodayUsage{}
	// ponytail: 上游没有 Key 分组历史接口；历史日期使用候选保存分组，若上游支持按日期查询再替换。
	resolveCurrentGroups := date == upstreamRelayUsageDate(time.Now())
	apiKeyByID := map[int64]UpstreamRelayAPIKeyOption{}
	if resolveCurrentGroups {
		apiKeys, err := s.fetchUpstreamAPIKeyOptions(ctx, connector)
		if err != nil {
			return nil, nil, nil, err
		}
		apiKeyByID = make(map[int64]UpstreamRelayAPIKeyOption, len(apiKeys))
		for _, apiKey := range apiKeys {
			apiKeyByID[apiKey.ID] = apiKey
		}
	}
	failedKeys := map[int64]string{}
	countedKeys := map[int64]struct{}{}
	invalidGroups := map[string]struct{}{}
	for _, binding := range bindings {
		if binding.UpstreamGroupID == "" {
			continue
		}
		if binding.UpstreamAPIKeyID <= 0 {
			issues = append(issues, UpstreamRelayMetricsIssueDetail{
				Code:            upstreamRelayMetricsIssueMissingAPIKeyBinding,
				Message:         fmt.Sprintf("candidate %d for bound account %d has no upstream api key binding", binding.CandidateID, binding.AccountID),
				CandidateID:     binding.CandidateID,
				AccountID:       binding.AccountID,
				UpstreamGroupID: binding.UpstreamGroupID,
			})
			invalidGroups[binding.UpstreamGroupID] = struct{}{}
			continue
		}
		upstreamKeyID := binding.UpstreamAPIKeyID
		currentGroupID := strings.TrimSpace(binding.UpstreamGroupID)
		if resolveCurrentGroups {
			apiKey, visible := apiKeyByID[upstreamKeyID]
			if !visible {
				issues = append(issues, UpstreamRelayMetricsIssueDetail{
					Code:            upstreamRelayMetricsIssueAPIKeyNotVisible,
					Message:         fmt.Sprintf("upstream api key %d is not visible", upstreamKeyID),
					CandidateID:     binding.CandidateID,
					AccountID:       binding.AccountID,
					UpstreamGroupID: binding.UpstreamGroupID,
				})
				invalidGroups[binding.UpstreamGroupID] = struct{}{}
				continue
			}
			currentGroupID = strings.TrimSpace(apiKey.GroupID)
			if currentGroupID == "" {
				issues = append(issues, UpstreamRelayMetricsIssueDetail{
					Code:            upstreamRelayMetricsIssueAPIKeyGroupUnavailable,
					Message:         fmt.Sprintf("upstream api key %d has no current group", upstreamKeyID),
					CandidateID:     binding.CandidateID,
					AccountID:       binding.AccountID,
					UpstreamGroupID: binding.UpstreamGroupID,
				})
				invalidGroups[binding.UpstreamGroupID] = struct{}{}
				continue
			}
		}
		if _, counted := countedKeys[upstreamKeyID]; counted {
			continue
		}
		if message, failed := failedKeys[upstreamKeyID]; failed {
			issues = append(issues, UpstreamRelayMetricsIssueDetail{
				Code:            upstreamRelayMetricsIssueUpstreamUsageRequest,
				Message:         message,
				CandidateID:     binding.CandidateID,
				AccountID:       binding.AccountID,
				UpstreamGroupID: currentGroupID,
			})
			invalidGroups[currentGroupID] = struct{}{}
			continue
		}
		usage, cached := usageByKey[upstreamKeyID]
		if !cached {
			usage, err = s.fetchUpstreamAPIKeyUsageStats(ctx, connector, upstreamKeyID, date)
			if err != nil {
				message := fmt.Sprintf("fetch upstream api key %d usage stats: %s", upstreamKeyID, err.Error())
				failedKeys[upstreamKeyID] = message
				issues = append(issues, UpstreamRelayMetricsIssueDetail{
					Code:            upstreamRelayMetricsIssueUpstreamUsageRequest,
					Message:         message,
					CandidateID:     binding.CandidateID,
					AccountID:       binding.AccountID,
					UpstreamGroupID: currentGroupID,
				})
				invalidGroups[currentGroupID] = struct{}{}
				continue
			}
			usageByKey[upstreamKeyID] = usage
		}
		groupUsage := out[currentGroupID]
		groupUsage.ActualCost += usage.ActualCost
		groupUsage.TotalTokens += usage.TotalTokens
		out[currentGroupID] = groupUsage
		countedKeys[upstreamKeyID] = struct{}{}
	}
	for groupID := range invalidGroups {
		delete(out, groupID)
	}
	return out, &checkedAt, issues, nil
}

func (s *UpstreamRelayGroupMonitoringService) FinalizeUsageForConnectorDate(ctx context.Context, connectorID int64, date string) error {
	targetEnd, err := upstreamRelayUsageDateEndOfDay(date)
	if err != nil {
		return infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_USAGE_DATE", "usage date must use YYYY-MM-DD")
	}
	if date >= upstreamRelayUsageDate(time.Now()) {
		return infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_FINALIZE_USAGE_DATE", "only historical usage dates can be finalized")
	}
	connector, err := s.repo.GetConnector(ctx, connectorID)
	if err != nil {
		return err
	}
	if err := s.decryptConnector(connector); err != nil {
		_ = s.repo.MarkConnectorSync(ctx, connectorID, UpstreamRelayConnectorStatusNeedsReauth, err.Error())
		return err
	}
	usageByGroup, _, issues, err := s.fetchUpstreamGroupUsageForDate(ctx, connector, date)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return fmt.Errorf("usage unavailable for %s: %s", date, issues[0].Message)
	}
	snapshots, err := s.repo.ListSnapshots(ctx, connectorID)
	if err != nil {
		return err
	}
	rows := make([]UpstreamRelayGroupUsageHistoryUpsert, 0, len(snapshots))
	for _, snapshot := range snapshots {
		usage := usageByGroup[snapshot.UpstreamGroupID]
		rows = append(rows, UpstreamRelayGroupUsageHistoryUpsert{
			UsageDate:       date,
			ConnectorID:     connectorID,
			UpstreamGroupID: snapshot.UpstreamGroupID,
			GroupName:       snapshot.Name,
			Platform:        snapshot.Platform,
			ActualCost:      usage.ActualCost,
			TotalTokens:     usage.TotalTokens,
			CheckedAt:       targetEnd,
			Finalized:       true,
		})
	}
	return s.repo.UpsertUsageHistory(ctx, rows)
}

func (s *UpstreamRelayGroupMonitoringService) FinalizeYesterdayUsage(ctx context.Context) (*UpstreamRelayBulkOperationResult, error) {
	connectors, err := s.listAllConnectors(ctx)
	if err != nil {
		return nil, err
	}
	targetDate := upstreamRelayPreviousUsageDate(time.Now())
	result := &UpstreamRelayBulkOperationResult{
		Total: len(connectors),
		Items: make([]UpstreamRelayBulkOperationItem, len(connectors)),
	}
	for i, connector := range connectors {
		item := UpstreamRelayBulkOperationItem{
			ID:            connector.ID,
			ConnectorID:   connector.ID,
			ConnectorName: connector.Name,
		}
		if err := s.FinalizeUsageForConnectorDate(ctx, connector.ID, targetDate); err != nil {
			item.ErrorReason = sanitizeUpstreamRelayError(err.Error())
			slog.Warn("upstream relay usage finalize failed", "connector_id", connector.ID, "date", targetDate, "error", err)
		} else {
			item.Success = true
			item.Count = 1
		}
		result.Items[i] = item
	}
	result.summarize()
	return result, nil
}

func (s *UpstreamRelayGroupMonitoringService) finalizePendingUsageForConnector(ctx context.Context, connectorID int64) {
	if s == nil || s.repo == nil || connectorID <= 0 {
		return
	}
	for _, date := range upstreamRelayRecentHistoricalUsageDates(time.Now(), 7) {
		summary, err := s.repo.SummarizeUsageHistory(ctx, UpstreamRelayUsageHistoryListFilters{
			StartDate:        date,
			EndDate:          date,
			ConnectorID:      connectorID,
			IncludeZeroUsage: true,
		})
		if err != nil {
			slog.Warn("upstream relay usage finalize status failed", "connector_id", connectorID, "date", date, "error", err)
			continue
		}
		if summary != nil && summary.RowCount > 0 && summary.PendingFinalize == 0 {
			continue
		}
		if err := s.FinalizeUsageForConnectorDate(ctx, connectorID, date); err != nil {
			slog.Warn("upstream relay pending usage finalize failed", "connector_id", connectorID, "date", date, "error", err)
		}
	}
}

func (s *UpstreamRelayGroupMonitoringService) fetchUpstreamAPIKeyOptions(ctx context.Context, connector *UpstreamRelayConnector) ([]UpstreamRelayAPIKeyOption, error) {
	out := []UpstreamRelayAPIKeyOption{}
	for page := 1; page <= upstreamRelayUsageFetchMaxPages; page++ {
		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(upstreamRelayUsageFetchPageSize))
		body, err := s.getUpstreamJSON(ctx, connector, "/api/v1/keys?"+query.Encode())
		if err != nil {
			return nil, fmt.Errorf("fetch upstream keys page %d: %w", page, err)
		}
		items, pages, err := parseUpstreamAPIKeyPage(body)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.ID <= 0 {
				continue
			}
			out = append(out, UpstreamRelayAPIKeyOption{
				ID:        item.ID,
				Name:      item.Name,
				MaskedKey: maskUpstreamRelayAPIKeyForDisplay(item.Key),
				GroupID:   item.GroupID,
			})
		}
		if pages <= page {
			return out, nil
		}
	}
	return nil, fmt.Errorf("upstream api key pagination exceeds safety limit %d", upstreamRelayUsageFetchMaxPages)
}

func (s *UpstreamRelayGroupMonitoringService) fetchUpstreamAPIKeyUsageStats(ctx context.Context, connector *UpstreamRelayConnector, apiKeyID int64, date string) (UpstreamRelayGroupTodayUsage, error) {
	query := url.Values{}
	query.Set("start_date", date)
	query.Set("end_date", date)
	query.Set("api_key_id", strconv.FormatInt(apiKeyID, 10))
	query.Set("timezone", "Asia/Shanghai")
	body, err := s.getUpstreamJSON(ctx, connector, "/api/v1/usage/stats?"+query.Encode())
	if err != nil {
		return UpstreamRelayGroupTodayUsage{}, err
	}
	return parseUpstreamUsageStats(body)
}

func upstreamRelayUsageDate(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return t.Format("2006-01-02")
	}
	return t.In(loc).Format("2006-01-02")
}

func upstreamRelayUsageLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}

func upstreamRelayUsageDateEndOfDay(date string) (time.Time, error) {
	loc := upstreamRelayUsageLocation()
	parsed, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil || parsed.Format("2006-01-02") != date {
		return time.Time{}, fmt.Errorf("invalid usage date")
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, loc), nil
}

func upstreamRelayPreviousUsageDate(now time.Time) string {
	loc := upstreamRelayUsageLocation()
	return now.In(loc).AddDate(0, 0, -1).Format("2006-01-02")
}

func upstreamRelayRecentHistoricalUsageDates(now time.Time, days int) []string {
	if days <= 0 {
		return nil
	}
	loc := upstreamRelayUsageLocation()
	base := now.In(loc)
	out := make([]string, 0, days)
	for i := 1; i <= days; i++ {
		out = append(out, base.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return out
}

func isRelayUsageDate(v string) bool {
	if len(v) != len("2006-01-02") {
		return false
	}
	parsed, err := time.Parse("2006-01-02", v)
	return err == nil && parsed.Format("2006-01-02") == v
}

func (s *UpstreamRelayGroupMonitoringService) refreshConnectorAccountBalance(ctx context.Context, connector *UpstreamRelayConnector) error {
	if s == nil || s.repo == nil || connector == nil {
		return nil
	}
	balance, err := s.fetchUpstreamAccountBalance(ctx, connector)
	checkedAt := time.Now()
	if err != nil {
		_ = s.repo.UpdateConnectorAccountBalance(ctx, connector.ID, nil, nil)
		if connector.Status == UpstreamRelayConnectorStatusNeedsReauth {
			return err
		}
		return nil
	}
	_ = s.repo.UpdateConnectorAccountBalance(ctx, connector.ID, balance, &checkedAt)
	return nil
}

func (s *UpstreamRelayGroupMonitoringService) fetchUpstreamAccountBalance(ctx context.Context, connector *UpstreamRelayConnector) (*float64, error) {
	body, err := s.getUpstreamJSON(ctx, connector, "/api/v1/user/profile")
	if err != nil {
		return nil, fmt.Errorf("fetch upstream account balance: %w", err)
	}
	balance, err := parseUpstreamAccountBalance(body)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

func (s *UpstreamRelayGroupMonitoringService) getUpstreamJSON(ctx context.Context, connector *UpstreamRelayConnector, path string) ([]byte, error) {
	body, err := s.doUpstreamJSON(ctx, connector, path)
	if err == nil {
		return body, nil
	}
	var upstreamErr *upstreamRelayHTTPError
	if !errors.As(err, &upstreamErr) || !upstreamErr.isTokenExpired() || strings.TrimSpace(connector.RefreshTokenPlain) == "" {
		return nil, err
	}
	if refreshErr := s.refreshUpstreamRelayConnectorToken(ctx, connector); refreshErr != nil {
		if s.reloadConnectorCredentialsAfterRefreshFailure(ctx, connector) == nil {
			return s.retryUpstreamJSONAfterRefresh(ctx, connector, path)
		}
		s.markConnectorRefreshFailure(ctx, connector, refreshErr)
		return nil, fmt.Errorf("upstream token refresh failed: %w", refreshErr)
	}
	return s.retryUpstreamJSONAfterRefresh(ctx, connector, path)
}

func (s *UpstreamRelayGroupMonitoringService) retryUpstreamJSONAfterRefresh(ctx context.Context, connector *UpstreamRelayConnector, path string) ([]byte, error) {
	body, err := s.doUpstreamJSON(ctx, connector, path)
	if err == nil {
		return body, nil
	}
	var upstreamErr *upstreamRelayHTTPError
	if errors.As(err, &upstreamErr) && upstreamErr.isTokenExpired() {
		s.markConnectorRefreshFailure(ctx, connector, err)
	}
	return nil, err
}

func (s *UpstreamRelayGroupMonitoringService) markConnectorRefreshFailure(ctx context.Context, connector *UpstreamRelayConnector, err error) {
	if s == nil || s.repo == nil || connector == nil || err == nil {
		return
	}
	message := sanitizeUpstreamRelayError(fmt.Sprintf("upstream token refresh failed: %v", err))
	connector.Status = UpstreamRelayConnectorStatusNeedsReauth
	connector.LastError = message
	_ = s.repo.MarkConnectorSync(ctx, connector.ID, UpstreamRelayConnectorStatusNeedsReauth, message)
}

func (s *UpstreamRelayGroupMonitoringService) reloadConnectorCredentialsAfterRefreshFailure(ctx context.Context, connector *UpstreamRelayConnector) error {
	if s == nil || s.repo == nil || connector == nil {
		return ErrUpstreamRelayConnectorNotFound
	}
	previousVersion := connector.CredentialVersion
	return s.reloadConnectorCredentialsIfVersionAdvanced(ctx, connector, previousVersion)
}

func (s *UpstreamRelayGroupMonitoringService) reloadConnectorCredentialsIfVersionAdvanced(ctx context.Context, connector *UpstreamRelayConnector, previousVersion int64) error {
	if err := s.reloadConnectorCredentials(ctx, connector); err != nil {
		return err
	}
	if connector.CredentialVersion <= previousVersion {
		return ErrUpstreamRelayCredentialVersionConflict
	}
	if strings.TrimSpace(connector.BearerTokenPlain) == "" {
		return fmt.Errorf("upstream bearer token is empty after credential reload")
	}
	return nil
}

func (s *UpstreamRelayGroupMonitoringService) doUpstreamJSON(ctx context.Context, connector *UpstreamRelayConnector, path string) ([]byte, error) {
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
		return nil, &upstreamRelayHTTPError{StatusCode: resp.StatusCode, Body: body}
	}
	return body, nil
}

type upstreamRelayHTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *upstreamRelayHTTPError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("upstream HTTP %d: %s", e.StatusCode, sanitizeUpstreamRelayError(string(e.Body)))
}

func (e *upstreamRelayHTTPError) isTokenExpired() bool {
	return e != nil && e.StatusCode == http.StatusUnauthorized && upstreamRelayBodyHasReason(e.Body, "TOKEN_EXPIRED")
}

func upstreamRelayBodyHasReason(body []byte, reason string) bool {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return strings.Contains(strings.ToUpper(string(body)), reason)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return false
	}
	if strings.EqualFold(relayString(root["code"]), reason) || strings.EqualFold(relayString(root["reason"]), reason) {
		return true
	}
	if data, ok := root["data"].(map[string]any); ok {
		return strings.EqualFold(relayString(data["code"]), reason) || strings.EqualFold(relayString(data["reason"]), reason)
	}
	return false
}

func (s *UpstreamRelayGroupMonitoringService) refreshUpstreamRelayConnectorToken(ctx context.Context, connector *UpstreamRelayConnector) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("upstream token refresh requires repository")
	}
	if connector == nil {
		return ErrUpstreamRelayConnectorNotFound
	}
	refreshToken := strings.TrimSpace(connector.RefreshTokenPlain)
	if refreshToken == "" {
		return fmt.Errorf("upstream refresh token is empty")
	}
	payload, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(connector.BaseURL, "/")+"/api/v1/auth/refresh", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream refresh HTTP %d: %s", resp.StatusCode, sanitizeUpstreamRelayError(string(body)))
	}
	token, _, err := parseUpstreamRelayLoginToken(body)
	if err != nil {
		return err
	}
	accessToken := normalizeBearerToken(token.AccessToken)
	nextRefreshToken := strings.TrimSpace(token.RefreshToken)
	if accessToken == "" || nextRefreshToken == "" {
		return fmt.Errorf("upstream refresh response missing access_token or refresh_token")
	}
	encryptedAccess, err := s.encryptor.Encrypt(accessToken)
	if err != nil {
		return fmt.Errorf("encrypt bearer token: %w", err)
	}
	encryptedRefresh, err := s.encryptor.Encrypt(nextRefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	updated, err := s.repo.UpdateConnectorTokens(ctx, connector.ID, connector.CredentialVersion, encryptedAccess, encryptedRefresh)
	if err != nil {
		if errors.Is(err, ErrUpstreamRelayCredentialVersionConflict) {
			return s.reloadConnectorCredentialsIfVersionAdvanced(ctx, connector, connector.CredentialVersion)
		}
		return err
	}
	connector.BearerTokenEncrypted = updated.BearerTokenEncrypted
	connector.RefreshTokenEncrypted = updated.RefreshTokenEncrypted
	connector.CredentialVersion = updated.CredentialVersion
	connector.Status = updated.Status
	connector.LastError = updated.LastError
	connector.BearerTokenPlain = accessToken
	connector.RefreshTokenPlain = nextRefreshToken
	return nil
}

func (s *UpstreamRelayGroupMonitoringService) reloadConnectorCredentials(ctx context.Context, connector *UpstreamRelayConnector) error {
	latest, err := s.repo.GetConnector(ctx, connector.ID)
	if err != nil {
		return err
	}
	if err := s.decryptConnector(latest); err != nil {
		return err
	}
	if strings.TrimSpace(latest.BearerTokenPlain) == "" {
		return fmt.Errorf("upstream bearer token is empty after credential reload")
	}
	connector.BearerTokenEncrypted = latest.BearerTokenEncrypted
	connector.RefreshTokenEncrypted = latest.RefreshTokenEncrypted
	connector.CredentialVersion = latest.CredentialVersion
	connector.Status = latest.Status
	connector.LastError = latest.LastError
	connector.BearerTokenPlain = latest.BearerTokenPlain
	connector.RefreshTokenPlain = latest.RefreshTokenPlain
	return nil
}

func (s *UpstreamRelayGroupMonitoringService) runCandidateProbe(ctx context.Context, candidate UpstreamRelayCandidate, account *Account) UpstreamRelayProbeResult {
	result := UpstreamRelayProbeResult{CandidateID: candidate.ID, ProbedAt: time.Now(), ErrorClass: "unknown"}
	if account == nil {
		result.ErrorClass = "account_missing"
		result.ErrorMessage = "account not found"
		return result
	}
	if s.accountTestService == nil {
		result.ErrorClass = "probe_unavailable"
		result.ErrorMessage = "account test service is not configured"
		return result
	}
	if candidate.ProbeProtocol == UpstreamRelayProbeProtocolAnthropic && !account.IsAnthropic() {
		result.ErrorClass = "invalid_request"
		result.ErrorMessage = "anthropic probe_protocol requires an anthropic account"
		return result
	}
	start := time.Now()
	testAccount := accountForUpstreamRelayProbe(account, candidate.ProbeProtocol)
	testResult, err := s.accountTestService.RunAccountTestBackground(ctx, testAccount, candidate.ProbeModel, "answer: 2", AccountTestModeDefault)
	latency := int(time.Since(start) / time.Millisecond)
	result.LatencyMs = &latency
	if err != nil {
		result.ErrorClass = classifyProbeError(0, err.Error())
		result.ErrorMessage = truncateRelayMessage(sanitizeUpstreamRelayError(err.Error()))
		return result
	}
	if testResult == nil {
		result.ErrorClass = "probe_unavailable"
		result.ErrorMessage = "account test returned no result"
		return result
	}
	if testResult.LatencyMs > 0 {
		latency = int(testResult.LatencyMs)
		result.LatencyMs = &latency
	}
	if testResult.Status != "success" {
		message := strings.TrimSpace(testResult.ErrorMessage)
		if message == "" {
			message = "account test failed"
		}
		result.ErrorClass = classifyProbeError(0, message)
		result.ErrorMessage = truncateRelayMessage(sanitizeUpstreamRelayError(message))
		return result
	}
	result.Success = true
	result.ErrorClass = ""
	result.ErrorMessage = ""
	return result
}

func accountForUpstreamRelayProbe(account *Account, protocol string) *Account {
	if account == nil || !account.IsOpenAI() || account.Type != AccountTypeAPIKey {
		return account
	}
	copied := *account
	extra := make(map[string]any, len(account.Extra)+1)
	for key, value := range account.Extra {
		extra[key] = value
	}
	switch protocol {
	case MonitorAPIModeChatCompletions:
		extra[openai_compat.ExtraKeyResponsesMode] = string(openai_compat.ResponsesSupportModeForceChatCompletions)
	case MonitorAPIModeResponses:
		extra[openai_compat.ExtraKeyResponsesMode] = string(openai_compat.ResponsesSupportModeForceResponses)
	}
	copied.Extra = extra
	return &copied
}

func normalizeUpstreamRelayCandidateInput(input UpstreamRelayCandidateInput, id int64, operatorID int64) (*UpstreamRelayCandidate, error) {
	upstreamGroupID := strings.TrimSpace(input.UpstreamGroupID)
	probeModel := strings.TrimSpace(input.ProbeModel)
	protocol := strings.TrimSpace(input.ProbeProtocol)
	if protocol == "" {
		protocol = MonitorAPIModeChatCompletions
	}
	if input.ConnectorID <= 0 || input.AccountID <= 0 || upstreamGroupID == "" || probeModel == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_CANDIDATE", "connector_id, account_id, upstream_group_id and probe_model are required")
	}
	if !isValidUpstreamRelayProbeProtocol(protocol) {
		return nil, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_PROBE_PROTOCOL", "probe_protocol must be chat_completions, responses or anthropic")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var upstreamAPIKeyID *int64
	if input.UpstreamAPIKeyID != nil && *input.UpstreamAPIKeyID > 0 {
		keyID := *input.UpstreamAPIKeyID
		upstreamAPIKeyID = &keyID
	}
	return &UpstreamRelayCandidate{
		ID:                   id,
		ConnectorID:          input.ConnectorID,
		AccountID:            input.AccountID,
		UpstreamGroupID:      upstreamGroupID,
		UpstreamAPIKeyID:     upstreamAPIKeyID,
		UpstreamAPIKeyName:   strings.TrimSpace(input.UpstreamAPIKeyName),
		UpstreamAPIKeyMasked: maskUpstreamRelayAPIKeyForDisplay(input.UpstreamAPIKeyMasked),
		ProbeModel:           probeModel,
		ProbeProtocol:        protocol,
		Enabled:              enabled,
		Notes:                strings.TrimSpace(input.Notes),
		CreatedBy:            operatorID,
	}, nil
}

func isValidUpstreamRelayProbeProtocol(protocol string) bool {
	switch protocol {
	case MonitorAPIModeChatCompletions, MonitorAPIModeResponses, UpstreamRelayProbeProtocolAnthropic:
		return true
	default:
		return false
	}
}

func defaultUpstreamRelayRecommendationPolicy() UpstreamRelayRecommendationPolicy {
	return UpstreamRelayRecommendationPolicy{
		SnapshotFreshnessMinutes:          int(upstreamRelaySnapshotFreshness / time.Minute),
		UsageDeltaFreshnessMinutes:        int(upstreamRelayUsageDeltaFreshness / time.Minute),
		ProbeFreshnessMinutes:             int(upstreamRelayProbeFreshness / time.Minute),
		MinSuccessRate:                    0.5,
		MinSampleSize:                     3,
		ExcludeConsecutiveFailures:        true,
		PriorityStart:                     upstreamRelayDefaultPriorityStart,
		PriorityStep:                      upstreamRelayPriorityStep,
		PauseRateGapEnabled:               false,
		PauseRateGapThreshold:             0.04,
		PauseConsecutiveFailuresEnabled:   false,
		PauseConsecutiveFailuresThreshold: 3,
		PauseSuccessRateEnabled:           false,
		SortFields: []string{
			UpstreamRelaySortRateAsc,
			UpstreamRelaySortSuccessRateDesc,
			UpstreamRelaySortLatencyAsc,
		},
	}
}

func defaultUpstreamRelayMonitoringPolicy() UpstreamRelayMonitoringPolicy {
	return withMonitoringPolicyDerivedFreshness(UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:                 false,
		SyncIntervalMinutes:             upstreamRelayDefaultSyncInterval,
		AutoProbeEnabled:                false,
		ProbeIntervalMinutes:            upstreamRelayDefaultProbeInterval,
		AutoRecommendationEnabled:       false,
		RecommendationIntervalMinutes:   upstreamRelayDefaultRecommendationInterval,
		AutoApplyRecommendationsEnabled: false,
		MaxAutoApplySuggestions:         upstreamRelayDefaultAutoApplySuggestionLimit,
		MaxAutoApplyPriorityDelta:       upstreamRelayDefaultAutoApplyPriorityDelta,
		MinAutoApplyConfidence:          upstreamRelayConfidenceMedium,
		AllowAutoApplyDegradedHealth:    false,
		FailureRetryIntervalMinutes:     upstreamRelayDefaultRetryInterval,
		SyncConcurrency:                 upstreamRelayDefaultSyncLimit,
		ProbeConcurrency:                upstreamRelayDefaultProbeLimit,
	})
}

func monitoringPolicyOrDefault(policy *UpstreamRelayMonitoringPolicy) UpstreamRelayMonitoringPolicy {
	if policy == nil {
		return defaultUpstreamRelayMonitoringPolicy()
	}
	return *policy
}

func policyOrDefault(policy *UpstreamRelayRecommendationPolicy) UpstreamRelayRecommendationPolicy {
	if policy == nil {
		return defaultUpstreamRelayRecommendationPolicy()
	}
	return *policy
}

func normalizeUpstreamRelayMonitoringPolicy(input UpstreamRelayMonitoringPolicy) (UpstreamRelayMonitoringPolicy, error) {
	defaults := defaultUpstreamRelayMonitoringPolicy()
	policy := input
	if policy.SyncIntervalMinutes == 0 {
		policy.SyncIntervalMinutes = defaults.SyncIntervalMinutes
	}
	if policy.ProbeIntervalMinutes == 0 {
		policy.ProbeIntervalMinutes = defaults.ProbeIntervalMinutes
	}
	if policy.RecommendationIntervalMinutes == 0 {
		policy.RecommendationIntervalMinutes = defaults.RecommendationIntervalMinutes
	}
	if policy.MaxAutoApplySuggestions == 0 {
		policy.MaxAutoApplySuggestions = defaults.MaxAutoApplySuggestions
	}
	if strings.TrimSpace(policy.MinAutoApplyConfidence) == "" {
		policy.MinAutoApplyConfidence = defaults.MinAutoApplyConfidence
	}
	if policy.FailureRetryIntervalMinutes == 0 {
		policy.FailureRetryIntervalMinutes = defaults.FailureRetryIntervalMinutes
	}
	if policy.SyncConcurrency == 0 {
		policy.SyncConcurrency = defaults.SyncConcurrency
	}
	if policy.ProbeConcurrency == 0 {
		policy.ProbeConcurrency = defaults.ProbeConcurrency
	}
	if policy.SyncIntervalMinutes < 1 || policy.ProbeIntervalMinutes < 1 || policy.RecommendationIntervalMinutes < 1 || policy.FailureRetryIntervalMinutes < 1 {
		return UpstreamRelayMonitoringPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_MONITORING_INTERVAL", "monitoring intervals must be positive minutes")
	}
	if policy.SyncConcurrency < 1 || policy.ProbeConcurrency < 1 {
		return UpstreamRelayMonitoringPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_MONITORING_CONCURRENCY", "monitoring concurrency limits must be positive")
	}
	if policy.MaxAutoApplySuggestions < 1 || policy.MaxAutoApplyPriorityDelta < 0 {
		return UpstreamRelayMonitoringPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_AUTO_APPLY_LIMIT", "auto-apply limits are invalid")
	}
	policy.MinAutoApplyConfidence = strings.ToLower(strings.TrimSpace(policy.MinAutoApplyConfidence))
	if _, ok := upstreamRelayConfidenceRank(policy.MinAutoApplyConfidence); !ok {
		return UpstreamRelayMonitoringPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_AUTO_APPLY_CONFIDENCE", "min_auto_apply_confidence is unsupported")
	}
	return withMonitoringPolicyDerivedFreshness(policy), nil
}

func withMonitoringPolicyDerivedFreshness(policy UpstreamRelayMonitoringPolicy) UpstreamRelayMonitoringPolicy {
	policy.SnapshotStaleAfterMinutes = policy.SyncIntervalMinutes * 3
	policy.UsageDeltaStaleAfterMinutes = policy.SyncIntervalMinutes * 3
	policy.ProbeStaleAfterMinutes = policy.ProbeIntervalMinutes * 3
	return policy
}

func normalizeUpstreamRelayRecommendationPolicy(input UpstreamRelayRecommendationPolicy) (UpstreamRelayRecommendationPolicy, error) {
	defaults := defaultUpstreamRelayRecommendationPolicy()
	policy := input
	if policy.SnapshotFreshnessMinutes == 0 {
		policy.SnapshotFreshnessMinutes = defaults.SnapshotFreshnessMinutes
	}
	if policy.UsageDeltaFreshnessMinutes == 0 {
		policy.UsageDeltaFreshnessMinutes = defaults.UsageDeltaFreshnessMinutes
	}
	if policy.ProbeFreshnessMinutes == 0 {
		policy.ProbeFreshnessMinutes = defaults.ProbeFreshnessMinutes
	}
	if policy.MinSampleSize == 0 {
		policy.MinSampleSize = defaults.MinSampleSize
	}
	if policy.PriorityStep == 0 {
		policy.PriorityStep = defaults.PriorityStep
	}
	if policy.PauseRateGapThreshold == 0 {
		policy.PauseRateGapThreshold = defaults.PauseRateGapThreshold
	}
	if policy.PauseConsecutiveFailuresThreshold == 0 {
		policy.PauseConsecutiveFailuresThreshold = defaults.PauseConsecutiveFailuresThreshold
	}
	if len(policy.SortFields) == 0 {
		policy.SortFields = append([]string{}, defaults.SortFields...)
	}
	if policy.SnapshotFreshnessMinutes < 1 || policy.UsageDeltaFreshnessMinutes < 1 || policy.ProbeFreshnessMinutes < 1 {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_FRESHNESS", "freshness windows must be positive minutes")
	}
	if policy.MinSuccessRate < 0 || policy.MinSuccessRate > 1 || math.IsNaN(policy.MinSuccessRate) || math.IsInf(policy.MinSuccessRate, 0) {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_SUCCESS_RATE", "min_success_rate must be between 0 and 1")
	}
	if policy.MinSampleSize < 1 {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_SAMPLE_SIZE", "min_sample_size must be positive")
	}
	if policy.PriorityStep < 1 {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_PRIORITY_STEP", "priority_step must be positive")
	}
	if policy.PauseRateGapThreshold <= 0 || math.IsNaN(policy.PauseRateGapThreshold) || math.IsInf(policy.PauseRateGapThreshold, 0) {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_PAUSE_RATE_GAP", "pause_rate_gap_threshold must be positive")
	}
	if policy.PauseConsecutiveFailuresThreshold < 1 {
		return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_PAUSE_FAILURES", "pause_consecutive_failures_threshold must be positive")
	}
	seen := map[string]bool{}
	for _, field := range policy.SortFields {
		if seen[field] {
			return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_SORT_FIELDS", "sort_fields must not contain duplicates")
		}
		seen[field] = true
		switch field {
		case UpstreamRelaySortRateAsc, UpstreamRelaySortSuccessRateDesc, UpstreamRelaySortLatencyAsc:
		default:
			return UpstreamRelayRecommendationPolicy{}, infraerrors.BadRequest("UPSTREAM_RELAY_INVALID_POLICY_SORT_FIELDS", "sort_fields contains unsupported field")
		}
	}
	return policy, nil
}

func buildUpstreamRelaySuggestions(candidates []UpstreamRelayCandidate) []UpstreamRelayRecommendationSuggestion {
	return buildUpstreamRelayRecommendationPreview(candidates, defaultUpstreamRelayRecommendationPolicy()).Suggestions
}

type upstreamRelayCandidateEvaluation struct {
	candidate UpstreamRelayCandidate
	exclusion UpstreamRelayRecommendationExclusion
	excluded  bool
}

type upstreamRelayAccountPauseEvidence struct {
	ReasonCode          string
	Reason              string
	FinalRateMultiplier float64
	HealthStatus        string
	Confidence          string
	HealthSummary       string
	RateSource          string
}

func buildUpstreamRelayRecommendationPreview(candidates []UpstreamRelayCandidate, policy UpstreamRelayRecommendationPolicy) UpstreamRelayRecommendationPreview {
	normalized, err := normalizeUpstreamRelayRecommendationPolicy(policy)
	if err != nil {
		normalized = defaultUpstreamRelayRecommendationPolicy()
	}
	now := time.Now()
	eligible := make([]UpstreamRelayCandidate, 0, len(candidates))
	exclusions := make([]UpstreamRelayRecommendationExclusion, 0)
	accountGateSuggestions := make([]UpstreamRelayRecommendationSuggestion, 0)
	schedulableByPlatform := countSchedulableRelayAccountsByPlatform(candidates)
	evaluations := make([]upstreamRelayCandidateEvaluation, 0, len(candidates))
	accountHasEligibleCandidate := map[int64]struct{}{}
	seenGateAccounts := map[int64]struct{}{}
	for _, candidate := range candidates {
		if exclusion, ok := evaluateUpstreamRelayCandidateExclusion(candidate, now, normalized); ok {
			evaluations = append(evaluations, upstreamRelayCandidateEvaluation{candidate: candidate, exclusion: exclusion, excluded: true})
			continue
		}
		if exclusion, ok := evaluateUpstreamRelayCandidatePauseExclusion(candidate, now, normalized); ok {
			evaluations = append(evaluations, upstreamRelayCandidateEvaluation{candidate: candidate, exclusion: exclusion, excluded: true})
			continue
		}
		if suggestion, ok := buildRelayAccountGateResumeSuggestion(candidate, now, normalized); ok {
			if _, seen := seenGateAccounts[suggestion.AccountID]; !seen {
				accountGateSuggestions = append(accountGateSuggestions, suggestion)
				seenGateAccounts[suggestion.AccountID] = struct{}{}
			}
			continue
		}
		eligible = append(eligible, candidate)
		if candidate.AccountSchedulable && !candidate.AccountGateActive {
			accountHasEligibleCandidate[candidate.AccountID] = struct{}{}
		}
		evaluations = append(evaluations, upstreamRelayCandidateEvaluation{candidate: candidate})
	}
	if normalized.PauseRateGapEnabled {
		for _, suggestion := range buildRelayRateGapPauseSuggestions(eligible, normalized, schedulableByPlatform, now) {
			if _, seen := seenGateAccounts[suggestion.AccountID]; !seen {
				accountGateSuggestions = append(accountGateSuggestions, suggestion)
				seenGateAccounts[suggestion.AccountID] = struct{}{}
			}
		}
	}
	for _, evaluation := range evaluations {
		if !evaluation.excluded {
			continue
		}
		if _, hasEligible := accountHasEligibleCandidate[evaluation.candidate.AccountID]; hasEligible {
			exclusions = append(exclusions, evaluation.exclusion)
			continue
		}
		if evidence, ok := buildRelayAccountGatePauseEvidence(evaluation.candidate, evaluation.exclusion, normalized, now); ok {
			suggestion, ok := buildRelayAccountGatePauseSuggestion(evaluation.candidate, evidence, schedulableByPlatform)
			if !ok {
				exclusions = append(exclusions, evaluation.exclusion)
				continue
			}
			if _, seen := seenGateAccounts[suggestion.AccountID]; !seen {
				accountGateSuggestions = append(accountGateSuggestions, suggestion)
				seenGateAccounts[suggestion.AccountID] = struct{}{}
				continue
			}
		}
		exclusions = append(exclusions, evaluation.exclusion)
	}
	sortRelayCandidatesWithPolicy(eligible, normalized, now)
	suggestions := make([]UpstreamRelayRecommendationSuggestion, 0, len(eligible))
	seenAccounts := make(map[int64]struct{}, len(eligible))
	uniqueRank := 0
	for _, candidate := range eligible {
		if _, ok := seenGateAccounts[candidate.AccountID]; ok {
			exclusions = append(exclusions, buildUpstreamRelayExclusion(
				candidate,
				now,
				normalized,
				"account_gate_suggestion_exists",
				"同一账号已有暂停或恢复账号承接建议，避免同一轮同时生成 priority 调整",
				nil,
			))
			continue
		}
		if _, ok := seenAccounts[candidate.AccountID]; ok {
			exclusions = append(exclusions, buildUpstreamRelayExclusion(
				candidate,
				now,
				normalized,
				"duplicate_account_candidate",
				"同一账号已有排序更优的候选生成 priority 建议，避免对同一账号生成多条 priority 建议",
				nil,
			))
			continue
		}
		seenAccounts[candidate.AccountID] = struct{}{}
		newPriority := normalized.PriorityStart + uniqueRank*normalized.PriorityStep
		uniqueRank++
		if candidate.CurrentPriority != nil && *candidate.CurrentPriority == newPriority {
			expected := newPriority
			exclusions = append(exclusions, buildUpstreamRelayExclusion(candidate, now, normalized, "priority_unchanged", fmt.Sprintf("当前 priority 已是策略建议值 %d，无需调整", newPriority), &expected))
			continue
		}
		rate, rateSource, _ := effectiveRelayRateWithPolicy(candidate, now, normalized)
		healthSummary := relayHealthSummary(candidate)
		suggestions = append(suggestions, UpstreamRelayRecommendationSuggestion{
			ActionType:          UpstreamRelaySuggestionActionPriorityUpdate,
			CandidateID:         candidate.ID,
			ConnectorID:         candidate.ConnectorID,
			ConnectorName:       candidate.ConnectorName,
			AccountID:           candidate.AccountID,
			AccountName:         candidate.AccountName,
			UpstreamGroupID:     candidate.UpstreamGroupID,
			UpstreamGroupName:   candidate.UpstreamGroupName,
			OldPriority:         candidate.CurrentPriority,
			NewPriority:         intPtr(newPriority),
			FinalRateMultiplier: rate,
			HealthStatus:        relayHealthStatus(candidate),
			ReasonCode:          "rate_health_priority",
			Confidence:          relayRateConfidence(rateSource),
			HealthSummary:       healthSummary,
			RateSource:          rateSource,
			Reason:              fmt.Sprintf("上游倍率 %.4g，来源 %s，%s，按策略排序建议 priority=%d", rate, rateSource, healthSummary, newPriority),
		})
	}
	suggestions = append(suggestions, accountGateSuggestions...)
	return UpstreamRelayRecommendationPreview{
		Policy:          normalized,
		TotalCandidates: len(candidates),
		SuggestionCount: len(suggestions),
		ExcludedCount:   len(exclusions),
		Suggestions:     suggestions,
		Exclusions:      exclusions,
	}
}

func validateUpstreamRelayAutoApplyRun(run *UpstreamRelayRecommendationRun, policy UpstreamRelayMonitoringPolicy) string {
	if run == nil {
		return "run_missing"
	}
	if run.Status != UpstreamRelayRunStatusSuccess {
		return "run_not_success"
	}
	if run.Applied {
		return "run_already_applied"
	}
	if len(run.Suggestions) == 0 {
		return "no_suggestions"
	}
	if len(run.Suggestions) > policy.MaxAutoApplySuggestions {
		return "too_many_suggestions"
	}
	minConfidenceRank, ok := upstreamRelayConfidenceRank(policy.MinAutoApplyConfidence)
	if !ok {
		return "invalid_min_confidence"
	}
	for _, suggestion := range run.Suggestions {
		if relaySuggestionActionOrDefault(suggestion.ActionType) != UpstreamRelaySuggestionActionPriorityUpdate {
			return "account_gate_suggestion_requires_manual_apply"
		}
		if suggestion.NewPriority == nil {
			return "invalid_priority_suggestion"
		}
		if upstreamRelayPriorityDelta(suggestion.OldPriority, *suggestion.NewPriority) > policy.MaxAutoApplyPriorityDelta {
			return "priority_delta_exceeded"
		}
		confidenceRank, ok := upstreamRelayConfidenceRank(suggestion.Confidence)
		if !ok || confidenceRank < minConfidenceRank {
			return "confidence_below_threshold"
		}
		if !policy.AllowAutoApplyDegradedHealth && suggestion.HealthStatus == upstreamRelayHealthDegraded {
			return "degraded_health"
		}
	}
	return ""
}

func countSchedulableRelayAccountsByPlatform(candidates []UpstreamRelayCandidate) map[string]int {
	seen := map[int64]struct{}{}
	out := map[string]int{}
	for _, candidate := range candidates {
		if !candidate.AccountSchedulable {
			continue
		}
		if _, ok := seen[candidate.AccountID]; ok {
			continue
		}
		seen[candidate.AccountID] = struct{}{}
		out[candidate.AccountPlatform]++
	}
	return out
}

func buildRelayRateGapPauseSuggestions(eligible []UpstreamRelayCandidate, policy UpstreamRelayRecommendationPolicy, schedulableByPlatform map[string]int, now time.Time) []UpstreamRelayRecommendationSuggestion {
	byAccount := map[int64][]UpstreamRelayCandidate{}
	for _, candidate := range eligible {
		if !candidate.AccountSchedulable || candidate.AccountGateActive {
			continue
		}
		byAccount[candidate.AccountID] = append(byAccount[candidate.AccountID], candidate)
	}
	suggestions := make([]UpstreamRelayRecommendationSuggestion, 0)
	for accountID, accountCandidates := range byAccount {
		if len(accountCandidates) == 0 || schedulableByPlatform[accountCandidates[0].AccountPlatform] <= 1 {
			continue
		}
		var selectedCandidate UpstreamRelayCandidate
		var selectedReplacement UpstreamRelayCandidate
		selectedRate := 0.0
		selectedReplacementRate := 0.0
		selectedReplacementSource := ""
		maxGap := 0.0
		allCovered := true
		for _, candidate := range accountCandidates {
			rate, _, ok := effectiveRelayRateWithPolicy(candidate, now, policy)
			if !ok {
				allCovered = false
				break
			}
			replacement, replacementRate, replacementSource, ok := findRelayRateGapReplacement(candidate, rate, eligible, policy, now)
			if !ok {
				allCovered = false
				break
			}
			gap := rate - replacementRate
			if selectedCandidate.ID == 0 || gap > maxGap {
				selectedCandidate = candidate
				selectedReplacement = replacement
				selectedRate = rate
				selectedReplacementRate = replacementRate
				selectedReplacementSource = replacementSource
				maxGap = gap
			}
		}
		if !allCovered || selectedCandidate.ID == 0 {
			continue
		}
		rateSource := ""
		if selectedCandidate.LatestSnapshot != nil {
			rateSource = selectedCandidate.LatestSnapshot.Source
		}
		if rateSource == "" {
			_, rateSource, _ = effectiveRelayRateWithPolicy(selectedCandidate, now, policy)
		}
		reason := fmt.Sprintf("账号所有健康候选均可由跨账号低倍率候选替代；当前候选 %s 倍率 %.4g，最低可替代候选 #%d %s 倍率 %.4g，差值 %.4g >= 策略阈值 %.4g，建议暂停账号承接",
			relayCandidateMappingForReason(selectedCandidate),
			selectedRate,
			selectedReplacement.AccountID,
			relayCandidateMappingForReason(selectedReplacement),
			selectedReplacementRate,
			maxGap,
			policy.PauseRateGapThreshold,
		)
		if selectedReplacementSource != "" {
			reason = fmt.Sprintf("%s；替代倍率来源 %s", reason, selectedReplacementSource)
		}
		evidence := upstreamRelayAccountPauseEvidence{
			ReasonCode:          "rate_gap_exceeded",
			Reason:              reason,
			FinalRateMultiplier: selectedRate,
			HealthStatus:        relayHealthStatus(selectedCandidate),
			Confidence:          relayRateConfidence(rateSource),
			HealthSummary:       relayHealthSummary(selectedCandidate),
			RateSource:          rateSource,
		}
		if suggestion, ok := buildRelayAccountGatePauseSuggestion(selectedCandidate, evidence, schedulableByPlatform); ok {
			suggestion.AccountID = accountID
			suggestions = append(suggestions, suggestion)
		}
	}
	return suggestions
}

func findRelayRateGapReplacement(candidate UpstreamRelayCandidate, candidateRate float64, eligible []UpstreamRelayCandidate, policy UpstreamRelayRecommendationPolicy, now time.Time) (UpstreamRelayCandidate, float64, string, bool) {
	var best UpstreamRelayCandidate
	bestRate := math.MaxFloat64
	bestSource := ""
	for _, replacement := range eligible {
		if replacement.AccountID == candidate.AccountID ||
			replacement.AccountPlatform != candidate.AccountPlatform ||
			replacement.ProbeProtocol != candidate.ProbeProtocol ||
			replacement.ProbeModel != candidate.ProbeModel {
			continue
		}
		rate, source, ok := effectiveRelayRateWithPolicy(replacement, now, policy)
		if !ok {
			continue
		}
		if candidateRate-rate < policy.PauseRateGapThreshold {
			continue
		}
		if rate < bestRate {
			best = replacement
			bestRate = rate
			bestSource = source
		}
	}
	if best.ID == 0 {
		return UpstreamRelayCandidate{}, 0, "", false
	}
	return best, bestRate, bestSource, true
}

func relayCandidateMappingForReason(candidate UpstreamRelayCandidate) string {
	group := strings.TrimSpace(candidate.UpstreamGroupName)
	if group == "" {
		group = strings.TrimSpace(candidate.UpstreamGroupID)
	}
	if group == "" {
		group = fmt.Sprintf("candidate #%d", candidate.ID)
	}
	return group
}

func buildRelayAccountGatePauseSuggestion(candidate UpstreamRelayCandidate, evidence upstreamRelayAccountPauseEvidence, schedulableByPlatform map[string]int) (UpstreamRelayRecommendationSuggestion, bool) {
	if !candidate.AccountSchedulable || candidate.AccountGateActive {
		return UpstreamRelayRecommendationSuggestion{}, false
	}
	if schedulableByPlatform[candidate.AccountPlatform] <= 1 {
		return UpstreamRelayRecommendationSuggestion{}, false
	}
	return UpstreamRelayRecommendationSuggestion{
		ActionType:          UpstreamRelaySuggestionActionAccountPause,
		CandidateID:         candidate.ID,
		ConnectorID:         candidate.ConnectorID,
		ConnectorName:       candidate.ConnectorName,
		AccountID:           candidate.AccountID,
		AccountName:         candidate.AccountName,
		UpstreamGroupID:     candidate.UpstreamGroupID,
		UpstreamGroupName:   candidate.UpstreamGroupName,
		OldPriority:         candidate.CurrentPriority,
		OldSchedulable:      boolPtr(true),
		NewSchedulable:      boolPtr(false),
		FinalRateMultiplier: evidence.FinalRateMultiplier,
		HealthStatus:        evidence.HealthStatus,
		ReasonCode:          "account_gate_" + evidence.ReasonCode,
		Confidence:          evidence.Confidence,
		HealthSummary:       evidence.HealthSummary,
		RateSource:          evidence.RateSource,
		Reason:              evidence.Reason,
	}, true
}

func buildRelayAccountGatePauseEvidence(candidate UpstreamRelayCandidate, exclusion UpstreamRelayRecommendationExclusion, policy UpstreamRelayRecommendationPolicy, now time.Time) (upstreamRelayAccountPauseEvidence, bool) {
	if candidate.Health == nil {
		return upstreamRelayAccountPauseEvidence{}, false
	}
	rate, rateSource, confidence := relayPauseEvidenceRate(candidate, exclusion, policy, now)
	healthSummary := relayHealthSummary(candidate)
	if policy.PauseConsecutiveFailuresEnabled && candidate.Health.ConsecutiveFailures >= policy.PauseConsecutiveFailuresThreshold {
		reason := fmt.Sprintf("连续失败 %d 次，达到策略阈值 %d；%s；建议暂停账号承接", candidate.Health.ConsecutiveFailures, policy.PauseConsecutiveFailuresThreshold, relayLatestProbeFailureSummary(candidate))
		return upstreamRelayAccountPauseEvidence{
			ReasonCode:          "consecutive_failures",
			Reason:              reason,
			FinalRateMultiplier: rate,
			HealthStatus:        relayHealthStatus(candidate),
			Confidence:          confidence,
			HealthSummary:       healthSummary,
			RateSource:          rateSource,
		}, true
	}
	if policy.PauseSuccessRateEnabled && candidate.Health.ProbeCount >= policy.MinSampleSize && candidate.Health.SuccessRate < policy.MinSuccessRate {
		reason := fmt.Sprintf("样本数 %d 已达到门槛 %d，成功率 %.0f%% 低于策略 %.0f%%；建议暂停账号承接", candidate.Health.ProbeCount, policy.MinSampleSize, candidate.Health.SuccessRate*100, policy.MinSuccessRate*100)
		return upstreamRelayAccountPauseEvidence{
			ReasonCode:          "success_rate_below_threshold",
			Reason:              reason,
			FinalRateMultiplier: rate,
			HealthStatus:        relayHealthStatus(candidate),
			Confidence:          confidence,
			HealthSummary:       healthSummary,
			RateSource:          rateSource,
		}, true
	}
	return upstreamRelayAccountPauseEvidence{}, false
}

func relayPauseEvidenceRate(candidate UpstreamRelayCandidate, exclusion UpstreamRelayRecommendationExclusion, policy UpstreamRelayRecommendationPolicy, now time.Time) (float64, string, string) {
	if exclusion.FinalRateMultiplier != nil {
		return *exclusion.FinalRateMultiplier, exclusion.RateSource, exclusion.Confidence
	}
	if rate, source, ok := effectiveRelayRateWithPolicy(candidate, now, policy); ok {
		return rate, source, relayRateConfidence(source)
	}
	return 0, "", upstreamRelayConfidenceUnknown
}

func relayLatestProbeFailureSummary(candidate UpstreamRelayCandidate) string {
	if candidate.LatestProbe == nil {
		return "最近一次探测不存在"
	}
	if strings.TrimSpace(candidate.LatestProbe.ErrorMessage) != "" {
		return fmt.Sprintf("最近错误：%s", candidate.LatestProbe.ErrorMessage)
	}
	if strings.TrimSpace(candidate.LatestProbe.ErrorClass) != "" {
		return fmt.Sprintf("最近错误类型：%s", candidate.LatestProbe.ErrorClass)
	}
	return "最近一次探测未成功"
}

func buildRelayAccountGateResumeSuggestion(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) (UpstreamRelayRecommendationSuggestion, bool) {
	if candidate.AccountSchedulable || !candidate.AccountGateActive {
		return UpstreamRelayRecommendationSuggestion{}, false
	}
	if _, ok := evaluateUpstreamRelayCandidateExclusion(candidate, now, policy); ok {
		return UpstreamRelayRecommendationSuggestion{}, false
	}
	rate, rateSource, _ := effectiveRelayRateWithPolicy(candidate, now, policy)
	healthSummary := relayHealthSummary(candidate)
	return UpstreamRelayRecommendationSuggestion{
		ActionType:          UpstreamRelaySuggestionActionAccountResume,
		CandidateID:         candidate.ID,
		ConnectorID:         candidate.ConnectorID,
		ConnectorName:       candidate.ConnectorName,
		AccountID:           candidate.AccountID,
		AccountName:         candidate.AccountName,
		UpstreamGroupID:     candidate.UpstreamGroupID,
		UpstreamGroupName:   candidate.UpstreamGroupName,
		OldPriority:         candidate.CurrentPriority,
		OldSchedulable:      boolPtr(false),
		NewSchedulable:      boolPtr(true),
		FinalRateMultiplier: rate,
		HealthStatus:        relayHealthStatus(candidate),
		ReasonCode:          "account_gate_recovered",
		Confidence:          relayRateConfidence(rateSource),
		HealthSummary:       healthSummary,
		RateSource:          rateSource,
		Reason:              fmt.Sprintf("账号由流量闸门暂停，当前上游倍率 %.4g，来源 %s，%s，建议恢复账号承接", rate, rateSource, healthSummary),
	}, true
}

func relaySuggestionActionOrDefault(action string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		return UpstreamRelaySuggestionActionPriorityUpdate
	}
	return action
}

func upstreamRelayPriorityDelta(oldPriority *int, newPriority int) int {
	if oldPriority == nil {
		if newPriority < 0 {
			return -newPriority
		}
		return newPriority
	}
	delta := newPriority - *oldPriority
	if delta < 0 {
		return -delta
	}
	return delta
}

func upstreamRelayConfidenceRank(confidence string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case upstreamRelayConfidenceUnknown:
		return 0, true
	case upstreamRelayConfidenceLow:
		return 1, true
	case upstreamRelayConfidenceMedium:
		return 2, true
	case upstreamRelayConfidenceHigh:
		return 3, true
	default:
		return 0, false
	}
}

func evaluateUpstreamRelayCandidateExclusion(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) (UpstreamRelayRecommendationExclusion, bool) {
	if !candidate.Enabled || candidate.ConnectorStatus != UpstreamRelayConnectorStatusActive {
		reason := "候选未启用或连接器不是 active 状态"
		return buildUpstreamRelayExclusion(candidate, now, policy, "inactive_candidate", reason, nil), true
	}
	if _, _, ok := effectiveRelayRateWithPolicy(candidate, now, policy); !ok {
		reason := fmt.Sprintf("缺少新鲜有效倍率，登录倍率需在 %d 分钟内，用量推导倍率需在 %d 分钟内", policy.SnapshotFreshnessMinutes, policy.UsageDeltaFreshnessMinutes)
		return buildUpstreamRelayExclusion(candidate, now, policy, "missing_fresh_rate", reason, nil), true
	}
	if candidate.LatestProbe == nil || !candidate.LatestProbe.Success {
		return buildUpstreamRelayExclusion(candidate, now, policy, "latest_probe_failed", "最近一次探测不存在或未成功", nil), true
	}
	if now.Sub(candidate.LatestProbe.ProbedAt) > time.Duration(policy.ProbeFreshnessMinutes)*time.Minute {
		return buildUpstreamRelayExclusion(candidate, now, policy, "stale_probe", fmt.Sprintf("最近探测超过 %d 分钟新鲜度窗口", policy.ProbeFreshnessMinutes), nil), true
	}
	if candidate.Health != nil {
		if policy.ExcludeConsecutiveFailures && candidate.Health.ConsecutiveFailures > 0 {
			return buildUpstreamRelayExclusion(candidate, now, policy, "consecutive_failures", "健康窗口内存在连续失败，策略要求排除", nil), true
		}
		if candidate.Health.ProbeCount >= policy.MinSampleSize && candidate.Health.SuccessRate < policy.MinSuccessRate {
			return buildUpstreamRelayExclusion(candidate, now, policy, "success_rate_below_threshold", fmt.Sprintf("样本数 %d 已达到门槛 %d，但成功率 %.0f%% 低于策略 %.0f%%", candidate.Health.ProbeCount, policy.MinSampleSize, candidate.Health.SuccessRate*100, policy.MinSuccessRate*100), nil), true
		}
	}
	return UpstreamRelayRecommendationExclusion{}, false
}

func evaluateUpstreamRelayCandidatePauseExclusion(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) (UpstreamRelayRecommendationExclusion, bool) {
	if candidate.Health == nil {
		return UpstreamRelayRecommendationExclusion{}, false
	}
	if policy.PauseConsecutiveFailuresEnabled && candidate.Health.ConsecutiveFailures >= policy.PauseConsecutiveFailuresThreshold {
		return buildUpstreamRelayExclusion(
			candidate,
			now,
			policy,
			"consecutive_failures",
			fmt.Sprintf("连续失败 %d 次，达到策略阈值 %d", candidate.Health.ConsecutiveFailures, policy.PauseConsecutiveFailuresThreshold),
			nil,
		), true
	}
	return UpstreamRelayRecommendationExclusion{}, false
}

func buildUpstreamRelayExclusion(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy, reasonCode string, reason string, expectedPriority *int) UpstreamRelayRecommendationExclusion {
	rate, rateSource, hasRate := effectiveRelayRateWithPolicy(candidate, now, policy)
	var ratePtr *float64
	confidence := ""
	if hasRate {
		rateValue := rate
		ratePtr = &rateValue
		confidence = relayRateConfidence(rateSource)
	}
	return UpstreamRelayRecommendationExclusion{
		CandidateID:         candidate.ID,
		ConnectorID:         candidate.ConnectorID,
		ConnectorName:       candidate.ConnectorName,
		AccountID:           candidate.AccountID,
		AccountName:         candidate.AccountName,
		UpstreamGroupID:     candidate.UpstreamGroupID,
		UpstreamGroupName:   candidate.UpstreamGroupName,
		OldPriority:         candidate.CurrentPriority,
		ExpectedPriority:    expectedPriority,
		FinalRateMultiplier: ratePtr,
		RateSource:          rateSource,
		Confidence:          confidence,
		HealthStatus:        relayHealthStatus(candidate),
		HealthSummary:       relayHealthSummary(candidate),
		ReasonCode:          reasonCode,
		Reason:              reason,
	}
}

func sortRelayCandidates(candidates []UpstreamRelayCandidate) {
	sortRelayCandidatesWithPolicy(candidates, defaultUpstreamRelayRecommendationPolicy(), time.Now())
}

func sortRelayCandidatesWithPolicy(candidates []UpstreamRelayCandidate, policy UpstreamRelayRecommendationPolicy, now time.Time) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		for _, field := range policy.SortFields {
			switch field {
			case UpstreamRelaySortRateAsc:
				leftRate, rightRate := math.MaxFloat64, math.MaxFloat64
				if rate, _, ok := effectiveRelayRateWithPolicy(left, now, policy); ok {
					leftRate = rate
				}
				if rate, _, ok := effectiveRelayRateWithPolicy(right, now, policy); ok {
					rightRate = rate
				}
				if leftRate != rightRate {
					return leftRate < rightRate
				}
			case UpstreamRelaySortSuccessRateDesc:
				leftSuccess, rightSuccess := relaySuccessRate(left), relaySuccessRate(right)
				if leftSuccess != rightSuccess {
					return leftSuccess > rightSuccess
				}
			case UpstreamRelaySortLatencyAsc:
				leftLatency, rightLatency := relayP95Latency(left), relayP95Latency(right)
				if leftLatency != rightLatency {
					return leftLatency < rightLatency
				}
			}
		}
		return left.ID < right.ID
	})
}

func effectiveRelayRate(candidate UpstreamRelayCandidate, now time.Time) (float64, string, bool) {
	return effectiveRelayRateWithPolicy(candidate, now, defaultUpstreamRelayRecommendationPolicy())
}

func effectiveRelayRateWithPolicy(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) (float64, string, bool) {
	if hasFreshLoginRelayRateWithPolicy(candidate, now, policy) {
		source := candidate.LatestSnapshot.Source
		if source == "" {
			source = UpstreamRelayRateSourceAvailable
		}
		return candidate.LatestSnapshot.FinalRateMultiplier, source, true
	}
	if hasFreshReliableUsageDeltaWithPolicy(candidate, now, policy) {
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
	return hasFreshLoginRelayRateWithPolicy(candidate, now, defaultUpstreamRelayRecommendationPolicy())
}

func hasFreshLoginRelayRateWithPolicy(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) bool {
	return candidate.LatestSnapshot != nil &&
		candidate.LatestSnapshot.Status != upstreamRelaySnapshotStatusStale &&
		now.Sub(candidate.LatestSnapshot.LastSeenAt) <= time.Duration(policy.SnapshotFreshnessMinutes)*time.Minute
}

func hasFreshReliableUsageDelta(candidate UpstreamRelayCandidate, now time.Time) bool {
	return hasFreshReliableUsageDeltaWithPolicy(candidate, now, defaultUpstreamRelayRecommendationPolicy())
}

func hasFreshReliableUsageDeltaWithPolicy(candidate UpstreamRelayCandidate, now time.Time, policy UpstreamRelayRecommendationPolicy) bool {
	return candidate.LatestUsageDelta != nil &&
		candidate.LatestUsageDelta.Status == "reliable" &&
		candidate.LatestUsageDelta.DerivedRateMultiplier != nil &&
		now.Sub(candidate.LatestUsageDelta.SampledAt) <= time.Duration(policy.UsageDeltaFreshnessMinutes)*time.Minute
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

func parseUpstreamAccountBalance(body []byte) (*float64, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse upstream account balance: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return nil, fmt.Errorf("upstream profile response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	if user, ok := root["user"].(map[string]any); ok {
		if balance, ok := relayFloatOK(firstPresent(user, "balance", "account_balance")); ok {
			return &balance, nil
		}
	}
	if balance, ok := relayFloatOK(firstPresent(root, "balance", "account_balance")); ok {
		return &balance, nil
	}
	return nil, fmt.Errorf("upstream profile response missing balance")
}

type upstreamRelayUsagePageItem struct {
	GroupID     string
	ActualCost  float64
	TotalTokens int64
}

type upstreamRelayAPIKeyPageItem struct {
	ID      int64
	Name    string
	Key     string
	GroupID string
}

func maskUpstreamRelayAPIKeyForDisplay(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.Contains(key, "***") {
		return key
	}
	runes := []rune(key)
	if len(runes) <= 8 {
		return "***"
	}
	head := 4
	tail := 4
	if strings.HasPrefix(key, "sk-") && len(runes) > 10 {
		head = 6
	}
	if len(runes) <= head+tail {
		return string(runes[:head]) + "***"
	}
	return string(runes[:head]) + "***" + string(runes[len(runes)-tail:])
}

func sanitizeUpstreamRelayCandidateAPIKeyDisplays(items []UpstreamRelayCandidate) {
	for i := range items {
		sanitizeUpstreamRelayCandidateAPIKeyDisplay(&items[i])
	}
}

func sanitizeUpstreamRelayCandidateAPIKeyDisplay(candidate *UpstreamRelayCandidate) {
	if candidate == nil {
		return
	}
	candidate.UpstreamAPIKeyMasked = maskUpstreamRelayAPIKeyForDisplay(candidate.UpstreamAPIKeyMasked)
}

func parseUpstreamAPIKeyPage(body []byte) ([]upstreamRelayAPIKeyPageItem, int, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, 0, fmt.Errorf("parse upstream api keys page: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return nil, 0, fmt.Errorf("upstream api keys response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	pages := int(relayFloat(firstPresent(root, "pages", "total_pages"), 1))
	rawItems := extractRelayArray(root)
	out := make([]upstreamRelayAPIKeyPageItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		m, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		id := relayInt64(m["id"])
		name := strings.TrimSpace(relayString(firstPresent(m, "name", "key_name", "label")))
		key := strings.TrimSpace(relayString(m["key"]))
		groupID := strings.TrimSpace(relayString(firstPresent(m, "group_id", "groupId")))
		if groupID == "" {
			if group, ok := m["group"].(map[string]any); ok {
				groupID = strings.TrimSpace(relayString(firstPresent(group, "id", "group_id")))
			}
		}
		if id <= 0 || (key == "" && name == "") {
			continue
		}
		out = append(out, upstreamRelayAPIKeyPageItem{ID: id, Name: name, Key: key, GroupID: groupID})
	}
	return out, pages, nil
}

func parseUpstreamUsageStats(body []byte) (UpstreamRelayGroupTodayUsage, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return UpstreamRelayGroupTodayUsage{}, fmt.Errorf("parse upstream usage stats: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return UpstreamRelayGroupTodayUsage{}, fmt.Errorf("upstream usage stats response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	actualCost, ok := relayFloatOK(firstPresent(root, "total_actual_cost", "actual_cost"))
	if !ok {
		return UpstreamRelayGroupTodayUsage{}, fmt.Errorf("upstream usage stats missing total_actual_cost")
	}
	totalTokens, ok := relayInt64OK(firstPresent(root, "total_tokens"))
	if !ok {
		inputTokens := relayInt64(firstPresent(root, "total_input_tokens", "input_tokens"))
		outputTokens := relayInt64(firstPresent(root, "total_output_tokens", "output_tokens"))
		cacheCreationTokens := relayInt64(firstPresent(root, "total_cache_creation_tokens", "cache_creation_tokens"))
		cacheReadTokens := relayInt64(firstPresent(root, "total_cache_read_tokens", "cache_read_tokens"))
		totalTokens = inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens
	}
	return UpstreamRelayGroupTodayUsage{ActualCost: actualCost, TotalTokens: totalTokens}, nil
}

func parseUpstreamUsagePage(body []byte) ([]upstreamRelayUsagePageItem, int, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, 0, fmt.Errorf("parse upstream usage page: %w", err)
	}
	root, _ := raw.(map[string]any)
	if root == nil {
		return nil, 0, fmt.Errorf("upstream usage response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	pages := int(relayFloat(firstPresent(root, "pages", "total_pages"), 1))
	rawItems := extractRelayArray(root)
	out := make([]upstreamRelayUsagePageItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		m, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		groupID := relayString(m["group_id"])
		if groupID == "" {
			return nil, 0, fmt.Errorf("upstream usage item missing group_id")
		}
		actualCost, ok := relayFloatOK(m["actual_cost"])
		if !ok {
			return nil, 0, fmt.Errorf("upstream usage item missing actual_cost")
		}
		inputTokens, inputOK := relayInt64OK(m["input_tokens"])
		outputTokens, outputOK := relayInt64OK(m["output_tokens"])
		cacheCreationTokens, cacheCreationOK := relayInt64OK(m["cache_creation_tokens"])
		cacheReadTokens, cacheReadOK := relayInt64OK(m["cache_read_tokens"])
		if !inputOK || !outputOK || !cacheCreationOK || !cacheReadOK {
			return nil, 0, fmt.Errorf("upstream usage item missing token fields")
		}
		out = append(out, upstreamRelayUsagePageItem{
			GroupID:     groupID,
			ActualCost:  actualCost,
			TotalTokens: inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
		})
	}
	return out, pages, nil
}

type upstreamRelayUsageSnapshot struct {
	Cost       float64
	ActualCost float64
}

func (s *UpstreamRelayGroupMonitoringService) fetchUsageSnapshotForAccount(ctx context.Context, account *Account) (*upstreamRelayUsageSnapshot, error) {
	if account == nil {
		return nil, fmt.Errorf("account not found")
	}
	apiKey := upstreamRelayAPIKeyFromAccount(account)
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

func upstreamRelayAPIKeyFromAccount(account *Account) string {
	if account == nil {
		return ""
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" && account.IsOpenAI() {
		apiKey = strings.TrimSpace(account.GetOpenAIAccessToken())
	}
	return apiKey
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

func stringPointerValue(value *string) string {
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

func relayInt64OK(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i, true
		}
		f, err := v.Float64()
		return int64(f), err == nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return i, true
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return int64(f), err == nil
	default:
		return 0, false
	}
}

func relayInt64(value any) int64 {
	out, _ := relayInt64OK(value)
	return out
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
