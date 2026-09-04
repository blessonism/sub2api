//go:build unit

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// 错误归类（ErrorCategory）：限流/容量类错误必须与真故障区分。
// 复用 v2 taxonomy（ClassifyChannelMonitorV2Error），另补充网关流内
// "pending requests" 形态（HTTP 200 + SSE response.failed，无顶层 429）。

func TestClassifyMonitorErrorCategory(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		message    string
		want       string
	}{
		{
			name:       "gateway stream rate limit without status code",
			statusCode: http.StatusOK,
			message:    "responses stream response.failed: Too many pending requests, please retry later (request id: x)",
			want:       MonitorErrorCategoryRateOrCapacity,
		},
		{
			name:       "pending requests without too many is not rate limited",
			statusCode: http.StatusOK,
			message:    "error processing pending requests for this session",
			want:       "other",
		},
		{
			name:       "too many requests without 429 status stays unclassified by supplement",
			statusCode: 0,
			message:    "could not enqueue too many requests in the local buffer",
			want:       "other",
		},
		{
			name:       "429 status code via v2 taxonomy",
			statusCode: http.StatusTooManyRequests,
			message:    `upstream HTTP 429: {"error":{"message":"slow down"}}`,
			want:       MonitorErrorCategoryRateOrCapacity,
		},
		{
			name:       "5xx falls into upstream_5xx via v2 taxonomy",
			statusCode: http.StatusBadGateway,
			message:    "upstream HTTP 502: bad gateway",
			want:       "upstream_5xx",
		},
		{
			name:       "connection refused classified as transport via v2 taxonomy",
			statusCode: 0,
			message:    "do request: dial tcp: connection refused",
			want:       "transport_or_stream",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyMonitorErrorCategory(tc.statusCode, tc.message)
			if got != tc.want {
				t.Fatalf("classifyMonitorErrorCategory(%d, %q) = %q, want %q", tc.statusCode, tc.message, got, tc.want)
			}
		})
	}
}

func TestRunCheckForModel_Upstream429SetsErrorCategory(t *testing.T) {
	h := &openAICaptureHandler{status: http.StatusTooManyRequests}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-key", MonitorDefaultGrokModel, nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("429 should be error, got %s", res.Status)
	}
	if res.ErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("429 should be classified rate_or_capacity, got %q", res.ErrorCategory)
	}
}

func TestRunCheckForModel_ResponsesStreamGatewayRateLimitSetsErrorCategory(t *testing.T) {
	// 网关等待队列满的流内形态：HTTP 200 + SSE response.failed（无顶层 429）。
	h := &openAICaptureHandler{
		responsesStream: func(w http.ResponseWriter, answer string) {
			writeResponsesSSEEvent(w, "response.failed", map[string]any{
				"type": "response.failed",
				"response": map[string]any{
					"status": "failed",
					"error":  map[string]any{"code": "rate_limit_error", "message": "Too many pending requests, please retry later"},
				},
			})
		},
	}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusError {
		t.Fatalf("stream rate limit must remain error, got %s", res.Status)
	}
	if !strings.Contains(res.Message, "Too many pending requests") {
		t.Fatalf("stream rate limit message should be preserved, got %q", res.Message)
	}
	if res.ErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("stream rate limit should be classified rate_or_capacity, got %q", res.ErrorCategory)
	}
}

func TestRunCheckForModel_SuccessAndChallengeMismatchHaveNoCategory(t *testing.T) {
	endpoint := setupFakeOpenAI(t, &openAICaptureHandler{})

	okRes := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", nil)
	if okRes.Status != MonitorStatusOperational {
		t.Fatalf("healthy probe should be operational, got %s", okRes.Status)
	}
	if okRes.ErrorCategory != "" {
		t.Fatalf("healthy probe must not carry error category, got %q", okRes.ErrorCategory)
	}

	mismatch := &openAICaptureHandler{rawResponse: `{"choices":[{"message":{"content":"wrong answer"}}]}`}
	mismatchEndpoint := setupFakeOpenAI(t, mismatch)
	failedRes := runCheckForModel(context.Background(), MonitorProviderOpenAI, mismatchEndpoint, "sk-openai", "gpt-5.5", nil)
	if failedRes.Status != MonitorStatusFailed {
		t.Fatalf("challenge mismatch should be failed, got %s", failedRes.Status)
	}
	if failedRes.ErrorCategory != "" {
		t.Fatalf("challenge mismatch must not carry error category, got %q", failedRes.ErrorCategory)
	}
}

func TestValidateTargetKind(t *testing.T) {
	for _, kind := range []string{"", "endpoint", "gateway_group", " endpoint "} {
		if err := validateTargetKind(kind); err != nil {
			t.Fatalf("validateTargetKind(%q) should pass, got %v", kind, err)
		}
	}
	if err := validateTargetKind("upstream_pool"); err == nil {
		t.Fatal("validateTargetKind should reject unknown values")
	}
	if got := defaultTargetKind(""); got != MonitorTargetKindEndpoint {
		t.Fatalf("defaultTargetKind(\"\") = %q, want endpoint", got)
	}
	if got := defaultTargetKind("gateway_group"); got != MonitorTargetKindGatewayGroup {
		t.Fatalf("defaultTargetKind(gateway_group) = %q, want gateway_group", got)
	}
}
