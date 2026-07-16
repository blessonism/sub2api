package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type upstreamRelayTestEncryptor struct{}

func (upstreamRelayTestEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (upstreamRelayTestEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

type upstreamRelayRecommendationServiceRepo struct {
	UpstreamRelayRepository

	candidates  []UpstreamRelayCandidate
	connector   *UpstreamRelayConnector
	snapshots   []UpstreamRelayGroupRateSnapshot
	policy      *UpstreamRelayRecommendationPolicy
	monitoring  *UpstreamRelayMonitoringPolicy
	createdRuns int
	appliedRuns int
	applyErr    error
}

func (r *upstreamRelayRecommendationServiceRepo) GetConnector(_ context.Context, id int64) (*UpstreamRelayConnector, error) {
	if r.connector == nil || r.connector.ID != id {
		return nil, ErrUpstreamRelayConnectorNotFound
	}
	copy := *r.connector
	return &copy, nil
}

func (r *upstreamRelayRecommendationServiceRepo) ListSnapshots(_ context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error) {
	items := make([]UpstreamRelayGroupRateSnapshot, 0, len(r.snapshots))
	for _, snapshot := range r.snapshots {
		if snapshot.ConnectorID == 0 || snapshot.ConnectorID == connectorID {
			items = append(items, snapshot)
		}
	}
	return items, nil
}

func (r *upstreamRelayRecommendationServiceRepo) ListRecommendationInputs(context.Context) ([]UpstreamRelayCandidate, error) {
	return append([]UpstreamRelayCandidate{}, r.candidates...), nil
}

func (r *upstreamRelayRecommendationServiceRepo) ListCandidates(context.Context, pagination.PaginationParams, UpstreamRelayCandidateListFilters) ([]UpstreamRelayCandidate, *pagination.PaginationResult, error) {
	items := append([]UpstreamRelayCandidate{}, r.candidates...)
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: 1, PageSize: len(items), Pages: 1}, nil
}

func (r *upstreamRelayRecommendationServiceRepo) GetRecommendationPolicy(context.Context) (*UpstreamRelayRecommendationPolicy, error) {
	if r.policy == nil {
		return nil, nil
	}
	policy := *r.policy
	policy.SortFields = append([]string{}, r.policy.SortFields...)
	return &policy, nil
}

func (r *upstreamRelayRecommendationServiceRepo) GetMonitoringPolicy(context.Context) (*UpstreamRelayMonitoringPolicy, error) {
	if r.monitoring == nil {
		return nil, nil
	}
	policy := *r.monitoring
	return &policy, nil
}

func (r *upstreamRelayRecommendationServiceRepo) UpsertMonitoringPolicy(_ context.Context, policy UpstreamRelayMonitoringPolicy, operatorID int64) (*UpstreamRelayMonitoringPolicy, error) {
	policy.UpdatedBy = operatorID
	r.monitoring = &policy
	copy := policy
	return &copy, nil
}

func (r *upstreamRelayRecommendationServiceRepo) CreateRecommendationRun(_ context.Context, run UpstreamRelayRecommendationRun, suggestions []UpstreamRelayRecommendationSuggestion) (*UpstreamRelayRecommendationRun, error) {
	r.createdRuns++
	run.ID = int64(r.createdRuns)
	run.SuggestionCount = len(suggestions)
	run.Suggestions = append([]UpstreamRelayRecommendationSuggestion{}, suggestions...)
	return &run, nil
}

func (r *upstreamRelayRecommendationServiceRepo) ApplyRecommendationRun(_ context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	r.appliedRuns++
	if r.applyErr != nil {
		return nil, r.applyErr
	}
	return &UpstreamRelayRecommendationRun{
		ID:        runID,
		Status:    UpstreamRelayRunStatusSuccess,
		Applied:   true,
		AppliedBy: &operatorID,
	}, nil
}

func (r *upstreamRelayRecommendationServiceRepo) RestoreRecommendationRun(_ context.Context, runID, operatorID int64) (*UpstreamRelayRecommendationRun, error) {
	_ = operatorID
	return &UpstreamRelayRecommendationRun{
		ID:      runID,
		Status:  UpstreamRelayRunStatusSuccess,
		Applied: false,
		Closed:  false,
	}, nil
}

func requireRelayNewPriority(t *testing.T, suggestion UpstreamRelayRecommendationSuggestion, want int) {
	t.Helper()
	require.NotNil(t, suggestion.NewPriority)
	require.Equal(t, want, *suggestion.NewPriority)
}

type upstreamRelayMetricsRefreshRepo struct {
	UpstreamRelayRepository

	connector            *UpstreamRelayConnector
	connectors           []UpstreamRelayConnector
	snapshots            []UpstreamRelayGroupRateSnapshot
	snapshotsByConnector map[int64][]UpstreamRelayGroupRateSnapshot
	bindings             []UpstreamRelayCandidateUsageBinding
	upsertSnapshotsCalls int
	tokenUpdateCalls     int
	tokenUpdateVersion   int64
	tokenUpdateAccess    string
	tokenUpdateRefresh   string
	tokenUpdateErr       error
	updatedBalance       *float64
	updatedBalanceAt     *time.Time
	syncStatus           string
	syncError            string
	usageByGroup         map[string]UpstreamRelayGroupTodayUsage
	usageCheckedAt       *time.Time
	usageHistoryRows     []UpstreamRelayGroupUsageHistoryUpsert
}

func (r *upstreamRelayMetricsRefreshRepo) GetConnector(_ context.Context, id int64) (*UpstreamRelayConnector, error) {
	if r.connector != nil && r.connector.ID == id {
		copy := *r.connector
		return &copy, nil
	}
	for _, connector := range r.connectors {
		if connector.ID != id {
			continue
		}
		copy := connector
		return &copy, nil
	}
	return nil, ErrUpstreamRelayConnectorNotFound
}

func (r *upstreamRelayMetricsRefreshRepo) ListConnectors(context.Context, pagination.PaginationParams, UpstreamRelayConnectorListFilters) ([]UpstreamRelayConnector, *pagination.PaginationResult, error) {
	items := append([]UpstreamRelayConnector{}, r.connectors...)
	if len(items) == 0 && r.connector != nil {
		items = append(items, *r.connector)
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: 1, PageSize: len(items), Pages: 1}, nil
}

func (r *upstreamRelayMetricsRefreshRepo) UpsertSnapshots(_ context.Context, connectorID int64, snapshots []UpstreamRelayGroupRateSnapshot) error {
	r.upsertSnapshotsCalls++
	next := append([]UpstreamRelayGroupRateSnapshot{}, snapshots...)
	for i := range next {
		next[i].ConnectorID = connectorID
	}
	r.snapshots = next
	if r.snapshotsByConnector != nil {
		r.snapshotsByConnector[connectorID] = append([]UpstreamRelayGroupRateSnapshot{}, next...)
	}
	return nil
}

func (r *upstreamRelayMetricsRefreshRepo) GetMonitoringPolicy(context.Context) (*UpstreamRelayMonitoringPolicy, error) {
	return nil, nil
}

func (r *upstreamRelayMetricsRefreshRepo) UpdateConnectorTokens(_ context.Context, connectorID int64, expectedCredentialVersion int64, bearerTokenEncrypted, refreshTokenEncrypted string) (*UpstreamRelayConnector, error) {
	r.tokenUpdateCalls++
	r.tokenUpdateVersion = expectedCredentialVersion
	r.tokenUpdateAccess = bearerTokenEncrypted
	r.tokenUpdateRefresh = refreshTokenEncrypted
	if r.tokenUpdateErr != nil {
		return nil, r.tokenUpdateErr
	}
	if r.connector == nil || r.connector.ID != connectorID {
		return nil, ErrUpstreamRelayConnectorNotFound
	}
	r.connector.BearerTokenEncrypted = bearerTokenEncrypted
	r.connector.RefreshTokenEncrypted = refreshTokenEncrypted
	r.connector.CredentialVersion = expectedCredentialVersion + 1
	r.connector.Status = UpstreamRelayConnectorStatusActive
	r.connector.LastError = ""
	copy := *r.connector
	return &copy, nil
}

func (r *upstreamRelayMetricsRefreshRepo) UpdateConnectorAccountBalance(_ context.Context, connectorID int64, balance *float64, checkedAt *time.Time) error {
	r.updatedBalance = balance
	r.updatedBalanceAt = checkedAt
	if r.connector != nil && r.connector.ID == connectorID {
		r.connector.UpstreamAccountBalance = balance
		r.connector.UpstreamAccountBalanceCheckedAt = checkedAt
	}
	for i := range r.connectors {
		if r.connectors[i].ID != connectorID {
			continue
		}
		r.connectors[i].UpstreamAccountBalance = balance
		r.connectors[i].UpstreamAccountBalanceCheckedAt = checkedAt
	}
	return nil
}

func (r *upstreamRelayMetricsRefreshRepo) MarkConnectorSync(_ context.Context, connectorID int64, status string, errMessage string) error {
	r.syncStatus = status
	r.syncError = errMessage
	if r.connector != nil && r.connector.ID == connectorID {
		r.connector.Status = status
		r.connector.LastError = errMessage
	}
	for i := range r.connectors {
		if r.connectors[i].ID != connectorID {
			continue
		}
		r.connectors[i].Status = status
		r.connectors[i].LastError = errMessage
	}
	return nil
}

func (r *upstreamRelayMetricsRefreshRepo) UpdateSnapshotTodayUsage(_ context.Context, connectorID int64, usageByGroup map[string]UpstreamRelayGroupTodayUsage, checkedAt *time.Time, complete bool) error {
	r.usageByGroup = usageByGroup
	r.usageCheckedAt = checkedAt
	update := func(snapshots []UpstreamRelayGroupRateSnapshot) []UpstreamRelayGroupRateSnapshot {
		out := append([]UpstreamRelayGroupRateSnapshot{}, snapshots...)
		for i := range out {
			if out[i].ConnectorID != 0 && out[i].ConnectorID != connectorID {
				continue
			}
			if checkedAt == nil || usageByGroup == nil {
				out[i].TodayActualCost = nil
				out[i].TodayTotalTokens = nil
				out[i].TodayUsageCheckedAt = nil
				continue
			}
			usage, known := usageByGroup[out[i].UpstreamGroupID]
			if !known && !complete {
				continue
			}
			actualCost := usage.ActualCost
			totalTokens := usage.TotalTokens
			out[i].TodayActualCost = &actualCost
			out[i].TodayTotalTokens = &totalTokens
			out[i].TodayUsageCheckedAt = checkedAt
		}
		return out
	}
	r.snapshots = update(r.snapshots)
	if r.snapshotsByConnector != nil {
		r.snapshotsByConnector[connectorID] = update(r.snapshotsByConnector[connectorID])
	}
	return nil
}

func (r *upstreamRelayMetricsRefreshRepo) ListSnapshots(_ context.Context, connectorID int64) ([]UpstreamRelayGroupRateSnapshot, error) {
	if r.snapshotsByConnector != nil {
		out := append([]UpstreamRelayGroupRateSnapshot{}, r.snapshotsByConnector[connectorID]...)
		return out, nil
	}
	out := make([]UpstreamRelayGroupRateSnapshot, 0, len(r.snapshots))
	for _, snapshot := range r.snapshots {
		if snapshot.ConnectorID != 0 && snapshot.ConnectorID != connectorID {
			continue
		}
		out = append(out, snapshot)
	}
	return out, nil
}

func (r *upstreamRelayMetricsRefreshRepo) UpsertUsageHistory(_ context.Context, rows []UpstreamRelayGroupUsageHistoryUpsert) error {
	r.usageHistoryRows = append([]UpstreamRelayGroupUsageHistoryUpsert{}, rows...)
	return nil
}

func (r *upstreamRelayMetricsRefreshRepo) SummarizeUsageHistory(context.Context, UpstreamRelayUsageHistoryListFilters) (*UpstreamRelayUsageHistorySummary, error) {
	return &UpstreamRelayUsageHistorySummary{RowCount: 1}, nil
}

func (r *upstreamRelayMetricsRefreshRepo) ListCandidateUsageBindings(context.Context, int64) ([]UpstreamRelayCandidateUsageBinding, error) {
	out := make([]UpstreamRelayCandidateUsageBinding, len(r.bindings))
	copy(out, r.bindings)
	return out, nil
}

type upstreamRelayMetricsRefreshAccountRepo struct {
	AccountRepository
	accounts map[int64]*Account
}

func (r upstreamRelayMetricsRefreshAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	account := r.accounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	copy := *account
	return &copy, nil
}

func TestUpstreamRelayParseGroupRatesAllowsEmptyObject(t *testing.T) {
	rates, err := parseGroupRates([]byte(`{}`))

	require.NoError(t, err)
	require.Empty(t, rates)
}

func TestUpstreamRelayParseAccountBalanceSupportsProfileShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want float64
	}{
		{name: "data balance", body: `{"data":{"balance":12.34}}`, want: 12.34},
		{name: "top level balance", body: `{"balance":"56.78"}`, want: 56.78},
		{name: "nested user account balance", body: `{"data":{"user":{"account_balance":90.12}}}`, want: 90.12},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			balance, err := parseUpstreamAccountBalance([]byte(tc.body))

			require.NoError(t, err)
			require.NotNil(t, balance)
			require.Equal(t, tc.want, *balance)
		})
	}
}

