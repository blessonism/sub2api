package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type modelRouteHandlerGroupRepo struct {
	service.GroupRepository
	mu     sync.Mutex
	groups map[int64]*service.Group
}

func (r *modelRouteHandlerGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	group := r.groups[id]
	if group == nil {
		return nil, service.ErrGroupNotFound
	}
	clone := *group
	return &clone, nil
}
func (r *modelRouteHandlerGroupRepo) rate(id int64, rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[id].RateMultiplier = rate
}

func routedHandlerGroups(sourceID int64, model string) (*service.Group, *modelRouteHandlerGroupRepo) {
	source := &service.Group{ID: sourceID, Name: "source", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.2, Hydrated: true,
		ModelGroupRoutes: []service.ModelGroupRoute{{Model: model, TargetGroupID: 40, Mode: "target_only", Enabled: true}}}
	target := &service.Group{ID: 40, Name: "Luna", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3, Hydrated: true}
	return source, &modelRouteHandlerGroupRepo{groups: map[int64]*service.Group{sourceID: source, 40: target}}
}

type modelRouteHTTPUpstream struct {
	service.HTTPUpstream
	stream  bool
	calls   int
	payload []byte
}

func (u *modelRouteHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	u.payload, _ = io.ReadAll(req.Body)
	body := `{"id":"resp_route_http","object":"response","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1}}`
	contentType := "application/json"
	if u.stream {
		contentType = "text/event-stream"
		body = "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + body + "}\n\n"
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestModelGroupRoutesHTTPActualForwardAndUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonstream", true: "stream"}[stream], func(t *testing.T) {
			source, groups := routedHandlerGroups(51, "gpt-5.1")
			source.SystemPromptInjection = service.GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "source-only-prompt"}
			groups.groups[40].SystemPromptInjection = service.GroupSystemPromptInjection{Enabled: true, Mode: "append", Content: "target-only-prompt"}
			key := &service.APIKey{ID: 1801, UserID: 1701, GroupID: &source.ID, Group: source, User: &service.User{ID: 1701, Status: service.StatusActive}}
			accounts := &openAIWSUsageHandlerAccountRepoStub{account: service.Account{ID: 230, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{40}, Credentials: map[string]any{"api_key": "test-upstream", "base_url": "https://upstream.example.test"}, Extra: map[string]any{"openai_passthrough": true}}}
			logs := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			cfg.Security.URLAllowlist.Enabled = false
			upstream := &modelRouteHTTPUpstream{stream: stream}
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			gateway := service.NewOpenAIGatewayService(accounts, logs, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, nil, cfg)
			admission := &GatewayHandler{modelGroupRoutes: service.NewModelGroupRouteResolver(groups, gateway)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1701})
				ctx := context.WithValue(c.Request.Context(), ctxkey.UserID, int64(1701))
				ctx = service.WithGroupSystemPromptInjection(ctx, source)
				c.Request = c.Request.WithContext(ctx)
			}, middleware.GroupModelAllowlist(), admission.ModelGroupRoutes(nil))
			router.POST("/v1/responses", h.Responses)
			payload := `{"model":"gpt-5.1","input":"hello","stream":false}`
			if stream {
				payload = strings.Replace(payload, "false", "true", 1)
			}
			request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(payload))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, 1, upstream.calls)
			require.Equal(t, "gpt-5.1", gjson.GetBytes(upstream.payload, "model").String())
			require.Contains(t, string(upstream.payload), "source-only-prompt")
			require.NotContains(t, string(upstream.payload), "target-only-prompt")
			select {
			case log := <-logs.created:
				require.Equal(t, int64(40), *log.GroupID)
				require.Equal(t, int64(51), *log.SourceGroupID)
				require.Equal(t, key.ID, log.APIKeyID)
				require.Equal(t, 0.3, log.RateMultiplier)
			case <-time.After(3 * time.Second):
				t.Fatal("usage record missing")
			}
			require.Empty(t, logs.created, "不能重复写用量或扣费")
			require.Equal(t, int64(51), *key.GroupID)
		})
	}
}

