package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func hasEnabledModelGroupRoutes(group *service.Group) bool {
	if group == nil {
		return false
	}
	for _, rule := range group.ModelGroupRoutes {
		if rule.Enabled {
			return true
		}
	}
	return false
}

// ModelGroupRoutes 在所有 HTTP 别名共用的链上挂载；WS 在首帧读取模型后解析。
func (h *GatewayHandler) ModelGroupRoutes(openai *OpenAIGatewayHandler) gin.HandlerFunc {
	if h != nil && h.gatewayService != nil && openai != nil {
		resolver := h.gatewayService.NewModelGroupRouteResolver(openai.gatewayService)
		h.modelGroupRoutes = resolver
		openai.modelGroupRoutes = resolver
		openai.modelGroupRouteCatalog = h.gatewayService
	}
	return func(c *gin.Context) {
		key, ok := middleware2.GetAPIKeyFromContext(c)
		if h == nil || h.modelGroupRoutes == nil || !ok || key == nil || !hasEnabledModelGroupRoutes(key.Group) || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		path := c.FullPath()
		if !strings.Contains(path, "/responses") && !strings.HasSuffix(path, "/chat/completions") && !strings.HasSuffix(path, "/messages") && !strings.HasSuffix(path, "/messages/count_tokens") {
			c.Next()
			return
		}
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := http.StatusBadRequest
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
			}
			h.errorResponse(c, status, "invalid_request_error", "Failed to read request body")
			c.Abort()
			return
		}
		requestmodel.ResetRequestBody(c.Request, body)
		candidates := requestmodel.FromBodyCandidates(path, c.GetHeader("Content-Type"), body)
		model := requestmodel.FromBodyForRoute(path, c.GetHeader("Content-Type"), body)
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate) != strings.TrimSpace(model) {
				h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Conflicting model values are not supported for group routing")
				c.Abort()
				return
			}
		}
		if model == "" {
			c.Next()
			return
		}
		req := modelGroupRouteRequest(path, model, body, false)
		selected, err := h.modelGroupRoutes.Resolve(c.Request.Context(), key, req)
		if err != nil {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
			h.errorResponse(c, infraerrors.Code(err), "invalid_request_error", infraerrors.Message(err))
			c.Abort()
			return
		}
		installModelGroupRouteKey(c, selected)
		c.Next()
	}
}

func modelGroupRouteRequest(path, model string, body []byte, ws bool) service.ModelGroupRouteRequest {
	req := service.ModelGroupRouteRequest{Model: model, RequiredTransport: service.OpenAIUpstreamTransportAny}
	if ws {
		req.RequiredTransport = service.OpenAIUpstreamTransportResponsesWebsocketV2Ingress
	}
	req.RequireCompact = strings.Contains(path, "/compact")
	if req.RequireCompact {
		req.RequiredCapability = service.OpenAIEndpointCapabilityResponses
	}
	if strings.Contains(path, "/responses") && isOpenAIRemoteCompactionV2Request(body) {
		req.RequiredCapability = service.OpenAIEndpointCapabilityResponses
	}
	return req
}

func installModelGroupRouteKey(c *gin.Context, key *service.APIKey) {
	c.Set(string(middleware2.ContextKeyAPIKey), key)
	c.Request = c.Request.WithContext(service.ModelGroupRouteContext(c.Request.Context(), key))
}

func consistentModelGroupRouteCandidates(model string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != strings.TrimSpace(model) {
			return false
		}
	}
	return true
}