func TestUpstreamRelayParseAccountBalanceRequiresBalance(t *testing.T) {
	balance, err := parseUpstreamAccountBalance([]byte(`{"data":{"user":{"email":"admin@example.com"}}}`))

	require.Error(t, err)
	require.Nil(t, balance)
	require.Contains(t, err.Error(), "missing balance")
}

func TestUpstreamRelayFetchGroupSnapshotsUsesOverrideRateFirst(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer session-token", r.Header.Get("Authorization"))
		require.Equal(t, "cf_clearance=ok", r.Header.Get("Cookie"))
		require.Equal(t, "relay-browser", r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte(`[
			{"id":101,"name":"fast","platform":"openai","status":"active","rate_multiplier":1.5},
			{"id":"slow","name":"slow","platform":"anthropic","status":"active","rate_multiplier":2.5}
		]`))
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"101": 0.75}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	svc := &UpstreamRelayGroupMonitoringService{httpClient: server.Client()}
	snapshots, err := svc.fetchGroupSnapshots(context.Background(), &UpstreamRelayConnector{
		ID:               7,
		BaseURL:          server.URL,
		BearerTokenPlain: "session-token",
		CookiePlain:      "cf_clearance=ok",
		UserAgentPlain:   "relay-browser",
	})

	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	require.Equal(t, "101", snapshots[0].UpstreamGroupID)
	require.Equal(t, 1.5, snapshots[0].DefaultRateMultiplier)
	require.NotNil(t, snapshots[0].OverrideRateMultiplier)
	require.Equal(t, 0.75, *snapshots[0].OverrideRateMultiplier)
	require.Equal(t, 0.75, snapshots[0].FinalRateMultiplier)
	require.Equal(t, UpstreamRelayRateSourceOverride, snapshots[0].Source)
	require.Equal(t, 2.5, snapshots[1].FinalRateMultiplier)
	require.Nil(t, snapshots[1].OverrideRateMultiplier)
	require.Equal(t, UpstreamRelayRateSourceAvailable, snapshots[1].Source)
}

func TestUpstreamRelayGetUpstreamJSONRefreshesExpiredTokenAndRetries(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		if profileCalls == 1 {
			require.Equal(t, "Bearer expired-access", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired","access_token":"leaked-access"}`))
			return
		}
		require.Equal(t, "Bearer fresh-access", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		require.Equal(t, http.MethodPost, r.Method)
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "old-refresh", payload["refresh_token"])
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-access","refresh_token":"fresh-refresh","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	body, err := svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.NoError(t, err)
	require.JSONEq(t, `{"data":{"balance":12.34}}`, string(body))
	require.Equal(t, 2, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, int64(3), repo.tokenUpdateVersion)
	require.Equal(t, "enc:fresh-access", repo.tokenUpdateAccess)
	require.Equal(t, "enc:fresh-refresh", repo.tokenUpdateRefresh)
	require.Equal(t, "fresh-access", connector.BearerTokenPlain)
	require.Equal(t, "fresh-refresh", connector.RefreshTokenPlain)
	require.Equal(t, int64(4), connector.CredentialVersion)
}

func TestUpstreamRelayManualSessionRefreshesExpiredTokenAndRetries(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		if profileCalls == 1 {
			require.Equal(t, "Bearer manual-expired-access", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired"}`))
			return
		}
		require.Equal(t, "Bearer manual-fresh-access", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":45.67}}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "manual-refresh", payload["refresh_token"])
		_, _ = w.Write([]byte(`{"data":{"access_token":"manual-fresh-access","refresh_token":"manual-fresh-refresh","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			AuthMode:              UpstreamRelayAuthModeManualSession,
			BearerTokenEncrypted:  "manual-expired-access",
			RefreshTokenEncrypted: "manual-refresh",
			CredentialVersion:     5,
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	body, err := svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.NoError(t, err)
	require.JSONEq(t, `{"data":{"balance":45.67}}`, string(body))
	require.Equal(t, 2, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, "enc:manual-fresh-access", repo.tokenUpdateAccess)
	require.Equal(t, "enc:manual-fresh-refresh", repo.tokenUpdateRefresh)
	require.Empty(t, repo.syncStatus)
}

func TestUpstreamRelayGetUpstreamJSONMarksNeedsReauthWhenRefreshFails(t *testing.T) {
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired"}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid refresh","refresh_token":"leaked-refresh","access_token":"leaked-access","cookie":"leaked-cookie"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	_, err = svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.Error(t, err)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, 0, repo.tokenUpdateCalls)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, repo.syncStatus)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, connector.Status)
	require.Contains(t, repo.syncError, "upstream refresh HTTP 401")
	require.Contains(t, repo.syncError, "[REDACTED]")
	require.NotContains(t, repo.syncError, "leaked-refresh")
	require.NotContains(t, repo.syncError, "leaked-access")
	require.NotContains(t, repo.syncError, "leaked-cookie")
	require.NotContains(t, err.Error(), "leaked-refresh")
	require.NotContains(t, err.Error(), "leaked-access")
	require.NotContains(t, err.Error(), "leaked-cookie")
}

func TestUpstreamRelayGetUpstreamJSONReloadsConcurrentRefreshAfterRefreshFailure(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	var repo *upstreamRelayMetricsRefreshRepo
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		if profileCalls == 1 {
			require.Equal(t, "Bearer expired-access", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired"}`))
			return
		}
		require.Equal(t, "Bearer concurrent-fresh-access", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":98.76}}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		repo.connector.BearerTokenEncrypted = "concurrent-fresh-access"
		repo.connector.RefreshTokenEncrypted = "concurrent-fresh-refresh"
		repo.connector.CredentialVersion = 4
		repo.connector.Status = UpstreamRelayConnectorStatusActive
		repo.connector.LastError = ""
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid refresh","refresh_token":"old-refresh"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo = &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
			Status:                UpstreamRelayConnectorStatusActive,
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	body, err := svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.NoError(t, err)
	require.JSONEq(t, `{"data":{"balance":98.76}}`, string(body))
	require.Equal(t, 2, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, int64(4), connector.CredentialVersion)
	require.Equal(t, "concurrent-fresh-access", connector.BearerTokenPlain)
	require.Equal(t, "concurrent-fresh-refresh", connector.RefreshTokenPlain)
	require.Empty(t, repo.syncStatus)
}

func TestUpstreamRelayGetUpstreamJSONMarksNeedsReauthWhenTokenUpdateConflictDoesNotAdvanceVersion(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		require.Equal(t, "Bearer expired-access", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired"}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-access","refresh_token":"fresh-refresh","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
		},
		tokenUpdateErr: ErrUpstreamRelayCredentialVersionConflict,
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	_, err = svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.Error(t, err)
	require.Equal(t, 1, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, repo.syncStatus)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, connector.Status)
	require.Contains(t, repo.syncError, "credentials were updated concurrently")
	require.Equal(t, int64(3), connector.CredentialVersion)
}

func TestUpstreamRelayGetUpstreamJSONMarksNeedsReauthWhenRetryStillTokenExpired(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		if profileCalls == 1 {
			require.Equal(t, "Bearer expired-access", r.Header.Get("Authorization"))
		} else {
			require.Equal(t, "Bearer fresh-but-rejected-access", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired","access_token":"leaked-retry-access"}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-but-rejected-access","refresh_token":"fresh-refresh","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()
	connector, err := repo.GetConnector(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, svc.decryptConnector(connector))

	_, err = svc.getUpstreamJSON(context.Background(), connector, "/api/v1/user/profile")

	require.Error(t, err)
	require.Equal(t, 2, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, repo.syncStatus)
	require.Equal(t, UpstreamRelayConnectorStatusNeedsReauth, connector.Status)
	require.Contains(t, repo.syncError, "upstream HTTP 401")
	require.Contains(t, repo.syncError, "[REDACTED]")
	require.NotContains(t, repo.syncError, "leaked-retry-access")
}

func TestUpstreamRelayGetUpstreamJSONDoesNotRefreshWithoutRefreshToken(t *testing.T) {
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired","refresh_token":"leaked-refresh"}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		http.Error(w, "refresh should not be called", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	svc := &UpstreamRelayGroupMonitoringService{httpClient: server.Client()}

	_, err := svc.getUpstreamJSON(context.Background(), &UpstreamRelayConnector{
		ID:               7,
		BaseURL:          server.URL,
		BearerTokenPlain: "expired-access",
	}, "/api/v1/user/profile")

	require.Error(t, err)
	require.Equal(t, 0, refreshCalls)
	require.NotContains(t, err.Error(), "leaked-refresh")
	require.Contains(t, err.Error(), "[REDACTED]")
}

func TestUpstreamRelayFetchGroupSnapshotsLeavesUsageEmptyWithoutBindings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":101,"name":"fast","platform":"openai","status":"active","rate_multiplier":1.5},
			{"id":202,"name":"slow","platform":"anthropic","status":"active","rate_multiplier":2.5}
		]`))
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/v1/usage", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "usage list should not be called by snapshot sync", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	svc := &UpstreamRelayGroupMonitoringService{httpClient: server.Client()}
	snapshots, err := svc.fetchGroupSnapshots(context.Background(), &UpstreamRelayConnector{
		ID:               7,
		BaseURL:          server.URL,
		BearerTokenPlain: "session-token",
	})

	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	require.Nil(t, snapshots[0].TodayActualCost)
	require.Nil(t, snapshots[0].TodayTotalTokens)
	require.Nil(t, snapshots[0].TodayUsageCheckedAt)
	require.Nil(t, snapshots[1].TodayActualCost)
	require.Nil(t, snapshots[1].TodayTotalTokens)
	require.Nil(t, snapshots[1].TodayUsageCheckedAt)
}

func TestUpstreamRelayFetchGroupSnapshotsLeavesUsageEmptyWhenUserUsageUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":101,"name":"fast","platform":"openai","status":"active","rate_multiplier":1.5}]`))
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/v1/usage", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "usage unavailable", http.StatusForbidden)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	svc := &UpstreamRelayGroupMonitoringService{httpClient: server.Client()}
	snapshots, err := svc.fetchGroupSnapshots(context.Background(), &UpstreamRelayConnector{
		ID:               7,
		BaseURL:          server.URL,
		BearerTokenPlain: "session-token",
	})

	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Nil(t, snapshots[0].TodayActualCost)
	require.Nil(t, snapshots[0].TodayTotalTokens)
	require.Nil(t, snapshots[0].TodayUsageCheckedAt)
}

func TestUpstreamRelayParseUsageStatsFallsBackToTokenBreakdown(t *testing.T) {
	usage, err := parseUpstreamUsageStats([]byte(`{"data":{
		"total_actual_cost": "1.25",
		"total_input_tokens": 10,
		"total_output_tokens": 20,
		"total_cache_creation_tokens": 3,
		"total_cache_read_tokens": 4
	}}`))

	require.NoError(t, err)
	require.Equal(t, 1.25, usage.ActualCost)
	require.Equal(t, int64(37), usage.TotalTokens)
}

