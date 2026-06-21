package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 捕获 ListUsers 入参、返回一个已删用户的 admin service 桩。
type searchUsersAdminStub struct {
	service.AdminService
	gotFilters service.UserListFilters
	idUser     *service.User
	gotUserID  int64
}

func (s *searchUsersAdminStub) ListUsers(ctx context.Context, page, pageSize int, filters service.UserListFilters, sortBy, sortOrder string) ([]service.User, int64, error) {
	s.gotFilters = filters
	ts := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	return []service.User{
		{ID: 1, Email: "active@test.com"},
		{ID: 2, Email: "deleted@test.com", DeletedAt: &ts},
	}, 2, nil
}

func (s *searchUsersAdminStub) GetUserIncludeDeleted(ctx context.Context, id int64) (*service.User, error) {
	s.gotUserID = id
	if s.idUser != nil {
		return s.idUser, nil
	}
	return nil, service.ErrUserNotFound
}

func TestAdminUsageSearchUsers_IncludesDeletedAndFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &searchUsersAdminStub{}
	handler := NewUsageHandler(nil, nil, stub, nil)
	router := gin.New()
	router.GET("/admin/usage/search-users", handler.SearchUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, stub.gotFilters.IncludeDeleted, "SearchUsers 必须请求 IncludeDeleted")

	var resp struct {
		Data []struct {
			ID      int64  `json:"id"`
			Email   string `json:"email"`
			Deleted bool   `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	require.False(t, resp.Data[0].Deleted)
	require.True(t, resp.Data[1].Deleted, "已删用户必须标记 deleted=true")
}

func TestAdminUsageSearchUsers_NumericKeywordIncludesExactUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ts := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	stub := &searchUsersAdminStub{
		idUser: &service.User{ID: 42, Email: "id-hit@test.com", DeletedAt: &ts},
	}
	handler := NewUsageHandler(nil, nil, stub, nil)
	router := gin.New()
	router.GET("/admin/usage/search-users", handler.SearchUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), stub.gotUserID)

	var resp struct {
		Data []struct {
			ID      int64  `json:"id"`
			Email   string `json:"email"`
			Deleted bool   `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Data)
	require.Equal(t, int64(42), resp.Data[0].ID)
	require.Equal(t, "id-hit@test.com", resp.Data[0].Email)
	require.True(t, resp.Data[0].Deleted)
}
