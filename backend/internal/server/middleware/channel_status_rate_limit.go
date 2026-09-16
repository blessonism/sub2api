package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	channelStatusRPM             = 12
	channelStatusRateLimitWindow = time.Minute
)

// ChannelStatusRateLimit 渠道状态查询按用户 12 次/分钟。Redis 异常 fail-open。
func ChannelStatusRateLimit(redisClient *redis.Client) gin.HandlerFunc {
	var limiter panelRateLimitAllower
	if redisClient != nil {
		limiter = middleware.NewRateLimiter(redisClient)
	}
	return channelStatusRateLimit(limiter)
}

func channelStatusRateLimit(limiter panelRateLimitAllower) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			c.Next()
			return
		}
		result, err := limiter.Allow(
			c.Request.Context(),
			"channel-status:user:"+strconv.FormatInt(subject.UserID, 10),
			channelStatusRPM,
			channelStatusRateLimitWindow,
		)
		if err != nil {
			slog.Warn("channel status rate limit check failed, allowing request", "error", err)
			c.Next()
			return
		}
		if !result.Allowed {
			abortChannelStatusRateLimited(c, result.RetryAfter)
			return
		}
		c.Next()
	}
}

func abortChannelStatusRateLimited(c *gin.Context, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = channelStatusRateLimitWindow
	}
	seconds := int64(retryAfter / time.Second)
	if retryAfter%time.Second > 0 {
		seconds++
	}
	c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "rate_limit_error",
			"message": "Too many requests, please slow down and try again later",
		},
	})
}