func TestModelGroupRoutesWebSocketMultiTurnPriceRefresh(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			source, groups := routedHandlerGroups(4201, "gpt-6-luna")
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				group: source, modelRouteGroups: groups, ingressMode: mode,
				firstPayload:              `{"type":"response.create","model":"gpt-6-luna","stream":false}`,
				secondPayload:             `{"type":"response.create","model":"gpt-6-luna","stream":false}`,
				afterFirstUpstreamRequest: func(_ *service.ChannelService) error { groups.rate(40, 0.4); return nil },
			})
			require.Len(t, got.logs, 2)
			require.Equal(t, 0.3, got.logs[0].RateMultiplier)
			require.Equal(t, 0.4, got.logs[1].RateMultiplier)
			for _, log := range got.logs {
				require.Equal(t, int64(40), *log.GroupID)
				require.Equal(t, int64(4201), *log.SourceGroupID)
			}
		})
	}
}

func TestModelGroupRoutesWebSocketGroupSwitchRequiresReconnect(t *testing.T) {
	source, groups := routedHandlerGroups(4201, "gpt-6-luna")
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		group: source, modelRouteGroups: groups,
		firstPayload:            `{"type":"response.create","model":"gpt-6-luna","stream":false}`,
		secondPayload:           `{"type":"response.create","model":"gpt-6-sol","stream":false}`,
		secondTurnCloseExpected: true, closeReason: "model switch requires reconnect",
	})
}

func TestModelGroupRoutesWebSocketLateEnableRequiresReconnect(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			repo := newWSGroupPricingAPIKeyRepoStub(0.2)
			apiKeys := newWSGroupPricingAPIKeyService(repo)
			_, groups := routedHandlerGroups(wsGroupPricingGroupID, "gpt-5.6-sol")
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				modelRouteGroups: groups, ingressMode: mode,
				apiKeyService: apiKeys, apiKeyCredential: wsGroupPricingKey,
				firstPayload: wsGroupPricingFirst, secondPayload: wsGroupPricingSecond,
				afterFirstUpstreamRequest: func(*service.ChannelService) error {
					return repo.adminUpdate(apiKeys, func(_ *service.APIKey, group *service.Group) {
						group.ModelGroupRoutes = []service.ModelGroupRoute{{Model: "gpt-5.6-sol", TargetGroupID: 40, Mode: "target_only", Enabled: true}}
					})
				},
				secondTurnCloseExpected: true, closeReason: "model switch requires reconnect",
			})
		})
	}
}

func TestModelGroupRoutesWebSocketRefreshKeepsConnectionIdentityAndLimits(t *testing.T) {
	repo := newWSGroupPricingAPIKeyRepoStub(0.2)
	source, groups := routedHandlerGroups(wsGroupPricingGroupID, "gpt-6-luna")
	repo.group = *source
	repo.apiKey.Quota = 10
	repo.apiKey.User.Concurrency = 2
	apiKeys := newWSGroupPricingAPIKeyService(repo)
	sourceKey, err := apiKeys.GetByKey(context.Background(), wsGroupPricingKey)
	require.NoError(t, err)
	resolver := service.NewModelGroupRouteResolver(groups, nil)
	req := service.ModelGroupRouteRequest{Model: "gpt-6-luna"}
	conn, err := resolver.Resolve(context.Background(), sourceKey, req)
	require.NoError(t, err)
	require.NoError(t, repo.adminUpdate(apiKeys, func(key *service.APIKey, _ *service.Group) {
		key.Quota = 999
		key.User.Concurrency = 9
	}))
	groups.rate(40, 0.4)
	h := &OpenAIGatewayHandler{modelGroupRoutes: resolver, apiKeyService: apiKeys}
	turn, err := h.resolveModelGroupRouteWSTurn(context.Background(), sourceKey, conn, req, []string{req.Model})
	require.NoError(t, err)
	require.Equal(t, 0.4, turn.Group.RateMultiplier)
	require.Equal(t, conn.Quota, turn.Quota)
	require.Same(t, conn.User, turn.User)
	require.Equal(t, 2, turn.User.Concurrency)
	require.Equal(t, *conn.GroupID, *turn.GroupID)
	require.Equal(t, *conn.SourceGroupID, *turn.SourceGroupID)
}

