//go:build unit

package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBatchUpdateConcurrencyFloorTargetsAllUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{ID: 1, Role: service.RoleUser, Status: service.StatusActive},
		{ID: 2, Role: service.RoleUser, Status: service.StatusDisabled},
		{ID: 3, Role: service.RoleAdmin, Status: service.StatusActive},
	}
	handler := NewUserHandler(adminSvc, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/batch-concurrency", strings.NewReader(`{"all":true,"concurrency":5,"mode":"floor"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.BatchUpdateConcurrency(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{1, 2, 3}, adminSvc.lastBatchConcurrency.userIDs)
	require.Equal(t, 5, adminSvc.lastBatchConcurrency.value)
	require.Equal(t, "floor", adminSvc.lastBatchConcurrency.mode)
}

func TestBatchUpdateConcurrencyFloorRejectsInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	handler := NewUserHandler(adminSvc, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/batch-concurrency", strings.NewReader(`{"all":true,"concurrency":0,"mode":"floor"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.BatchUpdateConcurrency(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, adminSvc.lastBatchConcurrency.calls)
}