func TestUpstreamRelayRefreshMonitoringDataSyncsSnapshotsAndMetrics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-***","group_id":"g1"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer session-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`[{"id":"g1","name":"Group 1","platform":"openai","status":"active","rate_multiplier":1.25}]`))
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"g1":0.75}`))
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer session-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "855", r.URL.Query().Get("api_key_id"))
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":4.5,"total_tokens":1200}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			Name:                 "relay-a",
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 10, UpstreamGroupID: "g1", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshMonitoringData(context.Background())

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, result.Status)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 1, result.Success)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, int64(7), item.ConnectorID)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.SnapshotStatus)
	require.Equal(t, 1, item.SnapshotCount)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.Status)
	require.NotNil(t, item.Metrics)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.Metrics.Status)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.Metrics.BalanceDetail.Status)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.Metrics.UsageDetail.Status)
	require.NotNil(t, item.Metrics.BalanceDetail.Value)
	require.Equal(t, 12.34, *item.Metrics.BalanceDetail.Value)
	require.Len(t, item.Snapshots, 1)
	require.NotNil(t, item.Snapshots[0].TodayActualCost)
	require.Equal(t, 4.5, *item.Snapshots[0].TodayActualCost)
}

func TestUpstreamRelayRefreshMonitoringDataReportsPartialSnapshotFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-***","group_id":"g1"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"g1","name":"Group 1","platform":"openai","status":"active","rate_multiplier":1.25}]`))
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary upstream outage", http.StatusBadGateway)
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":4.5,"total_tokens":1200}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			Name:                 "relay-a",
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		snapshots: []UpstreamRelayGroupRateSnapshot{
			{ID: 1, ConnectorID: 7, UpstreamGroupID: "g1", Name: "Group 1", FinalRateMultiplier: 1.25},
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 10, UpstreamGroupID: "g1", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshMonitoringData(context.Background())

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.Equal(t, 1, result.Partial)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, upstreamRelayMetricsRefreshStatusFailed, item.SnapshotStatus)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, item.Status)
	require.Contains(t, item.SnapshotError, "upstream HTTP 502")
	require.Contains(t, item.ErrorReason, "upstream HTTP 502")
	require.NotNil(t, item.Metrics)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, item.Metrics.Status)
}

func TestUpstreamRelayRefreshConnectorMetricsUpdatesUsageFromBoundAPIKeyStats(t *testing.T) {
	usageListCalls := 0
	statsCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"特惠","key":"sk-***","group_id":"g1"},{"id":856,"name":"稳定","key":"sk-***","group_id":"g1"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer session-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage", func(w http.ResponseWriter, r *http.Request) {
		usageListCalls++
		http.Error(w, "usage list should not be called by lightweight metrics refresh", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		statsCalls++
		switch r.URL.Query().Get("api_key_id") {
		case "855":
			_, _ = w.Write([]byte(`{"data":{"total_actual_cost":4.5,"total_tokens":1200}}`))
		case "856":
			_, _ = w.Write([]byte(`{"data":{"total_actual_cost":1.25,"total_input_tokens":100,"total_output_tokens":20,"total_cache_creation_tokens":3,"total_cache_read_tokens":4}}`))
		default:
			http.Error(w, "unexpected api key", http.StatusBadRequest)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		snapshots: []UpstreamRelayGroupRateSnapshot{
			{ID: 1, ConnectorID: 7, UpstreamGroupID: "g1", Name: "Group 1", FinalRateMultiplier: 0.8},
			{ID: 2, ConnectorID: 7, UpstreamGroupID: "g2", Name: "Group 2", FinalRateMultiplier: 1.2},
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 10, UpstreamGroupID: "g1", UpstreamAPIKeyID: 855, UpstreamAPIKeyName: "特惠"},
			{CandidateID: 2, ConnectorID: 7, AccountID: 11, UpstreamGroupID: "g1", UpstreamAPIKeyID: 856, UpstreamAPIKeyName: "稳定"},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, result.Status)
	require.True(t, result.BalanceAvailable)
	require.True(t, result.UsageAvailable)
	require.Empty(t, result.UsageError)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, result.BalanceDetail.Status)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSuccess, result.UsageDetail.Status)
	require.Equal(t, 1, result.UsageDetail.TotalGroups)
	require.Equal(t, 1, result.UsageDetail.UpdatedGroups)
	require.Empty(t, result.UsageDetail.MissingGroups)
	require.Equal(t, 0, repo.upsertSnapshotsCalls)
	require.Equal(t, 0, usageListCalls)
	require.Equal(t, 2, statsCalls)
	require.NotNil(t, repo.updatedBalance)
	require.Equal(t, 12.34, *repo.updatedBalance)
	require.NotNil(t, repo.updatedBalanceAt)
	require.NotNil(t, repo.usageCheckedAt)
	require.NotNil(t, repo.usageByGroup)
	require.Equal(t, 5.75, repo.usageByGroup["g1"].ActualCost)
	require.Equal(t, int64(1327), repo.usageByGroup["g1"].TotalTokens)
	require.Equal(t, 0.0, repo.usageByGroup["g2"].ActualCost)
	require.Len(t, repo.usageHistoryRows, 2)
	require.Equal(t, "g1", repo.usageHistoryRows[0].UpstreamGroupID)
	require.Equal(t, 5.75, repo.usageHistoryRows[0].ActualCost)
	require.Equal(t, int64(1327), repo.usageHistoryRows[0].TotalTokens)
	require.Equal(t, "g2", repo.usageHistoryRows[1].UpstreamGroupID)
	require.Equal(t, 0.0, repo.usageHistoryRows[1].ActualCost)
	require.Equal(t, int64(0), repo.usageHistoryRows[1].TotalTokens)
	require.Len(t, result.Snapshots, 2)
	require.NotNil(t, result.Snapshots[0].TodayActualCost)
	require.Equal(t, 5.75, *result.Snapshots[0].TodayActualCost)
	require.NotNil(t, result.Snapshots[1].TodayActualCost)
	require.Equal(t, 0.0, *result.Snapshots[1].TodayActualCost)
}

