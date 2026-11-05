package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/observability/capture"
	"github.com/Wei-Shaw/sub2api/internal/observability/langfuse"
)

// 观测状态枚举。只用固定短串，避免把内部错误文本外发到观测层。
const (
	observeStatusOK           = "ok"
	observeStatusUnbilled     = "unbilled"
	observeStatusBillingError = "billing_error"

	// observeErrorBillingFailed 是外发的固定错误类别，不带原始错误内容。
	observeErrorBillingFailed = "apply_usage_billing_failed"
)

// observeModelCall 把一行已落库的用量记录投影成观测层的 1 条 trace + 1 条 generation。
//
// 三条口径：
//   - 只上报元数据：检索键（userId/sessionId/requestId）、用量桶、状态；
//   - cost 直接透传业务库的实扣值（CostBreakdown.ActualCost 落库后的值），
//     观测层不重算价格——倍率、用户级覆盖、峰谷因子都在计费链路内部收敛，
//     在这里按 token×标价重算必然与 usage_logs 对不上；
//   - 不上报提示词正文与响应正文。正文取证归 securityaudit 审查台，
//     两侧不重复落正文；这也保证容器内不产生新的明文副本。
//
// 关闭态下 langfuse.Active() 为 false，函数在读原子标志后立即返回：
// 不构造 Call、不分配、不出网。
func observeModelCall(ctx context.Context, usageLog *UsageLog, charged float64, status, errorKind string) {
	if usageLog == nil || !langfuse.Active() {
		return
	}

	// 正文层：只有请求上装了 recorder 才带 input/output，未安装时两者为空串。
	var input, output, inputUser string
	var inputTruncated, outputTruncated bool
	var userTurns int
	var promptProtocol, promptStatus string
	if rec := capture.From(ctx); rec != nil {
		input = rec.RequestText()
		output = rec.ResponseText()
		inputUser, userTurns, promptProtocol, promptStatus = rec.UserPrompt()
		inputTruncated = rec.RequestTruncated()
		outputTruncated = rec.ResponseTruncated()
		// 图片/视频类响应的正文是 base64 载荷，不是可读文本，落进观测层只是噪声。
		if usageLog.ImageCount > 0 || usageLog.VideoCount > 0 {
			output = ""
		}
	}

	var duration time.Duration
	if usageLog.DurationMs != nil {
		duration = time.Duration(*usageLog.DurationMs) * time.Millisecond
	}

	langfuse.Observe(langfuse.Call{
		UserID:    usageLog.UserID,
		SessionID: derefString(usageLog.SessionID),
		RequestID: usageLog.RequestID,

		Model:    usageLog.Model,
		Endpoint: derefString(usageLog.InboundEndpoint),

		Status:    status,
		ErrorKind: errorKind,

		StartedAt: usageLog.CreatedAt.Add(-duration),
		Duration:  duration,

		InputTokens:       usageLog.InputTokens,
		OutputTokens:      usageLog.OutputTokens,
		CacheReadTokens:   usageLog.CacheReadTokens,
		CacheWriteTokens:  usageLog.CacheCreationTokens,
		ImageInputTokens:  usageLog.ImageInputTokens,
		ImageOutputTokens: usageLog.ImageOutputTokens,

		Cost:     charged,
		Currency: "USD",

		RateMultiplier:        usageLog.RateMultiplier,
		VisibleRateMultiplier: derefFloat64(usageLog.VisibleRateMultiplier),
		AccountRateMultiplier: derefFloat64(usageLog.AccountRateMultiplier),
		BillingType:           billingTypeLabel(usageLog.BillingType),
		BillingMode:           derefString(usageLog.BillingMode),
		ServiceTier:           derefString(usageLog.ServiceTier),
		Stream:                usageLog.Stream,

		AccountID: usageLog.AccountID,
		APIKeyID:  usageLog.APIKeyID,
		GroupID:   derefInt64(usageLog.GroupID),

		Input:           input,
		InputUser:       inputUser,
		UserTurns:       userTurns,
		PromptProtocol:  promptProtocol,
		PromptStatus:    promptStatus,
		Output:          output,
		InputTruncated:  inputTruncated,
		OutputTruncated: outputTruncated,
	})
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func billingTypeLabel(v int8) string {
	switch v {
	case BillingTypeBalance:
		return "balance"
	case BillingTypeSubscription:
		return "subscription"
	default:
		return ""
	}
}
