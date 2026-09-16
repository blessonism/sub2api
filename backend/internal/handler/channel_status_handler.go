package handler

import (
	"log/slog"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelStatusHandler 脚本用只读渠道状态。
// GET /v1/sub2api/channel-status
// 鉴权：用户 API Key（Bearer / x-api-key）。额度耗尽或 Key 所属分组停用仍可查询。
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
	var keyGroupID *int64
	var keyGroupName string
	if apiKey.GroupID != nil {
		id := *apiKey.GroupID
		keyGroupID = &id
		if apiKey.Group != nil {
			keyGroupName = apiKey.Group.Name
		}
	}
	snap, err := h.svc.Get(c.Request.Context(), apiKey.User.ID, keyGroupID, keyGroupName)
	if err != nil {
		slog.Warn("failed to load channel status", "error", err, "user_id", apiKey.User.ID)
		abortChannelStatusError(c, http.StatusInternalServerError, "api_error", "Failed to load channel status")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, snap)
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
