package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelStatusHandlerGetUsesAPIKeyGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(12)
	h := NewChannelStatusHandler(service.NewChannelStatusService(nil, nil, nil, nil))
	router := gin.New()
	router.GET("/v1/sub2api/channel-status", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
			User:    &service.User{ID: 7, Status: service.StatusActive},
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Name: "plus"},
		})
		h.Get(c)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var snap service.ChannelStatusSnapshot
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snap))
	require.Equal(t, "sub2api.channel_status", snap.Object)
	require.Equal(t, 2, snap.SchemaVersion)
	require.Equal(t, groupID, *snap.GroupID)
	require.Equal(t, "plus", snap.GroupName)
	require.Nil(t, snap.Connected)
	require.Equal(t, service.ChannelStatusStatusUnknown, snap.Status)
	require.Empty(t, snap.Items)
	require.NotContains(t, w.Body.String(), `"items"`)
	require.NotContains(t, w.Body.String(), "key_group")
}

func TestChannelStatusHandlerUnauthorizedWithoutAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewChannelStatusHandler(service.NewChannelStatusService(nil, nil, nil, nil))
	router := gin.New()
	router.GET("/v1/sub2api/channel-status", h.Get)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChannelStatusHandlerRejectsInvalidScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(12)
	h := NewChannelStatusHandler(service.NewChannelStatusService(nil, nil, nil, nil))
	router := gin.New()
	router.GET("/v1/sub2api/channel-status", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
			User:    &service.User{ID: 7, Status: service.StatusActive},
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Name: "plus"},
		})
		h.Get(c)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status?scope=all", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_request_error")
}