func TestUpstreamRelayRefreshConnectorMetricsReturnsConnectorAfterTokenRefresh(t *testing.T) {
	profileCalls := 0
	refreshCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		if profileCalls == 1 {
			require.Equal(t, "Bearer expired-access", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"Token has expired"}`))
			return
		}
		require.Equal(t, "Bearer fresh-access", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-access","refresh_token":"fresh-refresh","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                    7,
			BaseURL:               server.URL,
			BearerTokenEncrypted:  "expired-access",
			RefreshTokenEncrypted: "old-refresh",
			CredentialVersion:     3,
			Status:                UpstreamRelayConnectorStatusNeedsReauth,
			LastError:             "previous token expired",
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.True(t, result.BalanceAvailable)
	require.False(t, result.UsageAvailable)
	require.Equal(t, 2, profileCalls)
	require.Equal(t, 1, refreshCalls)
	require.NotNil(t, result.Connector)
	require.Equal(t, int64(4), result.Connector.CredentialVersion)
	require.Equal(t, UpstreamRelayConnectorStatusActive, result.Connector.Status)
	require.Empty(t, result.Connector.LastError)
	require.True(t, result.Connector.HasRefreshToken)
}

func TestUpstreamRelayListConnectorAPIKeysReturnsVisibleOptions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer session-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"items":[
			{"id":855,"name":"特惠","key":"sk-live-secret-value-a","group_id":"new-group"},
			{"id":856,"key_name":"稳定","key":"sk-***masked-b***","group_id":42}
		],"pages":1}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	items, err := svc.ListConnectorAPIKeys(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int64(855), items[0].ID)
	require.Equal(t, "特惠", items[0].Name)
	require.Equal(t, "sk-liv***ue-a", items[0].MaskedKey)
	require.Equal(t, "new-group", items[0].GroupID)
	require.Equal(t, int64(856), items[1].ID)
	require.Equal(t, "稳定", items[1].Name)
	require.Equal(t, "sk-***masked-b***", items[1].MaskedKey)
	require.Equal(t, "42", items[1].GroupID)
}

func TestUpstreamRelayUsageFollowsCurrentAPIKeyGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"特惠","key":"sk-live","group_id":"new-group"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "855", r.URL.Query().Get("api_key_id"))
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":1.25,"total_tokens":100}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{ID: 7, BaseURL: server.URL, BearerTokenEncrypted: "session-token"},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, AccountID: 10, UpstreamGroupID: "old-group", UpstreamAPIKeyID: 855},
			{CandidateID: 2, AccountID: 11, UpstreamGroupID: "old-group", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	usage, _, issues, err := svc.fetchUpstreamGroupUsageForDate(context.Background(), repo.connector, upstreamRelayUsageDate(time.Now()))

	require.NoError(t, err)
	require.Empty(t, issues)
	require.Equal(t, UpstreamRelayGroupTodayUsage{ActualCost: 1.25, TotalTokens: 100}, usage["new-group"])
	require.NotContains(t, usage, "old-group")
}

func TestUpstreamRelayHistoricalUsageKeepsSavedCandidateGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("historical usage must not resolve today's API key group")
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":1.25,"total_tokens":100}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{ID: 7, BaseURL: server.URL, BearerTokenEncrypted: "session-token"},
		bindings:  []UpstreamRelayCandidateUsageBinding{{CandidateID: 1, AccountID: 10, UpstreamGroupID: "saved-group", UpstreamAPIKeyID: 855}},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	usage, _, issues, err := svc.fetchUpstreamGroupUsageForDate(context.Background(), repo.connector, upstreamRelayPreviousUsageDate(time.Now()))

	require.NoError(t, err)
	require.Empty(t, issues)
	require.Equal(t, UpstreamRelayGroupTodayUsage{ActualCost: 1.25, TotalTokens: 100}, usage["saved-group"])
}

func TestUpstreamRelayListCandidatesUsesCurrentAPIKeyGroupSnapshot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-live","group_id":"new-group"}],"pages":1}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayRecommendationServiceRepo{
		connector:  &UpstreamRelayConnector{ID: 7, BaseURL: server.URL, BearerTokenEncrypted: "session-token"},
		candidates: []UpstreamRelayCandidate{{ID: 1, ConnectorID: 7, UpstreamGroupID: "old-group", UpstreamAPIKeyID: int64Ptr(855)}},
		snapshots:  []UpstreamRelayGroupRateSnapshot{{ConnectorID: 7, UpstreamGroupID: "new-group", Name: "New Group", FinalRateMultiplier: 0.5}},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	items, _, err := svc.ListCandidates(context.Background(), 1, 20, UpstreamRelayCandidateListFilters{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "new-group", items[0].UpstreamGroupID)
	require.Equal(t, "New Group", items[0].UpstreamGroupName)
	require.NotNil(t, items[0].LatestSnapshot)
}

func TestUpstreamRelayPreviewRecommendationsUsesCurrentAPIKeyGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-live","group_id":"new-group"}],"pages":1}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	now := time.Now()
	priority := 50
	repo := &upstreamRelayRecommendationServiceRepo{
		connector: &UpstreamRelayConnector{ID: 7, BaseURL: server.URL, BearerTokenEncrypted: "session-token"},
		candidates: []UpstreamRelayCandidate{{
			ID: 1, ConnectorID: 7, AccountID: 101, ConnectorStatus: UpstreamRelayConnectorStatusActive,
			UpstreamGroupID: "old-group", UpstreamAPIKeyID: int64Ptr(855), CurrentPriority: &priority,
			Enabled: true, LatestProbe: &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{UpstreamGroupID: "old-group", FinalRateMultiplier: 0.8, LastSeenAt: now},
		}, {
			ID: 2, ConnectorID: 7, AccountID: 102, ConnectorStatus: UpstreamRelayConnectorStatusActive,
			UpstreamGroupID: "missing-group", UpstreamAPIKeyID: int64Ptr(856), CurrentPriority: &priority,
			Enabled: true, LatestProbe: &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{UpstreamGroupID: "missing-group", FinalRateMultiplier: 0.4, LastSeenAt: now},
		}},
		snapshots: []UpstreamRelayGroupRateSnapshot{{ConnectorID: 7, UpstreamGroupID: "new-group", Name: "New Group", FinalRateMultiplier: 0.5, LastSeenAt: now}},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	preview, err := svc.PreviewRecommendations(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, preview.Suggestions, 1)
	require.Equal(t, "new-group", preview.Suggestions[0].UpstreamGroupID)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, int64(2), preview.Exclusions[0].CandidateID)
}

func TestUpstreamRelayUsageReportsMissingCurrentAPIKeyGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"特惠","key":"sk-live"}],"pages":1}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{ID: 7, BaseURL: server.URL, BearerTokenEncrypted: "session-token"},
		bindings:  []UpstreamRelayCandidateUsageBinding{{CandidateID: 1, AccountID: 10, UpstreamGroupID: "old-group", UpstreamAPIKeyID: 855}},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	usage, _, issues, err := svc.fetchUpstreamGroupUsageForDate(context.Background(), repo.connector, upstreamRelayUsageDate(time.Now()))

	require.NoError(t, err)
	require.Empty(t, usage)
	require.Len(t, issues, 1)
	require.Equal(t, upstreamRelayMetricsIssueAPIKeyGroupUnavailable, issues[0].Code)
}

func TestNormalizeUpstreamRelayCandidateMasksAPIKeyDisplayValue(t *testing.T) {
	keyID := int64(855)

	candidate, err := normalizeUpstreamRelayCandidateInput(UpstreamRelayCandidateInput{
		ConnectorID:          7,
		AccountID:            10,
		UpstreamGroupID:      "g1",
		UpstreamAPIKeyID:     &keyID,
		UpstreamAPIKeyName:   "特惠",
		UpstreamAPIKeyMasked: "sk-live-secret-value-a",
		ProbeModel:           "gpt-4o-mini",
		ProbeProtocol:        MonitorAPIModeChatCompletions,
	}, 0, 88)

	require.NoError(t, err)
	require.Equal(t, "sk-liv***ue-a", candidate.UpstreamAPIKeyMasked)
}

func TestNormalizeUpstreamRelayCandidateAcceptsAnthropicProbeProtocol(t *testing.T) {
	candidate, err := normalizeUpstreamRelayCandidateInput(UpstreamRelayCandidateInput{
		ConnectorID:     7,
		AccountID:       10,
		UpstreamGroupID: "g1",
		ProbeModel:      "claude-sonnet-4-5",
		ProbeProtocol:   UpstreamRelayProbeProtocolAnthropic,
	}, 0, 88)

	require.NoError(t, err)
	require.Equal(t, UpstreamRelayProbeProtocolAnthropic, candidate.ProbeProtocol)
}

func TestUpstreamRelayRefreshConnectorMetricsReportsMissingCandidateAPIKeyBinding(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-***","group_id":"g2"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "855", r.URL.Query().Get("api_key_id"))
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":1.25,"total_tokens":88}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		snapshots: []UpstreamRelayGroupRateSnapshot{
			{ID: 1, ConnectorID: 7, UpstreamGroupID: "g1", Name: "Group 1", FinalRateMultiplier: 1},
			{ID: 2, ConnectorID: 7, UpstreamGroupID: "g2", Name: "Group 2", FinalRateMultiplier: 1},
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 6, UpstreamGroupID: "g1"},
			{CandidateID: 2, ConnectorID: 7, AccountID: 7, UpstreamGroupID: "g2", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.True(t, result.BalanceAvailable)
	require.False(t, result.UsageAvailable)
	require.Contains(t, result.UsageError, "candidate 1 for bound account 6 has no upstream api key binding")
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.UsageDetail.Status)
	require.Equal(t, 2, result.UsageDetail.TotalGroups)
	require.Equal(t, 1, result.UsageDetail.UpdatedGroups)
	require.NotNil(t, result.UsageDetail.Issue)
	require.Equal(t, upstreamRelayMetricsIssueMissingAPIKeyBinding, result.UsageDetail.Issue.Code)
	require.Equal(t, int64(1), result.UsageDetail.Issue.CandidateID)
	require.Equal(t, int64(6), result.UsageDetail.Issue.AccountID)
	require.Equal(t, "g1", result.UsageDetail.Issue.UpstreamGroupID)
	require.Len(t, result.UsageDetail.MissingGroups, 1)
	require.Equal(t, "g1", result.UsageDetail.MissingGroups[0].UpstreamGroupID)
	require.Equal(t, upstreamRelayMetricsIssueMissingAPIKeyBinding, result.UsageDetail.MissingGroups[0].Reason)
	require.Equal(t, 1.25, repo.usageByGroup["g2"].ActualCost)
	require.Equal(t, int64(88), repo.usageByGroup["g2"].TotalTokens)
}

func TestUpstreamRelayRefreshConnectorMetricsReportsMissingCandidateBindingsAsStructuredIssue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.True(t, result.BalanceAvailable)
	require.False(t, result.UsageAvailable)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSkipped, result.UsageDetail.Status)
	require.NotNil(t, result.UsageDetail.Issue)
	require.Equal(t, upstreamRelayMetricsIssueNoCandidateBindings, result.UsageDetail.Issue.Code)
	require.Empty(t, result.UsageDetail.MissingGroups)
}

func TestUpstreamRelayRefreshConnectorMetricsKeepsExistingUsageSnapshotOnUsageFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-***","group_id":"g1"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "usage unavailable", http.StatusForbidden)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cost := 9.9
	tokens := int64(99)
	checkedAt := time.Date(2026, 6, 28, 8, 0, 0, 0, time.UTC)
	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		snapshots: []UpstreamRelayGroupRateSnapshot{
			{ID: 1, ConnectorID: 7, UpstreamGroupID: "g1", Name: "Group 1", TodayActualCost: &cost, TodayTotalTokens: &tokens, TodayUsageCheckedAt: &checkedAt},
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 10, UpstreamGroupID: "g1", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.True(t, result.BalanceAvailable)
	require.False(t, result.UsageAvailable)
	require.Contains(t, result.UsageError, "usage unavailable")
	require.Equal(t, upstreamRelayMetricsRefreshStatusFailed, result.UsageDetail.Status)
	require.Equal(t, 1, result.UsageDetail.TotalGroups)
	require.Equal(t, 0, result.UsageDetail.UpdatedGroups)
	require.Len(t, result.UsageDetail.MissingGroups, 1)
	require.Equal(t, upstreamRelayMetricsIssueUpstreamUsageRequest, result.UsageDetail.MissingGroups[0].Reason)
	require.Empty(t, repo.usageByGroup)
	require.NotNil(t, repo.usageCheckedAt)
	require.Empty(t, repo.usageHistoryRows)
	require.Len(t, result.Snapshots, 1)
	require.NotNil(t, result.Snapshots[0].TodayActualCost)
	require.Equal(t, 9.9, *result.Snapshots[0].TodayActualCost)
	require.NotNil(t, result.Snapshots[0].TodayTotalTokens)
	require.Equal(t, int64(99), *result.Snapshots[0].TodayTotalTokens)
	require.NotNil(t, result.Snapshots[0].TodayUsageCheckedAt)
}

func TestUpstreamRelayRefreshConnectorMetricsExplainsMissingSnapshots(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":855,"name":"Key","key":"sk-***","group_id":"g1"}],"pages":1}}`))
	})
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"balance":12.34}}`))
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"total_actual_cost":1.5,"total_tokens":88}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	repo := &upstreamRelayMetricsRefreshRepo{
		connector: &UpstreamRelayConnector{
			ID:                   7,
			BaseURL:              server.URL,
			BearerTokenEncrypted: "session-token",
		},
		bindings: []UpstreamRelayCandidateUsageBinding{
			{CandidateID: 1, ConnectorID: 7, AccountID: 10, UpstreamGroupID: "g1", UpstreamAPIKeyID: 855},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	svc.httpClient = server.Client()

	result, err := svc.RefreshConnectorMetrics(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, upstreamRelayMetricsRefreshStatusPartial, result.Status)
	require.True(t, result.BalanceAvailable)
	require.True(t, result.UsageAvailable)
	require.Equal(t, upstreamRelayMetricsRefreshStatusSkipped, result.UsageDetail.Status)
	require.Equal(t, 1, result.UsageDetail.TotalGroups)
	require.Equal(t, 0, result.UsageDetail.UpdatedGroups)
	require.Len(t, result.UsageDetail.MissingGroups, 1)
	require.Equal(t, "g1", result.UsageDetail.MissingGroups[0].UpstreamGroupID)
	require.Equal(t, "no_snapshot", result.UsageDetail.MissingGroups[0].Reason)
	require.Empty(t, result.Snapshots)
}

func TestUpstreamRelayConnectorInputEncryptsAndResponseRedactsSecrets(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	token := "Bearer sk-live-secret-token"
	refreshToken := "refresh-live-secret-token"
	cookie := "cf_clearance=super-secret-cookie"
	userAgent := "Mozilla/5.0 relay integration browser"

	connector, credentialsUpdated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:         "relay",
		BaseURL:      "https://relay.example.com/",
		AuthMode:     UpstreamRelayAuthModeManualSession,
		BearerToken:  &token,
		RefreshToken: &refreshToken,
		Cookie:       &cookie,
		UserAgent:    &userAgent,
	}, nil, 99)

	require.NoError(t, err)
	require.True(t, credentialsUpdated)
	require.Equal(t, "https://relay.example.com", connector.BaseURL)
	require.Equal(t, "enc:sk-live-secret-token", connector.BearerTokenEncrypted)
	require.Equal(t, "enc:refresh-live-secret-token", connector.RefreshTokenEncrypted)
	require.Equal(t, "enc:cf_clearance=super-secret-cookie", connector.CookieEncrypted)
	require.Equal(t, "enc:Mozilla/5.0 relay integration browser", connector.UserAgentEncrypted)

	connector.BearerTokenPlain = "sk-live-secret-token"
	connector.RefreshTokenPlain = refreshToken
	connector.CookiePlain = "cf_clearance=super-secret-cookie"
	connector.UserAgentPlain = userAgent
	svc.prepareConnectorResponse(connector)

	require.True(t, connector.HasBearerToken)
	require.True(t, connector.HasRefreshToken)
	require.True(t, connector.HasCookie)
	require.True(t, connector.HasUserAgent)
	require.Equal(t, "sk-l...oken", connector.BearerTokenMasked)
	require.NotContains(t, connector.RefreshTokenMasked, refreshToken)
	require.NotContains(t, connector.CookieMasked, "super-secret-cookie")
	require.Empty(t, connector.BearerTokenPlain)
	require.Empty(t, connector.RefreshTokenPlain)
	require.Empty(t, connector.CookiePlain)
	require.Empty(t, connector.UserAgentPlain)
}

func TestUpstreamRelayPasswordLoginExchangesTokenWithoutStoringPassword(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		_, _ = w.Write([]byte(`{"data":{"access_token":"login-access-token","refresh_token":"login-refresh-token","token_type":"Bearer"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	email := "admin@example.com"
	password := "upstream-password"
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}, httpClient: server.Client()}

	connector, credentialsUpdated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:          "relay",
		BaseURL:       server.URL,
		AuthMode:      UpstreamRelayAuthModePasswordLogin,
		LoginEmail:    &email,
		LoginPassword: &password,
	}, nil, 99)

	require.NoError(t, err)
	require.True(t, credentialsUpdated)
	require.Equal(t, UpstreamRelayAuthModePasswordLogin, connector.AuthMode)
	require.Equal(t, "enc:login-access-token", connector.BearerTokenEncrypted)
	require.Equal(t, "enc:login-refresh-token", connector.RefreshTokenEncrypted)
	require.Equal(t, "enc:admin@example.com", connector.LoginEmailEncrypted)
	require.NotContains(t, connector.BearerTokenEncrypted, password)
}

