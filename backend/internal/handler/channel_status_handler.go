package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelStatusHandler 脚本用只读渠道状态。
// GET /v1/sub2api/channel-status
// 鉴权：用户 API Key（Bearer / x-api-key）。额度耗尽或 Key 所属分组停用仍可查询。
// 默认只返回当前 Key 绑定分组；?scope=visible 额外返回可见分组列表。
type ChannelStatusHandler struct {
	svc *service.ChannelStatusService
}

func NewChannelStatusHandler(svc *service.ChannelStatusService) *ChannelStatusHandler {
	return &ChannelStatusHandler{svc: svc}
}

func (h *ChannelStatusHandler) Get(c *gin.Context) {
	if h == nil || h.svc == nil {
		abortChannelStatusError(c, http.StatusServiceUnavailable, "api_error", "channel status is unavailable")
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.User == nil {
		abortChannelStatusError(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	includeVisible, ok := parseChannelStatusScope(c.Query("scope"))
	if !ok {
		abortChannelStatusError(c, http.StatusBadRequest, "invalid_request_error", "Invalid scope")
		return
	}
	var keyGroupID *int64
	var keyGroupName string
	if apiKey.GroupID != nil {
		id := *apiKey.GroupID
		keyGroupID = &id
		if apiKey.Group != nil {
			keyGroupName = apiKey.Group.Name
		}
	}
	snap, err := h.svc.Get(c.Request.Context(), service.ChannelStatusQuery{
		UserID:         apiKey.User.ID,
		KeyGroupID:     keyGroupID,
		KeyGroupName:   keyGroupName,
		IncludeVisible: includeVisible,
	})
	if err != nil {
		slog.Warn("failed to load channel status", "error", err, "user_id", apiKey.User.ID)
		abortChannelStatusError(c, http.StatusInternalServerError, "api_error", "Failed to load channel status")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, snap)
}

func parseChannelStatusScope(raw string) (includeVisible bool, ok bool) {
	switch strings.TrimSpace(raw) {
	case "", service.ChannelStatusScopeKey:
		return false, true
	case service.ChannelStatusScopeVisible:
		return true, true
	default:
		return false, false
	}
}

func abortChannelStatusError(c *gin.Context, status int, errType, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
