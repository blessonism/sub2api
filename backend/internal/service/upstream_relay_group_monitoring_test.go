package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type upstreamRelayTestEncryptor struct{}

func (upstreamRelayTestEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (upstreamRelayTestEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestUpstreamRelayParseGroupRatesAllowsEmptyObject(t *testing.T) {
	rates, err := parseGroupRates([]byte(`{}`))

	require.NoError(t, err)
	require.Empty(t, rates)
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

func TestUpstreamRelayConnectorInputEncryptsAndResponseRedactsSecrets(t *testing.T) {
	svc := &UpstreamRelayGroupMonitoringService{encryptor: upstreamRelayTestEncryptor{}}
	token := "Bearer sk-live-secret-token"
	cookie := "cf_clearance=super-secret-cookie"
	userAgent := "Mozilla/5.0 relay integration browser"

	connector, credentialsUpdated, err := svc.normalizeConnectorInput(context.Background(), UpstreamRelayConnectorInput{
		Name:        "relay",
		BaseURL:     "https://relay.example.com/",
		AuthMode:    UpstreamRelayAuthModeManualSession,
		BearerToken: &token,
		Cookie:      &cookie,
		UserAgent:   &userAgent,
	}, nil, 99)

	require.NoError(t, err)
	require.True(t, credentialsUpdated)
	require.Equal(t, "https://relay.example.com", connector.BaseURL)
	require.Equal(t, "enc:sk-live-secret-token", connector.BearerTokenEncrypted)
	require.Equal(t, "enc:cf_clearance=super-secret-cookie", connector.CookieEncrypted)
	require.Equal(t, "enc:Mozilla/5.0 relay integration browser", connector.UserAgentEncrypted)

	connector.BearerTokenPlain = "sk-live-secret-token"
	connector.CookiePlain = "cf_clearance=super-secret-cookie"
	connector.UserAgentPlain = userAgent
	svc.prepareConnectorResponse(connector)

	require.True(t, connector.HasBearerToken)
	require.True(t, connector.HasCookie)
	require.True(t, connector.HasUserAgent)
	require.Equal(t, "sk-l...oken", connector.BearerTokenMasked)
	require.NotContains(t, connector.CookieMasked, "super-secret-cookie")
	require.Empty(t, connector.BearerTokenPlain)
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
			TargetGroupID:   7,
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
			TargetGroupID:   7,
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
			TargetGroupID:   7,
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
			TargetGroupID:   7,
			Enabled:         true,
			LatestProbe:     &UpstreamRelayProbeResult{Success: true},
		},
	}

	suggestions := buildUpstreamRelaySuggestions(candidates)

	require.Len(t, suggestions, 2)
	require.Equal(t, int64(2), suggestions[0].CandidateID)
	require.Equal(t, 10, suggestions[0].NewPriority)
	require.Equal(t, 0.6, suggestions[0].FinalRateMultiplier)
	require.Equal(t, int64(1), suggestions[1].CandidateID)
	require.Equal(t, 20, suggestions[1].NewPriority)
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
