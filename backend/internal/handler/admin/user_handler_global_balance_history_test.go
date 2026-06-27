package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerGetGlobalBalanceHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	usedBy := int64(12)
	usedAt := time.Date(2026, 6, 27, 9, 30, 0, 0, time.UTC)
	adminSvc := newStubAdminService()
	adminSvc.redeems = []service.RedeemCode{
		{
			ID:        99,
			Code:      "TEST-CODE-123",
			Type:      service.RedeemTypeBalance,
			Value:     20,
			Status:    service.StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &usedAt,
			CreatedAt: usedAt,
			User:      &service.User{ID: usedBy, Email: "user@example.com"},
		},
	}

	handler := &UserHandler{adminService: adminSvc}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/admin/redeem-records?page=1&page_size=20", nil)

	handler.GetGlobalBalanceHistory(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Items []struct {
				ID   int64 `json:"id"`
				User *struct {
					ID    int64  `json:"id"`
					Email string `json:"email"`
				} `json:"user"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, int64(1), body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	require.Equal(t, int64(99), body.Data.Items[0].ID)
	require.NotNil(t, body.Data.Items[0].User)
	require.Equal(t, "user@example.com", body.Data.Items[0].User.Email)
}