type modelRouteCatalogAccounts struct{ gatewayModelsAccountRepoStub }

func (r *modelRouteCatalogAccounts) ListByGroup(_ context.Context, id int64) ([]service.Account, error) {
	return r.byGroup[id], nil
}

func (r *modelRouteCatalogAccounts) ListSchedulableByGroupIDAndPlatform(ctx context.Context, id int64, _ string) ([]service.Account, error) {
	return r.ListSchedulableByGroupID(ctx, id)
}

func TestModelGroupRoutesCatalogDoesNotRestoreUnavailableRouteFromDefaults(t *testing.T) {
	for _, unavailable := range []string{"disabled", "deleted", "permission revoked"} {
		t.Run(unavailable, func(t *testing.T) {
			source, groups := routedHandlerGroups(51, "gpt-6-luna")
			source.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-luna"}}
			user := &service.User{ID: 2}
			switch unavailable {
			case "disabled":
				groups.groups[40].Status = service.StatusDisabled
			case "deleted":
				delete(groups.groups, 40)
			case "permission revoked":
				user.RestrictPublicGroups = true
				user.AllowedGroups = []int64{51}
			}
			repo := &modelRouteCatalogAccounts{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
				51: {{ID: 248, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-luna": "gpt-6-luna"}}}},
			}}}
			h := newGatewayModelsHandlerForTest(repo)
			gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h.modelGroupRoutes = service.NewModelGroupRouteResolver(groups, gateway)
			for _, model := range []string{"", "gpt-6-luna"} {
				rec := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(rec)
				ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
				ctx.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, GroupID: &source.ID, Group: source, User: user})
				if model != "" {
					ctx.Params = gin.Params{{Key: "model", Value: model}}
				}
				h.Models(ctx)
				if model == "" {
					require.Equal(t, http.StatusOK, rec.Code)
					require.Empty(t, gjson.GetBytes(rec.Body.Bytes(), "data").Array(), rec.Body.String())
				} else {
					require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestModelGroupRoutesPinnedCatalogWithoutLocalCandidates(t *testing.T) {
	source, groups := routedHandlerGroups(51, "gpt-6-luna")
	source.CodexModelsManifestConfig = service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{248}}
	repo := &modelRouteCatalogAccounts{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		40: {{ID: 230, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-luna": "gpt-6-luna"}}}},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h.modelGroupRoutes = service.NewModelGroupRouteResolver(groups, gateway)
	h.openAIGatewayService = gateway
	codex := &OpenAIGatewayHandler{gatewayService: gateway, modelGroupRoutes: h.modelGroupRoutes, modelGroupRouteCatalog: h.gatewayService}
	for _, endpoint := range []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{"ordinary", h.Models}, {"codex", codex.CodexModels},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/models", nil)
			ctx.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, GroupID: &source.ID, Group: source, User: &service.User{ID: 2}})
			endpoint.handler(ctx)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "gpt-6-luna")
		})
	}
}

func (r *modelRouteCatalogAccounts) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, accounts := range r.byGroup {
		for _, account := range accounts {
			if account.ID == id {
				copy := account
				return &copy, nil
			}
		}
	}
	return nil, nil
}

