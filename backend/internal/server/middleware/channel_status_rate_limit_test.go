package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countingChannelStatusLimiter struct {
	limit int
	count int64
	err   error
	keys  []string
}

func (l *countingChannelStatusLimiter) Allow(_ context.Context, key string, _ int, _ time.Duration) (middleware.AllowResult, error) {
	l.keys = append(l.keys, key)
	if l.err != nil {
		return middleware.AllowResult{}, l.err
	}
	l.count++
	return middleware.AllowResult{
		Allowed:    l.count <= int64(l.limit),
		Count:      l.count,
		RetryAfter: time.Second,
	}, nil
}

func TestChannelStatusRateLimitBlocksThirteenthRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &countingChannelStatusLimiter{limit: channelStatusRPM}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
		c.Set(string(ContextKeyUserRole), "admin")
		c.Next()
	})
	router.GET("/v1/sub2api/channel-status", channelStatusRateLimit(limiter), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < channelStatusRPM; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
		require.Equal(t, http.StatusOK, w.Code, "request %d", i+1)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "1", w.Header().Get("Retry-After"))
	require.Equal(t, "channel-status:user:7", limiter.keys[0])
	require.Len(t, limiter.keys, channelStatusRPM+1)
	var body struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "error", body.Type)
	require.Equal(t, "rate_limit_error", body.Error.Type)
}

func TestChannelStatusRateLimitFailOpenOnRedisError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &countingChannelStatusLimiter{err: context.DeadlineExceeded}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
		c.Next()
	})
	router.GET("/v1/sub2api/channel-status", channelStatusRateLimit(limiter), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestChannelStatusRateLimitNilLimiterAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/sub2api/channel-status", channelStatusRateLimit(nil), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/sub2api/channel-status", nil))
	require.Equal(t, http.StatusOK, w.Code)
}
