package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserChannelMonitorGptIntelligenceRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(v1, &handler.Handlers{
		ChannelMonitor: &handler.ChannelMonitorUserHandler{},
	}, func(c *gin.Context) { c.Next() }, nil, nil)

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	require.True(t, registered["GET /api/v1/channel-monitors/gpt-intelligence"])
	require.True(t, registered["GET /api/v1/channel-monitors/:id/status"])
}