func TestUpstreamRelayPasswordLoginRejects2FA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"requires_2fa":true,"temp_token":"tmp"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	email := "admin@example.com"
	password := "upstream-password"
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}, httpClient: server.Client()}

	_, _, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:          "relay",
		BaseURL:       server.URL,
		AuthMode:      UpstreamRelayAuthModePasswordLogin,
		LoginEmail:    &email,
		LoginPassword: &password,
	}, nil, 99)

	require.ErrorIs(t, err, ErrUpstreamRelayPasswordLoginNeedsManualSession)
}

func TestUpstreamRelayPasswordLoginClearsStaleRefreshToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"access_token":"new-access-token"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	email := "admin@example.com"
	password := "upstream-password"
	existing := &UpstreamRelayConnector{
		Name:                  "relay",
		BaseURL:               server.URL,
		AuthMode:              UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc:old-access-token",
		RefreshTokenEncrypted: "enc:old-refresh-token",
		LoginEmailEncrypted:   "enc:admin@example.com",
	}
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}, httpClient: server.Client()}

	connector, updated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:          "relay",
		BaseURL:       server.URL,
		AuthMode:      UpstreamRelayAuthModePasswordLogin,
		LoginEmail:    &email,
		LoginPassword: &password,
	}, existing, 99)

	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, "enc:new-access-token", connector.BearerTokenEncrypted)
	require.Empty(t, connector.RefreshTokenEncrypted)
	require.Equal(t, "enc:admin@example.com", connector.LoginEmailEncrypted)
}

func TestUpstreamRelayManualSessionBaseURLChangeRequiresFreshBearerToken(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	existing := &UpstreamRelayConnector{
		Name:                 "relay",
		BaseURL:              "https://old-relay.example.com",
		AuthMode:             UpstreamRelayAuthModeManualSession,
		BearerTokenEncrypted: "enc:old-token",
		CookieEncrypted:      "enc:old-cookie",
	}

	_, _, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:     "relay",
		BaseURL:  "https://new-relay.example.com",
		AuthMode: UpstreamRelayAuthModeManualSession,
	}, existing, 99)

	require.ErrorIs(t, err, ErrUpstreamRelayInvalidManualSession)

	token := "Bearer new-token"
	connector, updated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:        "relay",
		BaseURL:     "https://new-relay.example.com",
		AuthMode:    UpstreamRelayAuthModeManualSession,
		BearerToken: &token,
	}, existing, 99)

	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, "https://new-relay.example.com", connector.BaseURL)
	require.Equal(t, "enc:new-token", connector.BearerTokenEncrypted)
	require.Empty(t, connector.CookieEncrypted)
}

func TestUpstreamRelayPasswordLoginBaseURLChangeRequiresFreshPasswordLogin(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	existing := &UpstreamRelayConnector{
		Name:                 "relay",
		BaseURL:              "https://old-relay.example.com",
		AuthMode:             UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted: "enc:old-access-token",
		LoginEmailEncrypted:  "enc:admin@example.com",
	}

	_, _, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:     "relay",
		BaseURL:  "https://new-relay.example.com",
		AuthMode: UpstreamRelayAuthModePasswordLogin,
	}, existing, 99)

	require.ErrorIs(t, err, ErrUpstreamRelayInvalidPasswordLogin)
}

func TestUpstreamRelayPasswordLoginBlankCredentialsKeepVersionStableOnUnchangedBaseURL(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	existing := &UpstreamRelayConnector{
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc:old-access-token",
		RefreshTokenEncrypted: "enc:old-refresh-token",
		LoginEmailEncrypted:   "enc:admin@example.com",
		CredentialVersion:     3,
	}

	connector, updated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:     "renamed relay",
		BaseURL:  "https://relay.example.com",
		AuthMode: UpstreamRelayAuthModePasswordLogin,
	}, existing, 99)

	require.NoError(t, err)
	require.False(t, updated)
	require.Equal(t, "renamed relay", connector.Name)
	require.Equal(t, "enc:old-access-token", connector.BearerTokenEncrypted)
	require.Equal(t, "enc:old-refresh-token", connector.RefreshTokenEncrypted)
	require.Equal(t, "enc:admin@example.com", connector.LoginEmailEncrypted)
}

func TestUpstreamRelaySwitchPasswordLoginToManualRequiresExplicitToken(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	existing := &UpstreamRelayConnector{
		Name:                  "relay",
		BaseURL:               "https://relay.example.com",
		AuthMode:              UpstreamRelayAuthModePasswordLogin,
		BearerTokenEncrypted:  "enc:old-access-token",
		RefreshTokenEncrypted: "enc:old-refresh-token",
		LoginEmailEncrypted:   "enc:admin@example.com",
	}

	_, _, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:     "relay",
		BaseURL:  "https://relay.example.com",
		AuthMode: UpstreamRelayAuthModeManualSession,
	}, existing, 99)

	require.ErrorIs(t, err, ErrUpstreamRelayInvalidManualSession)

	token := "Bearer new-manual-token"
	connector, updated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:        "relay",
		BaseURL:     "https://relay.example.com",
		AuthMode:    UpstreamRelayAuthModeManualSession,
		BearerToken: &token,
	}, existing, 99)

	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, "enc:new-manual-token", connector.BearerTokenEncrypted)
	require.Empty(t, connector.RefreshTokenEncrypted)
	require.Empty(t, connector.LoginEmailEncrypted)
}

func TestUpstreamRelaySanitizeErrorRedactsSensitiveJSONAndText(t *testing.T) {
	message := `{"error":{"message":"login failed","password":"plain-password","access_token":"secret-access","refresh_token":"secret-refresh","nested":{"cookie":"cf_clearance=secret"}}}`
	sanitized := sanitizeUpstreamRelayError(message + "\nAuthorization: Bearer header-token\npassword=plain-password token: text-token Cookie: cf_clearance=secret")

	require.Contains(t, sanitized, "[REDACTED]")
	require.NotContains(t, sanitized, "plain-password")
	require.NotContains(t, sanitized, "secret-access")
	require.NotContains(t, sanitized, "secret-refresh")
	require.NotContains(t, sanitized, "cf_clearance=secret")
	require.NotContains(t, sanitized, "header-token")
	require.NotContains(t, sanitized, "text-token")
}

func TestUpstreamRelayPasswordLoginFailureBodyIsSanitized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad login","password":"submitted-password","access_token":"leaked-token","refresh_token":"leaked-refresh"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	email := "admin@example.com"
	password := "submitted-password"
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}, httpClient: server.Client()}

	_, _, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:          "relay",
		BaseURL:       server.URL,
		AuthMode:      UpstreamRelayAuthModePasswordLogin,
		LoginEmail:    &email,
		LoginPassword: &password,
	}, nil, 99)

	require.Error(t, err)
	require.Equal(t, "UPSTREAM_RELAY_PASSWORD_LOGIN_FAILED", infraerrors.Reason(err))
	require.NotContains(t, infraerrors.Message(err), "submitted-password")
	require.NotContains(t, infraerrors.Message(err), "leaked-token")
	require.NotContains(t, infraerrors.Message(err), "leaked-refresh")
	require.Contains(t, infraerrors.Message(err), "[REDACTED]")
}

func TestUpstreamRelayGetUpstreamJSONSanitizesHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"denied","Authorization":"Bearer response-token","cookie":"cf_clearance=response-cookie","refresh_token":"response-refresh"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	svc := &UpstreamRelayGroupMonitoringService{httpClient: server.Client()}

	_, err := svc.getUpstreamJSON(context.Background(), &UpstreamRelayConnector{
		BaseURL:          server.URL,
		BearerTokenPlain: "request-token",
	}, "/api/v1/groups/available")

	require.Error(t, err)
	require.NotContains(t, err.Error(), "response-token")
	require.NotContains(t, err.Error(), "response-cookie")
	require.NotContains(t, err.Error(), "response-refresh")
	require.Contains(t, err.Error(), "[REDACTED]")
}

func TestUpstreamRelayPrepareConnectorResponseMasksStoredEncryptedCredentials(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	connector := &UpstreamRelayConnector{
		BearerTokenEncrypted:  "stored-session-token",
		RefreshTokenEncrypted: "stored-refresh-token",
		LoginEmailEncrypted:   "stored-admin@example.com",
		CookieEncrypted:       "stored-cookie-secret",
		UserAgentEncrypted:    "stored-user-agent",
	}

	svc.prepareConnectorResponse(connector)

	require.True(t, connector.HasBearerToken)
	require.True(t, connector.HasRefreshToken)
	require.True(t, connector.HasLoginEmail)
	require.True(t, connector.HasCookie)
	require.True(t, connector.HasUserAgent)
	require.Equal(t, "stor...oken", connector.BearerTokenMasked)
	require.Equal(t, "stor...oken", connector.RefreshTokenMasked)
	require.NotContains(t, connector.LoginEmailMasked, "stored-admin")
	require.Equal(t, "stor...cret", connector.CookieMasked)
	require.Equal(t, "stored-user-agent", connector.UserAgentMasked)
	require.Empty(t, connector.BearerTokenPlain)
	require.Empty(t, connector.RefreshTokenPlain)
	require.Empty(t, connector.LoginEmailPlain)
	require.Empty(t, connector.CookiePlain)
	require.Empty(t, connector.UserAgentPlain)
}

func TestUpstreamRelayBuildSuggestionsOnlyUsesMappedHealthyCandidates(t *testing.T) {
	now := time.Now()
	priority50 := 50
	priority60 := 60
	latencyFast := 40
	latencySlow := 120
	candidates := []UpstreamRelayCandidate{
		{
			ID:              1,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       101,
			UpstreamGroupID: "cheap",
			CurrentPriority: &priority50,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, LatencyMs: &latencySlow, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
		},
		{
			ID:              2,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       102,
			UpstreamGroupID: "cheaper",
			CurrentPriority: &priority60,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, LatencyMs: &latencyFast, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.6, LastSeenAt: now},
		},
		{
			ID:              3,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       103,
			UpstreamGroupID: "failed",
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: false, ErrorClass: "auth_failed"},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.1, LastSeenAt: now},
		},
		{
			ID:              4,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       104,
			UpstreamGroupID: "missing-snapshot",
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true},
		},
	}

	suggestions := buildUpstreamRelaySuggestions(candidates)

	require.Len(t, suggestions, 2)
	require.Equal(t, int64(2), suggestions[0].CandidateID)
	requireRelayNewPriority(t, suggestions[0], 10)
	require.Equal(t, 0.6, suggestions[0].FinalRateMultiplier)
	require.Equal(t, int64(1), suggestions[1].CandidateID)
	requireRelayNewPriority(t, suggestions[1], 20)
	require.Equal(t, "rate_health_priority", suggestions[0].ReasonCode)
	require.NotEmpty(t, suggestions[0].HealthSummary)
	require.NotEmpty(t, suggestions[0].RateSource)
}

func TestUpstreamRelayBuildSuggestionsSkipsStaleState(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:              1,
			ConnectorStatus: UpstreamRelayConnectorStatusNeedsReauth,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
		},
		{
			ID:              2,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, Status: upstreamRelaySnapshotStatusStale, LastSeenAt: now},
		},
		{
			ID:              3,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now.Add(-upstreamRelayProbeFreshness - time.Minute)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
		},
		{
			ID:              4,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now.Add(-upstreamRelaySnapshotFreshness - time.Minute)},
		},
	}

	require.Empty(t, buildUpstreamRelaySuggestions(candidates))
}

func TestUpstreamRelayBuildSuggestionsUsesUsageDeltaFallback(t *testing.T) {
	now := time.Now()
	priority50 := 50
	rate := 0.7
	candidates := []UpstreamRelayCandidate{
		{
			ID:              9,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       109,
			UpstreamGroupID: "usage-delta",
			CurrentPriority: &priority50,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestUsageDelta: &UpstreamRelayUsageDeltaSample{
				Status:                "reliable",
				DerivedRateMultiplier: &rate,
				SampledAt:             now,
			},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:           3,
				SuccessCount:         3,
				SuccessRate:          1,
				ConsecutiveSuccesses: 3,
				WindowMinutes:        30,
				SampleSize:           3,
			},
		},
	}

	suggestions := buildUpstreamRelaySuggestions(candidates)

	require.Len(t, suggestions, 1)
	require.Equal(t, UpstreamRelayRateSourceUsageDelta, suggestions[0].RateSource)
	require.Equal(t, "low", suggestions[0].Confidence)
	require.Equal(t, rate, suggestions[0].FinalRateMultiplier)
}