func (h *OpenAIGatewayHandler) resolveModelGroupRouteWSTurn(ctx context.Context, source, conn *service.APIKey, req service.ModelGroupRouteRequest, candidates []string) (*service.APIKey, error) {
	if h.modelGroupRoutes == nil || source == nil || source.Group == nil || conn == nil || source.Group.Platform != service.PlatformOpenAI || source.Group.IsSubscriptionType() {
		return conn, nil
	}
	routing := hasEnabledModelGroupRoutes(source.Group) || conn.SourceGroupID != nil
	sourceKey := source
	if h.apiKeyService != nil && source.Key != "" {
		latest, err := h.apiKeyService.GetByKey(ctx, source.Key)
		if err != nil || latest == nil || latest.ID != source.ID || latest.GroupID == nil || source.GroupID == nil || *latest.GroupID != *source.GroupID || latest.Group == nil || latest.Group.Platform != source.Group.Platform || latest.Group.SubscriptionType != source.Group.SubscriptionType {
			if routing {
				return nil, newOpenAIWSUnsupportedModelSwitchError(req.Model)
			}
			// 没有路由的既有连接保留原调价刷新失败的回退契约。
			return conn, nil
		}
		sourceKey = latest
	}
	// 即使建连时没有规则，也要识别后来启用的规则。
	if !routing && !hasEnabledModelGroupRoutes(sourceKey.Group) {
		return conn, nil
	}
	if !consistentModelGroupRouteCandidates(req.Model, candidates) {
		return nil, newOpenAIWSUnsupportedModelSwitchError(req.Model)
	}
	if blocked := blockedModelAllowlistCandidate(sourceKey.Group, candidates); blocked != "" {
		return nil, newOpenAIWSUnsupportedModelSwitchError(req.Model)
	}
	selected, err := h.modelGroupRoutes.Resolve(ctx, sourceKey, req)
	if err != nil || selected == nil || selected.GroupID == nil || conn.GroupID == nil || *selected.GroupID != *conn.GroupID {
		return nil, newOpenAIWSUnsupportedModelSwitchError(req.Model)
	}
	// 只刷新实际组；Key 额度、用户限制仍沿用建连快照。
	turnKey := *conn
	turnKey.Group = selected.Group
	return &turnKey, nil
}

// routedCodexManifest 保留原目录元数据，仅替换规则选中模型的能力描述。
func routedCodexManifest(ctx context.Context, resolver *service.ModelGroupRouteResolver, catalog *service.GatewayService, key *service.APIKey, base *service.OpenAIModelsResponse, ifNoneMatch string) (*service.OpenAIModelsResponse, error) {
	var ids []string
	entries := make(map[string]json.RawMessage)
	for _, entry := range gjson.GetBytes(base.Body, "models").Array() {
		id := entry.Get("slug").String()
		if id != "" {
			ids = append(ids, id)
			entries[id] = json.RawMessage(entry.Raw)
		}
	}
	models, effective := resolver.VisibleModels(ctx, key, ids)
	for model, selected := range effective {
		body, err := catalog.BuildCodexModelsManifestForGroup(ctx, selected.Group, "", []string{model})
		if err != nil {
			return nil, err
		}
		for _, entry := range gjson.GetBytes(body, "models").Array() {
			if entry.Get("slug").String() == model {
				entries[model] = json.RawMessage(entry.Raw)
			}
		}
	}
	output := make([]json.RawMessage, 0, len(models))
	for _, model := range models {
		if entry, ok := entries[model]; ok {
			output = append(output, entry)
		}
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	body, err := sjson.SetRawBytes(base.Body, "models", raw)
	if err != nil {
		return nil, err
	}
	etag := service.CodexModelsManifestETag(body)
	return &service.OpenAIModelsResponse{Body: body, ETag: etag, NotModified: service.CodexModelsManifestETagMatches(ifNoneMatch, etag)}, nil
}

func (h *GatewayHandler) routedOpenAIModelsResponse(c *gin.Context, key *service.APIKey, base *service.OpenAIModelsResponse) (*service.OpenAIModelsResponse, error) {
	var ids []string
	entries := make(map[string]json.RawMessage)
	for _, entry := range gjson.GetBytes(base.Body, "data").Array() {
		id := entry.Get("id").String()
		if id != "" {
			ids = append(ids, id)
			entries[id] = json.RawMessage(entry.Raw)
		}
	}
	models, _ := h.modelGroupRoutes.VisibleModels(c.Request.Context(), key, ids)
	output := make([]json.RawMessage, 0, len(models))
	for _, model := range models {
		entry, ok := entries[model]
		if !ok {
			var err error
			entry, err = json.Marshal(gin.H{"id": model, "object": "model", "owned_by": "openai"})
			if err != nil {
				return nil, err
			}
		}
		output = append(output, entry)
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	body, err := sjson.SetRawBytes(base.Body, "data", raw)
	if err != nil {
		return nil, err
	}
	etag := service.CodexModelsManifestETag(body)
	notModified := c.Param("model") == "" && service.CodexModelsManifestETagMatches(c.GetHeader("If-None-Match"), etag)
	return &service.OpenAIModelsResponse{Body: body, ETag: etag, NotModified: notModified}, nil
}