func TestModelGroupRoutesCatalogAndTargetMetadata(t *testing.T) {
	source, groups := routedHandlerGroups(51, "gpt-6-luna")
	source.ModelGroupRoutes = append(source.ModelGroupRoutes, service.ModelGroupRoute{Model: "gpt-5.6-luna", TargetGroupID: 40, Mode: "target_only", Enabled: true})
	local := service.Account{ID: 248, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-sol": "gpt-6-sol"}}}
	target := service.Account{ID: 230, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-luna": "gpt-6-luna", "gpt-5.6-luna": "gpt-5.6-luna"}}}
	target.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Models: map[string]service.UpstreamModelMetadata{
		"gpt-6-luna": {InputModalities: []string{"text"}, SupportedReasoningLevels: []string{"low", "high"}, ContextWindow: 123456},
	}})
	repo := &modelRouteCatalogAccounts{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{51: {local}, 40: {target}}}}
	generic := newGatewayModelsHandlerForTest(repo)
	openai := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	resolver := service.NewModelGroupRouteResolver(groups, openai)
	generic.modelGroupRoutes = resolver
	codex := &OpenAIGatewayHandler{gatewayService: openai, modelGroupRoutes: resolver, modelGroupRouteCatalog: generic.gatewayService}
	key := &service.APIKey{ID: 1, GroupID: &source.ID, Group: source, User: &service.User{ID: 2}}
	request := func(path, model, etag string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
		ctx.Request.Header.Set("If-None-Match", etag)
		ctx.Set(string(middleware.ContextKeyAPIKey), key)
		if model != "" {
			ctx.Params = gin.Params{{Key: "model", Value: model}}
		}
		handler(ctx)
		return rec
	}
	listed := request("/v1/models", "", "", generic.Models)
	require.Equal(t, 200, listed.Code)
	require.Contains(t, listed.Body.String(), "gpt-6-luna")
	require.Contains(t, listed.Body.String(), "gpt-5.6-luna")
	require.Contains(t, listed.Body.String(), "gpt-6-sol")
	single := request("/v1/models/gpt-6-luna", "gpt-6-luna", "", generic.Models)
	require.Equal(t, 200, single.Code)
	require.Equal(t, "gpt-6-luna", gjson.GetBytes(single.Body.Bytes(), "id").String())
	manifest := request("/models?client_version=0.153.0", "", "", codex.CodexModels)
	require.Equal(t, 200, manifest.Code, manifest.Body.String())
	luna := gjson.GetBytes(manifest.Body.Bytes(), `models.#(slug=="gpt-6-luna")`)
	require.True(t, luna.Exists())
	require.Equal(t, int64(123456), luna.Get("context_window").Int())
	require.Equal(t, []any{"text"}, luna.Get("input_modalities").Value())
	cached := request("/models?client_version=0.153.0", "", manifest.Header().Get("ETag"), codex.CodexModels)
	require.Equal(t, http.StatusNotModified, cached.Code)
	groups.groups[40].Status = service.StatusDisabled
	removed := request("/models?client_version=0.153.0", "", manifest.Header().Get("ETag"), codex.CodexModels)
	require.Equal(t, 200, removed.Code)
	require.NotContains(t, removed.Body.String(), `"slug":"gpt-6-luna"`)
}

func TestModelGroupRoutesPreservesLiveSourceCodexMetadata(t *testing.T) {
	source, groups := routedHandlerGroups(51, "gpt-6-luna")
	repo := &modelRouteCatalogAccounts{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		51: {{ID: 248, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{51}, Credentials: map[string]any{"api_key": "source-key", "base_url": "https://source.example.test"}}},
		40: {{ID: 230, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{40}, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-luna": "gpt-6-luna"}}}},
	}}}
	upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{248: `{"models":[{"slug":"gpt-6-sol","display_name":"source-live","context_window":765432,"supported_reasoning_levels":[{"effort":"high"}],"input_modalities":["text"]}]}`}}
	generic := newGatewayModelsHandlerForTest(repo)
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &OpenAIGatewayHandler{gatewayService: gateway, modelGroupRoutes: service.NewModelGroupRouteResolver(groups, gateway), modelGroupRouteCatalog: generic.gatewayService, maxAccountSwitches: 1}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.153.0", nil)
	ctx.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, GroupID: &source.ID, Group: source, User: &service.User{ID: 2}})
	h.CodexModels(ctx)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, int64(765432), gjson.GetBytes(rec.Body.Bytes(), `models.#(slug=="gpt-6-sol").context_window`).Int())
	require.True(t, gjson.GetBytes(rec.Body.Bytes(), `models.#(slug=="gpt-6-luna")`).Exists())
	require.Equal(t, []int64{248}, upstream.accountIDs())
}