func TestUpstreamRelayShouldSampleUsageDeltaOnlyWhenFallbackIsNeeded(t *testing.T) {
	now := time.Now()
	rate := 0.7

	require.False(t, shouldSampleUsageDelta(UpstreamRelayCandidate{
		LatestSnapshot: &UpstreamRelayGroupRateSnapshot{
			FinalRateMultiplier: 0.5,
			LastSeenAt:          now,
		},
	}, now))

	require.True(t, shouldSampleUsageDelta(UpstreamRelayCandidate{
		LatestSnapshot: &UpstreamRelayGroupRateSnapshot{
			FinalRateMultiplier: 0.5,
			LastSeenAt:          now.Add(-upstreamRelaySnapshotFreshness - time.Minute),
		},
	}, now))

	require.False(t, shouldSampleUsageDelta(UpstreamRelayCandidate{
		LatestUsageDelta: &UpstreamRelayUsageDeltaSample{
			Status:                "reliable",
			DerivedRateMultiplier: &rate,
			SampledAt:             now,
		},
	}, now))

	require.True(t, shouldSampleUsageDelta(UpstreamRelayCandidate{
		LatestUsageDelta: &UpstreamRelayUsageDeltaSample{
			Status:                "reliable",
			DerivedRateMultiplier: &rate,
			SampledAt:             now.Add(-upstreamRelayUsageDeltaFreshness - time.Minute),
		},
	}, now))

	require.True(t, shouldSampleUsageDelta(UpstreamRelayCandidate{
		LatestUsageDelta: &UpstreamRelayUsageDeltaSample{
			Status:    "insufficient",
			SampledAt: now,
		},
	}, now))
}

func TestUpstreamRelayBuildSuggestionsSkipsConsecutiveFailures(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:              10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:           4,
				SuccessCount:         2,
				SuccessRate:          0.5,
				ConsecutiveFailures:  1,
				ConsecutiveSuccesses: 0,
				WindowMinutes:        30,
				SampleSize:           4,
			},
		},
	}

	require.Empty(t, buildUpstreamRelaySuggestions(candidates))
}

func TestUpstreamRelayPreviewReportsSuggestionsAndExclusions(t *testing.T) {
	now := time.Now()
	priority50 := 50
	candidates := []UpstreamRelayCandidate{
		{
			ID:              1,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       101,
			UpstreamGroupID: "cheap",
			CurrentPriority: &priority50,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
		},
		{
			ID:              2,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now.Add(-time.Hour)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.7, LastSeenAt: now},
		},
	}

	preview := buildUpstreamRelayRecommendationPreview(candidates, defaultUpstreamRelayRecommendationPolicy())

	require.Equal(t, 2, preview.TotalCandidates)
	require.Len(t, preview.Suggestions, 1)
	require.Equal(t, int64(1), preview.Suggestions[0].CandidateID)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, int64(2), preview.Exclusions[0].CandidateID)
	require.Equal(t, "stale_probe", preview.Exclusions[0].ReasonCode)
}

func TestUpstreamRelayPreviewDeduplicatesSuggestionsByAccount(t *testing.T) {
	now := time.Now()
	priority50 := 50
	candidates := []UpstreamRelayCandidate{
		{
			ID:              1,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       101,
			AccountName:     "account-a",
			UpstreamGroupID: "best",
			CurrentPriority: &priority50,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(80)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
		},
		{
			ID:              2,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       101,
			AccountName:     "account-a",
			UpstreamGroupID: "duplicate",
			CurrentPriority: &priority50,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(90)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
		},
		{
			ID:              3,
			ConnectorID:     10,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       102,
			AccountName:     "account-b",
			UpstreamGroupID: "other",
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(70)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.7, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PriorityStart = 30
	policy.PriorityStep = 10

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	require.Len(t, preview.Suggestions, 2)
	require.Equal(t, int64(1), preview.Suggestions[0].CandidateID)
	requireRelayNewPriority(t, preview.Suggestions[0], 30)
	require.Equal(t, int64(3), preview.Suggestions[1].CandidateID)
	requireRelayNewPriority(t, preview.Suggestions[1], 40)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, int64(2), preview.Exclusions[0].CandidateID)
	require.Equal(t, "duplicate_account_candidate", preview.Exclusions[0].ReasonCode)
	require.Contains(t, preview.Exclusions[0].Reason, "同一账号")
}

func TestUpstreamRelayPreviewKeepsHealthyCandidateWhenSameAccountHasFailedCandidate(t *testing.T) {
	now := time.Now()
	priority50 := 50
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountName:        "account-a",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			UpstreamGroupID:    "failed",
			CurrentPriority:    &priority50,
			Enabled:            true,
			LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.4, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountName:        "account-a",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			UpstreamGroupID:    "healthy",
			CurrentPriority:    &priority50,
			Enabled:            true,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(70)},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.6, LastSeenAt: now},
		},
		{
			ID:                 3,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountName:        "account-b",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			UpstreamGroupID:    "fallback",
			Enabled:            true,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(90)},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
		},
	}

	preview := buildUpstreamRelayRecommendationPreview(candidates, defaultUpstreamRelayRecommendationPolicy())

	require.Len(t, preview.Suggestions, 2)
	require.Equal(t, UpstreamRelaySuggestionActionPriorityUpdate, preview.Suggestions[0].ActionType)
	require.Equal(t, int64(101), preview.Suggestions[0].AccountID)
	require.Equal(t, int64(2), preview.Suggestions[0].CandidateID)
	require.Equal(t, UpstreamRelaySuggestionActionPriorityUpdate, preview.Suggestions[1].ActionType)
	require.Equal(t, int64(102), preview.Suggestions[1].AccountID)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, "latest_probe_failed", preview.Exclusions[0].ReasonCode)
	require.Equal(t, int64(1), preview.Exclusions[0].CandidateID)
}

func TestUpstreamRelayPreviewKeepsRankForUnchangedAccount(t *testing.T) {
	now := time.Now()
	priority30 := 30
	priority90 := 90
	candidates := []UpstreamRelayCandidate{
		{
			ID:              1,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       101,
			CurrentPriority: &priority30,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(80)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
		},
		{
			ID:              2,
			ConnectorStatus: UpstreamRelayConnectorStatusActive,
			AccountID:       102,
			CurrentPriority: &priority90,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now, LatencyMs: intPtr(90)},
			LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PriorityStart = 30
	policy.PriorityStep = 10

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	require.Len(t, preview.Suggestions, 1)
	require.Equal(t, int64(2), preview.Suggestions[0].CandidateID)
	requireRelayNewPriority(t, preview.Suggestions[0], 40)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, "priority_unchanged", preview.Exclusions[0].ReasonCode)
}

