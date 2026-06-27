package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamRelayGroupMonitoringRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")

	registerUpstreamRelayGroupMonitoringRoutes(admin, &handler.Handlers{
		Admin: &handler.AdminHandlers{
			UpstreamRelayMonitoring: &adminhandler.UpstreamRelayGroupMonitoringHandler{},
		},
	})

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		"GET /api/v1/admin/upstream-relay-group-monitors/connectors",
		"POST /api/v1/admin/upstream-relay-group-monitors/connectors",
		"POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/sync",
		"GET /api/v1/admin/upstream-relay-group-monitors/connectors/:id/snapshots",
		"POST /api/v1/admin/upstream-relay-group-monitors/candidates/:id/probe",
		"POST /api/v1/admin/upstream-relay-group-monitors/recommendations",
		"POST /api/v1/admin/upstream-relay-group-monitors/recommendations/:id/apply",
	} {
		require.True(t, registered[route], "%s should be registered", route)
	}
}
