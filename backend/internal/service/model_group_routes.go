package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ModelGroupRouteRequest 是选组所需的调度资格，不进行抢槽或上游探测。
type ModelGroupRouteRequest struct {
	Model              string
	RequiredTransport  OpenAIUpstreamTransport
	RequiredCapability OpenAIEndpointCapability
	RequireCompact     bool
}

type ModelGroupRouteResolver struct {
	groups GroupRepository
	openai *OpenAIGatewayService
}

func NewModelGroupRouteResolver(groups GroupRepository, openai *OpenAIGatewayService) *ModelGroupRouteResolver {
	return &ModelGroupRouteResolver{groups: groups, openai: openai}
}

func (s *GatewayService) NewModelGroupRouteResolver(openai *OpenAIGatewayService) *ModelGroupRouteResolver {
	return NewModelGroupRouteResolver(s.groupRepo, openai)
}

func modelGroupRouteError(status int, message string) error {
	return infraerrors.New(status, "MODEL_GROUP_ROUTE_UNAVAILABLE", message)
}

func (s *adminServiceImpl) normalizeModelGroupRoutes(ctx context.Context, sourceID int64, platform, subscriptionType string, routes []ModelGroupRoute) ([]ModelGroupRoute, error) {
	out := make([]ModelGroupRoute, 0, len(routes))
	seen := make(map[string]bool, len(routes))
	for _, route := range routes {
		route.Model = strings.TrimSpace(route.Model)
		if route.Mode == "" {
			route.Mode = domain.ModelGroupRouteTargetOnly
		}
		if platform != PlatformOpenAI || subscriptionType == SubscriptionTypeSubscription || route.Model == "" || strings.Contains(route.Model, "*") || route.TargetGroupID <= 0 || route.TargetGroupID == sourceID || seen[route.Model] || (route.Mode != domain.ModelGroupRouteTargetOnly && route.Mode != domain.ModelGroupRoutePreferLocal) {
			return nil, infraerrors.New(http.StatusBadRequest, "INVALID_MODEL_GROUP_ROUTES", "Model routes require unique exact models, another OpenAI balance group and a supported mode")
		}
		if !route.Enabled {
			seen[route.Model] = true
			out = append(out, route)
			continue
		}
		target, err := s.groupRepo.GetByIDLite(ctx, route.TargetGroupID)
		if err != nil || target == nil || target.Platform != PlatformOpenAI || target.IsSubscriptionType() || !target.IsActive() {
			return nil, infraerrors.New(http.StatusBadRequest, "INVALID_MODEL_GROUP_ROUTES", "Target group must be an active OpenAI balance group")
		}
		seen[route.Model] = true
		out = append(out, route)
	}
	return out, nil
}

// Resolve 只解析原绑定分组的规则；不递归应用目标组规则。
func (r *ModelGroupRouteResolver) Resolve(ctx context.Context, key *APIKey, req ModelGroupRouteRequest) (*APIKey, error) {
	if r == nil || key == nil || key.Group == nil || key.SourceGroupID != nil {
		return key, nil
	}
	source := key.Group
	if source.Platform != PlatformOpenAI || source.IsSubscriptionType() {
		return key, nil
	}
	var matched *ModelGroupRoute
	for i := range source.ModelGroupRoutes {
		route := &source.ModelGroupRoutes[i]
		if route.Enabled && route.Model == strings.TrimSpace(req.Model) {
			matched = route
			break
		}
	}
	if matched == nil {
		return key, nil
	}
	if key.User == nil || !source.IsActive() || !key.User.CanBindGroup(source.ID, source.IsExclusive) || !source.ModelAllowlist.Allows(req.Model) {
		return nil, modelGroupRouteError(http.StatusForbidden, "Source group does not allow this model")
	}
	if matched.Mode == domain.ModelGroupRoutePreferLocal {
		if r.openai == nil {
			return nil, modelGroupRouteError(http.StatusServiceUnavailable, "Model route scheduling is unavailable")
		}
		local, err := r.openai.hasModelGroupRouteCandidate(ctx, key, req)
		if err != nil {
			return nil, err
		}
		if local {
			return key, nil
		}
	}
	if r.groups == nil || matched.TargetGroupID == source.ID {
		return nil, modelGroupRouteError(http.StatusServiceUnavailable, "Model route target is unavailable")
	}
	target, err := r.groups.GetByIDLite(ctx, matched.TargetGroupID)
	if err != nil || target == nil || !target.IsActive() || target.Platform != PlatformOpenAI || target.IsSubscriptionType() {
		return nil, modelGroupRouteError(http.StatusServiceUnavailable, "Model route target is unavailable")
	}
	if !key.User.CanBindGroup(target.ID, target.IsExclusive) {
		return nil, modelGroupRouteError(http.StatusForbidden, "Target group is not allowed for this user")
	}
	if !target.ModelAllowlist.Allows(req.Model) {
		return nil, modelGroupRouteError(http.StatusNotFound, "Model is not available for the target group")
	}
	cloned := *key
	user := *key.User
	// 该值来自原 (user,group) 的认证快照；由 billing service 查询目标组覆盖。
	user.UserGroupRPMOverride = nil
	cloned.User = &user
	id, sourceID := target.ID, source.ID
	cloned.GroupID, cloned.Group = &id, target
	cloned.SourceGroupID = &sourceID
	return &cloned, nil
}