func TestUpstreamRelayPolicyCanAllowConsecutiveFailures(t *testing.T) {
	now := time.Now()
	candidate := UpstreamRelayCandidate{
		ID:              10,
		ConnectorStatus: UpstreamRelayConnectorStatusActive,
		Enabled:         true,
		LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
		LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.5, LastSeenAt: now},
		Health: &UpstreamRelayCandidateHealth{
			ProbeCount:          4,
			SuccessCount:        2,
			SuccessRate:         0.5,
			ConsecutiveFailures: 1,
			WindowMinutes:       30,
			SampleSize:          4,
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.ExcludeConsecutiveFailures = false

	preview := buildUpstreamRelayRecommendationPreview([]UpstreamRelayCandidate{candidate}, policy)

	require.Len(t, preview.Suggestions, 1)
	require.Empty(t, preview.Exclusions)
}

func TestNormalizeUpstreamRelayRecommendationPolicyDefaultsPauseStrategies(t *testing.T) {
	policy, err := normalizeUpstreamRelayRecommendationPolicy(UpstreamRelayRecommendationPolicy{})

	require.NoError(t, err)
	require.False(t, policy.PauseRateGapEnabled)
	require.Equal(t, 0.04, policy.PauseRateGapThreshold)
	require.False(t, policy.PauseConsecutiveFailuresEnabled)
	require.Equal(t, 3, policy.PauseConsecutiveFailuresThreshold)
	require.False(t, policy.PauseSuccessRateEnabled)
}

func TestNormalizeUpstreamRelayRecommendationPolicyRejectsInvalidPauseStrategyThresholds(t *testing.T) {
	_, err := normalizeUpstreamRelayRecommendationPolicy(UpstreamRelayRecommendationPolicy{PauseRateGapThreshold: -0.01})
	require.Error(t, err)

	_, err = normalizeUpstreamRelayRecommendationPolicy(UpstreamRelayRecommendationPolicy{PauseConsecutiveFailuresThreshold: -1})
	require.Error(t, err)
}

func TestUpstreamRelayPreviewRecommendationsDoesNotPersistRun(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	repo := &upstreamRelayRecommendationServiceRepo{
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PriorityStart = 30

	preview, err := svc.PreviewRecommendations(context.Background(), &policy)

	require.NoError(t, err)
	require.Equal(t, 0, repo.createdRuns)
	require.Len(t, preview.Suggestions, 1)
	requireRelayNewPriority(t, preview.Suggestions[0], 30)
}

func TestUpstreamRelayGetMonitoringPolicyReturnsDefaultsWithDerivedFreshness(t *testing.T) {
	repo := &upstreamRelayRecommendationServiceRepo{}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	policy, err := svc.GetMonitoringPolicy(context.Background())

	require.NoError(t, err)
	require.False(t, policy.AutoSyncEnabled)
	require.Equal(t, upstreamRelayDefaultSyncInterval, policy.SyncIntervalMinutes)
	require.Equal(t, upstreamRelayDefaultSyncInterval*3, policy.SnapshotStaleAfterMinutes)
	require.Equal(t, upstreamRelayDefaultSyncInterval*3, policy.UsageDeltaStaleAfterMinutes)
	require.Equal(t, upstreamRelayDefaultProbeInterval*3, policy.ProbeStaleAfterMinutes)
}

func TestUpstreamRelayListCandidatesMarksStaleHealthSnapshots(t *testing.T) {
	fresh := time.Now().Add(-5 * time.Minute)
	stale := time.Now().Add(-40 * time.Minute)
	repo := &upstreamRelayRecommendationServiceRepo{
		monitoring: &UpstreamRelayMonitoringPolicy{ProbeIntervalMinutes: 10},
		candidates: []UpstreamRelayCandidate{
			{ID: 1, Health: &UpstreamRelayCandidateHealth{ProbeCount: 1, CalculatedAt: fresh}},
			{ID: 2, Health: &UpstreamRelayCandidateHealth{ProbeCount: 1, CalculatedAt: stale}},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	items, _, err := svc.ListCandidates(context.Background(), 1, 20, UpstreamRelayCandidateListFilters{})

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.False(t, items[0].Health.Stale)
	require.True(t, items[1].Health.Stale)
}

func TestUpstreamRelayUpdateMonitoringPolicyReturnsDerivedFreshness(t *testing.T) {
	repo := &upstreamRelayRecommendationServiceRepo{}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	policy, err := svc.UpdateMonitoringPolicy(context.Background(), UpstreamRelayMonitoringPolicy{
		AutoSyncEnabled:             true,
		SyncIntervalMinutes:         5,
		AutoProbeEnabled:            true,
		ProbeIntervalMinutes:        2,
		FailureRetryIntervalMinutes: 1,
		SyncConcurrency:             3,
		ProbeConcurrency:            4,
	}, 88)

	require.NoError(t, err)
	require.True(t, policy.AutoSyncEnabled)
	require.Equal(t, int64(88), policy.UpdatedBy)
	require.Equal(t, 15, policy.SnapshotStaleAfterMinutes)
	require.Equal(t, 15, policy.UsageDeltaStaleAfterMinutes)
	require.Equal(t, 6, policy.ProbeStaleAfterMinutes)
}

func TestUpstreamRelayUpdateMonitoringPolicyRejectsNonPositiveIntervalsAndConcurrency(t *testing.T) {
	svc := NewUpstreamRelayGroupMonitoringService(&upstreamRelayRecommendationServiceRepo{}, nil, upstreamRelayTestEncryptor{})

	_, err := svc.UpdateMonitoringPolicy(context.Background(), UpstreamRelayMonitoringPolicy{
		SyncIntervalMinutes:         -1,
		ProbeIntervalMinutes:        2,
		FailureRetryIntervalMinutes: 1,
		SyncConcurrency:             1,
		ProbeConcurrency:            1,
	}, 88)
	require.Error(t, err)
	require.Contains(t, err.Error(), "UPSTREAM_RELAY_INVALID_MONITORING_INTERVAL")

	_, err = svc.UpdateMonitoringPolicy(context.Background(), UpstreamRelayMonitoringPolicy{
		SyncIntervalMinutes:         5,
		ProbeIntervalMinutes:        2,
		FailureRetryIntervalMinutes: 1,
		SyncConcurrency:             -1,
		ProbeConcurrency:            1,
	}, 88)
	require.Error(t, err)
	require.Contains(t, err.Error(), "UPSTREAM_RELAY_INVALID_MONITORING_CONCURRENCY")
}

func TestUpstreamRelayPreviewFreshnessIsDerivedFromMonitoringPolicy(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	repo := &upstreamRelayRecommendationServiceRepo{
		monitoring: &UpstreamRelayMonitoringPolicy{
			SyncIntervalMinutes:         5,
			ProbeIntervalMinutes:        2,
			FailureRetryIntervalMinutes: 1,
			SyncConcurrency:             1,
			ProbeConcurrency:            1,
		},
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now.Add(-7 * time.Minute)},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now.Add(-14 * time.Minute)},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	preview, err := svc.PreviewRecommendations(context.Background(), nil)

	require.NoError(t, err)
	require.Equal(t, 15, preview.Policy.SnapshotFreshnessMinutes)
	require.Equal(t, 15, preview.Policy.UsageDeltaFreshnessMinutes)
	require.Equal(t, 6, preview.Policy.ProbeFreshnessMinutes)
	require.Empty(t, preview.Suggestions)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, "stale_probe", preview.Exclusions[0].ReasonCode)
}

func TestUpstreamRelayGenerateRecommendationsUsesSavedPolicy(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PriorityStart = 40
	policy.PriorityStep = 5
	repo := &upstreamRelayRecommendationServiceRepo{
		policy: &policy,
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	run, err := svc.GenerateRecommendations(context.Background(), 88)

	require.NoError(t, err)
	require.Equal(t, 1, repo.createdRuns)
	require.Equal(t, int64(88), run.CreatedBy)
	require.Len(t, run.Suggestions, 1)
	requireRelayNewPriority(t, run.Suggestions[0], 40)
}

func TestUpstreamRelayGenerateRecommendationsFreshnessIsDerivedFromMonitoringPolicy(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.ProbeFreshnessMinutes = 120
	repo := &upstreamRelayRecommendationServiceRepo{
		policy: &policy,
		monitoring: &UpstreamRelayMonitoringPolicy{
			SyncIntervalMinutes:         5,
			ProbeIntervalMinutes:        2,
			FailureRetryIntervalMinutes: 1,
			SyncConcurrency:             1,
			ProbeConcurrency:            1,
		},
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now.Add(-7 * time.Minute)},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now.Add(-14 * time.Minute)},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	run, err := svc.GenerateRecommendations(context.Background(), 88)

	require.NoError(t, err)
	require.Equal(t, 1, repo.createdRuns)
	require.Empty(t, run.Suggestions)
	require.Equal(t, 0, run.SuggestionCount)
}

func TestUpstreamRelayGenerateAndMaybeApplyRecommendationsGenerateOnly(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	repo := &upstreamRelayRecommendationServiceRepo{
		monitoring: &UpstreamRelayMonitoringPolicy{
			SyncIntervalMinutes:             5,
			ProbeIntervalMinutes:            2,
			RecommendationIntervalMinutes:   30,
			AutoApplyRecommendationsEnabled: false,
			MaxAutoApplySuggestions:         20,
			MaxAutoApplyPriorityDelta:       100,
			MinAutoApplyConfidence:          upstreamRelayConfidenceMedium,
			FailureRetryIntervalMinutes:     1,
			SyncConcurrency:                 1,
			ProbeConcurrency:                1,
		},
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	result, err := svc.GenerateAndMaybeApplyRecommendations(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result.Run)
	require.Equal(t, 1, repo.createdRuns)
	require.Zero(t, repo.appliedRuns)
	require.False(t, result.Applied)
	require.Equal(t, "auto_apply_disabled", result.SkippedReason)
}

func TestUpstreamRelayGenerateAndMaybeApplyRecommendationsAppliesWhenGatePasses(t *testing.T) {
	now := time.Now()
	currentPriority := 50
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PriorityStart = 40
	repo := &upstreamRelayRecommendationServiceRepo{
		policy: &policy,
		monitoring: &UpstreamRelayMonitoringPolicy{
			SyncIntervalMinutes:             5,
			ProbeIntervalMinutes:            2,
			RecommendationIntervalMinutes:   30,
			AutoApplyRecommendationsEnabled: true,
			MaxAutoApplySuggestions:         20,
			MaxAutoApplyPriorityDelta:       100,
			MinAutoApplyConfidence:          upstreamRelayConfidenceMedium,
			FailureRetryIntervalMinutes:     1,
			SyncConcurrency:                 1,
			ProbeConcurrency:                1,
		},
		candidates: []UpstreamRelayCandidate{
			{
				ID:              10,
				ConnectorID:     20,
				ConnectorStatus: UpstreamRelayConnectorStatusActive,
				AccountID:       101,
				UpstreamGroupID: "cheap",
				CurrentPriority: &currentPriority,
				Enabled:         true,
				LatestProbe:     &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
				LatestSnapshot:  &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, LastSeenAt: now},
			},
		},
	}
	svc := NewUpstreamRelayGroupMonitoringService(repo, nil, upstreamRelayTestEncryptor{})

	result, err := svc.GenerateAndMaybeApplyRecommendations(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, repo.createdRuns)
	require.Equal(t, 1, repo.appliedRuns)
	require.True(t, result.Applied)
	require.Empty(t, result.SkippedReason)
	require.True(t, result.Run.Applied)
}

func TestUpstreamRelayGenerateAndMaybeApplyRecommendationsSkipsUnsafeRuns(t *testing.T) {
	oldPriority := 10
	cases := []struct {
		name        string
		policy      UpstreamRelayMonitoringPolicy
		suggestions []UpstreamRelayRecommendationSuggestion
		wantReason  string
	}{
		{
			name: "too many suggestions",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      1,
				MaxAutoApplyPriorityDelta:    100,
				MinAutoApplyConfidence:       upstreamRelayConfidenceLow,
				AllowAutoApplyDegradedHealth: true,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{OldPriority: &oldPriority, NewPriority: intPtr(20), Confidence: upstreamRelayConfidenceHigh, HealthStatus: "healthy"},
				{OldPriority: &oldPriority, NewPriority: intPtr(30), Confidence: upstreamRelayConfidenceHigh, HealthStatus: "healthy"},
			},
			wantReason: "too_many_suggestions",
		},
		{
			name: "priority delta exceeded",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      5,
				MaxAutoApplyPriorityDelta:    5,
				MinAutoApplyConfidence:       upstreamRelayConfidenceLow,
				AllowAutoApplyDegradedHealth: true,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{OldPriority: &oldPriority, NewPriority: intPtr(20), Confidence: upstreamRelayConfidenceHigh, HealthStatus: "healthy"},
			},
			wantReason: "priority_delta_exceeded",
		},
		{
			name: "zero priority delta rejects any change",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      5,
				MaxAutoApplyPriorityDelta:    0,
				MinAutoApplyConfidence:       upstreamRelayConfidenceLow,
				AllowAutoApplyDegradedHealth: true,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{OldPriority: &oldPriority, NewPriority: intPtr(11), Confidence: upstreamRelayConfidenceHigh, HealthStatus: "healthy"},
			},
			wantReason: "priority_delta_exceeded",
		},
		{
			name: "confidence below threshold",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      5,
				MaxAutoApplyPriorityDelta:    100,
				MinAutoApplyConfidence:       upstreamRelayConfidenceMedium,
				AllowAutoApplyDegradedHealth: true,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{OldPriority: &oldPriority, NewPriority: intPtr(20), Confidence: upstreamRelayConfidenceLow, HealthStatus: "healthy"},
			},
			wantReason: "confidence_below_threshold",
		},
		{
			name: "degraded health",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      5,
				MaxAutoApplyPriorityDelta:    100,
				MinAutoApplyConfidence:       upstreamRelayConfidenceLow,
				AllowAutoApplyDegradedHealth: false,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{OldPriority: &oldPriority, NewPriority: intPtr(20), Confidence: upstreamRelayConfidenceHigh, HealthStatus: upstreamRelayHealthDegraded},
			},
			wantReason: "degraded_health",
		},
		{
			name: "account gate suggestions require manual apply",
			policy: UpstreamRelayMonitoringPolicy{
				MaxAutoApplySuggestions:      5,
				MaxAutoApplyPriorityDelta:    100,
				MinAutoApplyConfidence:       upstreamRelayConfidenceLow,
				AllowAutoApplyDegradedHealth: true,
			},
			suggestions: []UpstreamRelayRecommendationSuggestion{
				{ActionType: UpstreamRelaySuggestionActionAccountPause, OldSchedulable: boolPtr(true), NewSchedulable: boolPtr(false), Confidence: upstreamRelayConfidenceHigh, HealthStatus: "healthy"},
			},
			wantReason: "account_gate_suggestion_requires_manual_apply",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := tc.policy
			policy.AutoApplyRecommendationsEnabled = true
			run := &UpstreamRelayRecommendationRun{
				Status:      UpstreamRelayRunStatusSuccess,
				Suggestions: tc.suggestions,
			}

			reason := validateUpstreamRelayAutoApplyRun(run, policy)

			require.Equal(t, tc.wantReason, reason)
		})
	}
}

func TestUpstreamRelayRecommendationPreviewDoesNotPauseWhenStrategiesDisabled(t *testing.T) {
	now := time.Now()
	priority := 50
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountName:        "高风险账号",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			CurrentPriority:    &priority,
			Enabled:            true,
			UpstreamGroupID:    "expensive",
			LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:          3,
				SuccessCount:        2,
				SuccessRate:         0.67,
				ConsecutiveFailures: 1,
				WindowMinutes:       30,
				SampleSize:          3,
			},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "healthy",
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}

	preview := buildUpstreamRelayRecommendationPreview(candidates, defaultUpstreamRelayRecommendationPolicy())

	for _, suggestion := range preview.Suggestions {
		require.NotEqual(t, UpstreamRelaySuggestionActionAccountPause, suggestion.ActionType)
	}
	require.NotEmpty(t, preview.Exclusions)
}

func TestUpstreamRelayRecommendationPreviewCreatesSingleAccountPauseWhenAllCandidatesExcluded(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountName:        "高风险账号",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "failed",
			LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:          3,
				SuccessCount:        0,
				SuccessRate:         0,
				ConsecutiveFailures: 3,
				WindowMinutes:       30,
				SampleSize:          3,
			},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountName:        "高风险账号",
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "stale",
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now.Add(-time.Hour)},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 3,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "healthy",
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}

	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PauseConsecutiveFailuresEnabled = true
	policy.PauseConsecutiveFailuresThreshold = 3

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	pauseCount := 0
	for _, suggestion := range preview.Suggestions {
		if suggestion.ActionType == UpstreamRelaySuggestionActionAccountPause {
			pauseCount++
			require.Equal(t, int64(101), suggestion.AccountID)
		}
	}
	require.Equal(t, 1, pauseCount)
	require.Len(t, preview.Exclusions, 1)
	require.Equal(t, int64(2), preview.Exclusions[0].CandidateID)
	require.Equal(t, "stale_probe", preview.Exclusions[0].ReasonCode)
}

