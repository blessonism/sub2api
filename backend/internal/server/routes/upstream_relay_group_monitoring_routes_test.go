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
		"POST /api/v1/admin/upstream-relay-group-monitors/connectors/sync-all",
		"POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/sync",
		"POST /api/v1/admin/upstream-relay-group-monitors/connectors/:id/metrics/refresh",
		"GET /api/v1/admin/upstream-relay-group-monitors/connectors/:id/api-keys",
		"GET /api/v1/admin/upstream-relay-group-monitors/connectors/:id/snapshots",
		"GET /api/v1/admin/upstream-relay-group-monitors/snapshot-changes",
		"GET /api/v1/admin/upstream-relay-group-monitors/usage-history",
		"GET /api/v1/admin/upstream-relay-group-monitors/candidates",
		"POST /api/v1/admin/upstream-relay-group-monitors/candidates",
		"PUT /api/v1/admin/upstream-relay-group-monitors/candidates/:id",
		"DELETE /api/v1/admin/upstream-relay-group-monitors/candidates/:id",
		"POST /api/v1/admin/upstream-relay-group-monitors/candidates/probe-all",
		"POST /api/v1/admin/upstream-relay-group-monitors/candidates/:id/probe",
		"GET /api/v1/admin/upstream-relay-group-monitors/monitoring-policy",
		"PUT /api/v1/admin/upstream-relay-group-monitors/monitoring-policy",
		"GET /api/v1/admin/upstream-relay-group-monitors/recommendation-policy",
		"PUT /api/v1/admin/upstream-relay-group-monitors/recommendation-policy",
		"POST /api/v1/admin/upstream-relay-group-monitors/recommendations/preview",
		"POST /api/v1/admin/upstream-relay-group-monitors/recommendations",
		"POST /api/v1/admin/upstream-relay-group-monitors/recommendations/:id/apply",
		"DELETE /api/v1/admin/upstream-relay-group-monitors/recommendations/:id",
	} {
		require.True(t, registered[route], "%s should be registered", route)
	}
}