func ModelGroupRouteContext(ctx context.Context, key *APIKey) context.Context {
	if key == nil || key.Group == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.Group, key.Group)
}

func (s *OpenAIGatewayService) hasModelGroupRouteCandidate(ctx context.Context, key *APIKey, req ModelGroupRouteRequest) (bool, error) {
	if s == nil || s.accountRepo == nil {
		return false, nil
	}
	group := key.Group
	ctx = ModelGroupRouteContext(ctx, key)
	ctx = context.WithValue(ctx, ctxkey.UserID, key.User.ID)
	ctx = s.withOpenAIQuotaAutoPauseContext(ctx)
	ctx, _ = s.WithOpenAIRequestPricingContext(ctx, key.GroupID)
	mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, key.GroupID, req.Model)
	model := req.Model
	if mapping.MappedModel != "" {
		model = mapping.MappedModel
	}
	if s.checkChannelPricingRestriction(ctx, key.GroupID, model) {
		return false, nil
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, PlatformOpenAI)
	if err != nil {
		return false, err
	}
	accounts = s.filterOpenAIAccountsBySchedulingThreshold(ctx, accounts)
	binding, _, err := resolveRequestUserGroupAccountBinding(ctx, s.userGroupAccountBindingResolver, key.GroupID)
	if err != nil {
		return false, err
	}
	scheduler := &defaultOpenAIAccountScheduler{service: s}
	schedule := OpenAIAccountScheduleRequest{GroupID: key.GroupID, Platform: PlatformOpenAI, RequestedModel: model, RequiredTransport: req.RequiredTransport, RequiredCapability: req.RequiredCapability, RequireCompact: req.RequireCompact, RequirePrivacySet: group.RequirePrivacySet}
	for i := range accounts {
		account := &accounts[i]
		if binding != nil && !binding.FallbackToGroup && !containsInt64(binding.AccountIDs, account.ID) {
			continue
		}
		if account.IsSchedulableForModelWithContext(ctx, model) && scheduler.isAccountRequestCompatible(ctx, account, schedule) && scheduler.isAccountTransportCompatible(account, req.RequiredTransport) {
			return true, nil
		}
	}
	return false, nil
}

// VisibleModels 在每次请求中合并，避免把某个用户的目标组权限缓存给其他用户。
func (r *ModelGroupRouteResolver) VisibleModels(ctx context.Context, key *APIKey, base []string) ([]string, map[string]*APIKey) {
	models := append([]string(nil), base...)
	effective := make(map[string]*APIKey)
	if r == nil || key == nil || key.Group == nil {
		return models, effective
	}
	for _, route := range key.Group.ModelGroupRoutes {
		if !route.Enabled {
			continue
		}
		filtered := models[:0]
		for _, model := range models {
			if model != route.Model {
				filtered = append(filtered, model)
			}
		}
		models = filtered
		selected, err := r.Resolve(ctx, key, ModelGroupRouteRequest{Model: route.Model, RequiredTransport: OpenAIUpstreamTransportAny})
		if err != nil || !key.Group.ModelAllowlist.Allows(route.Model) {
			continue
		}
		// 模型必须确实有符合资格的账号；不把配置中的拼写错误展示为可用模型。
		if r.openai == nil {
			continue
		}
		available, err := r.openai.hasModelGroupRouteCandidate(ctx, selected, ModelGroupRouteRequest{Model: route.Model, RequiredTransport: OpenAIUpstreamTransportAny})
		if err != nil || !available {
			continue
		}
		models = append(models, route.Model)
		effective[route.Model] = selected
	}
	return key.Group.ModelAllowlist.FilterForListing(dedupeAndSortModelIDs(models)), effective
}