func TestUpstreamRelayRecommendationPreviewSkipsConsecutiveFailurePauseBelowThreshold(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "failed",
			LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:          3,
				SuccessCount:        1,
				SuccessRate:         0.33,
				ConsecutiveFailures: 2,
				WindowMinutes:       30,
				SampleSize:          3,
			},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "healthy",
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PauseConsecutiveFailuresEnabled = true
	policy.PauseConsecutiveFailuresThreshold = 3

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	for _, suggestion := range preview.Suggestions {
		require.NotEqual(t, UpstreamRelaySuggestionActionAccountPause, suggestion.ActionType)
	}
	require.NotEmpty(t, preview.Exclusions)
}

func TestUpstreamRelayRecommendationPreviewCreatesPauseForLowSuccessRateStrategy(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "low-success",
			LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
			Health: &UpstreamRelayCandidateHealth{
				ProbeCount:          4,
				SuccessCount:        1,
				SuccessRate:         0.25,
				ConsecutiveFailures: 1,
				WindowMinutes:       30,
				SampleSize:          4,
			},
			LatestSnapshot: &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.5, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "healthy",
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PauseSuccessRateEnabled = true

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	require.NotEmpty(t, preview.Suggestions)
	var pause *UpstreamRelayRecommendationSuggestion
	for i := range preview.Suggestions {
		if preview.Suggestions[i].ActionType == UpstreamRelaySuggestionActionAccountPause {
			pause = &preview.Suggestions[i]
			break
		}
	}
	require.NotNil(t, pause)
	require.Equal(t, "account_gate_success_rate_below_threshold", pause.ReasonCode)
	require.Contains(t, pause.Reason, "成功率")
}

func TestUpstreamRelayRecommendationPreviewCreatesPauseForReplaceableRateGap(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "expensive",
			ProbeModel:         "gpt-5.5",
			ProbeProtocol:      MonitorAPIModeChatCompletions,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "cheap",
			ProbeModel:         "gpt-5.5",
			ProbeProtocol:      MonitorAPIModeChatCompletions,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.0, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PauseRateGapEnabled = true
	policy.PauseRateGapThreshold = 0.04

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	var pause *UpstreamRelayRecommendationSuggestion
	for i := range preview.Suggestions {
		if preview.Suggestions[i].ActionType == UpstreamRelaySuggestionActionAccountPause {
			pause = &preview.Suggestions[i]
			break
		}
	}
	require.NotNil(t, pause)
	require.Equal(t, int64(101), pause.AccountID)
	require.Equal(t, "account_gate_rate_gap_exceeded", pause.ReasonCode)
	require.Contains(t, pause.Reason, "差值")
	require.Contains(t, pause.Reason, "策略阈值")
}

func TestUpstreamRelayRecommendationPreviewSkipsRateGapPauseWhenAccountHasUncoveredHealthyCandidate(t *testing.T) {
	now := time.Now()
	candidates := []UpstreamRelayCandidate{
		{
			ID:                 1,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "expensive",
			ProbeModel:         "gpt-5.5",
			ProbeProtocol:      MonitorAPIModeChatCompletions,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 2,
			ConnectorID:        10,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          101,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "uncovered",
			ProbeModel:         "gpt-5.5-large",
			ProbeProtocol:      MonitorAPIModeChatCompletions,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.1, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
		{
			ID:                 3,
			ConnectorID:        11,
			ConnectorStatus:    UpstreamRelayConnectorStatusActive,
			AccountID:          102,
			AccountPlatform:    PlatformOpenAI,
			AccountSchedulable: true,
			Enabled:            true,
			UpstreamGroupID:    "cheap",
			ProbeModel:         "gpt-5.5",
			ProbeProtocol:      MonitorAPIModeChatCompletions,
			LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
			LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 1.0, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
		},
	}
	policy := defaultUpstreamRelayRecommendationPolicy()
	policy.PauseRateGapEnabled = true
	policy.PauseRateGapThreshold = 0.04

	preview := buildUpstreamRelayRecommendationPreview(candidates, policy)

	for _, suggestion := range preview.Suggestions {
		require.NotEqual(t, UpstreamRelaySuggestionActionAccountPause, suggestion.ActionType)
	}
}

func TestUpstreamRelayRecommendationPreviewSkipsPauseForLastSchedulablePlatformAccount(t *testing.T) {
	now := time.Now()
	candidate := UpstreamRelayCandidate{
		ID:                 1,
		ConnectorID:        10,
		ConnectorStatus:    UpstreamRelayConnectorStatusActive,
		AccountID:          101,
		AccountPlatform:    PlatformOpenAI,
		AccountSchedulable: true,
		Enabled:            true,
		UpstreamGroupID:    "failed",
		LatestProbe:        &UpstreamRelayProbeResult{Success: false, ProbedAt: now},
		LatestSnapshot:     &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 2, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
	}

	preview := buildUpstreamRelayRecommendationPreview([]UpstreamRelayCandidate{candidate}, defaultUpstreamRelayRecommendationPolicy())

	for _, suggestion := range preview.Suggestions {
		require.NotEqual(t, UpstreamRelaySuggestionActionAccountPause, suggestion.ActionType)
	}
	require.Len(t, preview.Exclusions, 1)
}

func TestUpstreamRelayRecommendationPreviewCreatesAccountResumeSuggestion(t *testing.T) {
	now := time.Now()
	candidate := UpstreamRelayCandidate{
		ID:                 1,
		ConnectorID:        10,
		ConnectorStatus:    UpstreamRelayConnectorStatusActive,
		AccountID:          101,
		AccountName:        "已恢复账号",
		AccountPlatform:    PlatformOpenAI,
		AccountSchedulable: false,
		AccountGateActive:  true,
		Enabled:            true,
		UpstreamGroupID:    "recovered",
		LatestProbe:        &UpstreamRelayProbeResult{Success: true, ProbedAt: now},
		Health: &UpstreamRelayCandidateHealth{
			ProbeCount:           3,
			SuccessCount:         3,
			SuccessRate:          1,
			ConsecutiveSuccesses: 3,
			WindowMinutes:        30,
			SampleSize:           3,
		},
		LatestSnapshot: &UpstreamRelayGroupRateSnapshot{FinalRateMultiplier: 0.8, Source: UpstreamRelayRateSourceOverride, LastSeenAt: now},
	}

	preview := buildUpstreamRelayRecommendationPreview([]UpstreamRelayCandidate{candidate}, defaultUpstreamRelayRecommendationPolicy())

	require.Len(t, preview.Suggestions, 1)
	require.Equal(t, UpstreamRelaySuggestionActionAccountResume, preview.Suggestions[0].ActionType)
	require.Equal(t, boolPtr(false), preview.Suggestions[0].OldSchedulable)
	require.Equal(t, boolPtr(true), preview.Suggestions[0].NewSchedulable)
}

func TestUpstreamRelayUsageDeltaSampleReliability(t *testing.T) {
	probeID := int64(77)
	probe := &UpstreamRelayProbeResult{ID: probeID, Success: true}
	sample := buildUsageDeltaSample(
		UpstreamRelayCandidate{ID: 1, ProbeModel: "gpt-5.5"},
		probe,
		&upstreamRelayUsageSnapshot{Cost: 10, ActualCost: 2},
		nil,
		&upstreamRelayUsageSnapshot{Cost: 14, ActualCost: 3},
		nil,
	)

	require.NotNil(t, sample)
	require.Equal(t, "reliable", sample.Status)
	require.NotNil(t, sample.DerivedRateMultiplier)
	require.Equal(t, 4.0, *sample.DerivedRateMultiplier)
	require.Equal(t, probeID, *sample.ProbeResultID)

	insufficient := buildUsageDeltaSample(
		UpstreamRelayCandidate{ID: 1, ProbeModel: "gpt-5.5"},
		probe,
		&upstreamRelayUsageSnapshot{Cost: 10, ActualCost: 2},
		nil,
		&upstreamRelayUsageSnapshot{Cost: 10, ActualCost: 2},
		nil,
	)
	require.Equal(t, "insufficient", insufficient.Status)
	require.Contains(t, insufficient.UnreliableReason, "not positive")
}

func TestUpstreamRelayCandidateProbeReusesAccountTestService(t *testing.T) {
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	accountTest := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	svc := &UpstreamRelayGroupMonitoringService{accountTestService: accountTest}
	account := &Account{
		ID:          91,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example/v1",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	result := svc.runCandidateProbe(context.Background(), UpstreamRelayCandidate{
		ID:          12,
		ProbeModel:  "gpt-5.4",
		AccountID:   account.ID,
		ConnectorID: 3,
	}, account)

	require.True(t, result.Success)
	require.Equal(t, int64(12), result.CandidateID)
	require.Empty(t, result.ErrorClass)
	require.NotNil(t, result.LatencyMs)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestUpstreamRelayCandidateProbeHonorsResponsesProtocolThroughAccountTest(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(`data: {"type":"response.output_text.delta","delta":"pong"}

data: {"type":"response.completed"}

`)),
	}}
	accountTest := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	svc := &UpstreamRelayGroupMonitoringService{accountTestService: accountTest}
	account := &Account{
		ID:          92,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example/v1",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	result := svc.runCandidateProbe(context.Background(), UpstreamRelayCandidate{
		ID:            13,
		ProbeModel:    "gpt-5.4",
		ProbeProtocol: MonitorAPIModeResponses,
	}, account)

	require.True(t, result.Success)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://compat-upstream.example/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
}

func TestUpstreamRelayCandidateProbeHonorsAnthropicProtocolThroughAccountTest(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"pong"}}

data: {"type":"message_stop"}

`)),
	}}
	accountTest := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	svc := &UpstreamRelayGroupMonitoringService{accountTestService: accountTest}
	account := &Account{
		ID:          93,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-ant-test",
			"base_url": "https://anthropic-upstream.example",
		},
	}

	result := svc.runCandidateProbe(context.Background(), UpstreamRelayCandidate{
		ID:            14,
		ProbeModel:    "claude-sonnet-4-5",
		ProbeProtocol: UpstreamRelayProbeProtocolAnthropic,
	}, account)

	require.True(t, result.Success)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://anthropic-upstream.example/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, "sk-ant-test", upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", upstream.lastReq.Header.Get("anthropic-version"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("anthropic-beta"))
	require.Equal(t, "claude-sonnet-4-5", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, "system").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestUpstreamRelayCandidateProbeRejectsAnthropicProtocolForNonAnthropicAccount(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{accountTestService: &AccountTestService{}}
	account := &Account{
		ID:       94,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	result := svc.runCandidateProbe(context.Background(), UpstreamRelayCandidate{
		ID:            15,
		ProbeModel:    "claude-sonnet-4-5",
		ProbeProtocol: UpstreamRelayProbeProtocolAnthropic,
	}, account)

	require.False(t, result.Success)
	require.Equal(t, "invalid_request", result.ErrorClass)
	require.Contains(t, result.ErrorMessage, "anthropic account")
}

func TestUpstreamRelayCandidateProbeRequiresAccountTestService(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{}
	account := &Account{
		ID:          91,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://compat-upstream.example"},
	}

	result := svc.runCandidateProbe(context.Background(), UpstreamRelayCandidate{ID: 12, ProbeModel: "gpt-5.4"}, account)

	require.False(t, result.Success)
	require.Equal(t, "probe_unavailable", result.ErrorClass)
	require.Contains(t, result.ErrorMessage, "account test service")
}

func TestUpstreamRelayClassifyProbeErrorPhase2Categories(t *testing.T) {
	require.Equal(t, "insufficient_quota", classifyProbeError(http.StatusPaymentRequired, "insufficient quota balance"))
	require.Equal(t, "context_window_exceeded", classifyProbeError(http.StatusBadRequest, "input exceeds the context window"))
	require.Equal(t, "browser_challenge", classifyProbeError(http.StatusForbidden, "<html>cloudflare turnstile</html>"))
	require.Equal(t, "network_error", classifyProbeError(0, "dial tcp: no such host"))
	require.Equal(t, "invalid_request", classifyProbeError(http.StatusBadRequest, "invalid request body"))
}
