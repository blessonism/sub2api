package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewayRoutesChannelStatusPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/v1/sub2api/channel-status" {
			return
		}
	}
	t.Fatal("GET /v1/sub2api/channel-status should be registered")
}

func TestGatewayRoutesChannelStatusRequiresAPIKey(t *testing.T) {
	router, _, _ := newKeyBillingRouteTestRouter(config.RunModeStandard)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
