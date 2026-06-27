package handler

import (
	"context"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const conversationCaptureStateKey = "conversation_capture_state"

type conversationCaptureWriter struct {
	gin.ResponseWriter
	mu         sync.Mutex
	buf        []byte
	limit      int
	truncated  bool
	disconnect bool
}

type conversationCaptureState struct {
	mu           sync.Mutex
	svc          *service.ConversationCaptureService
	decision     service.ConversationCaptureDecision
	meta         service.ConversationCaptureMeta
	responseRaw  []byte
	metaReady    bool
	responseDone bool
	truncated    bool
	disconnect   bool
	submitted    bool
}

func newConversationCaptureWriter(w gin.ResponseWriter, limit int) *conversationCaptureWriter {
	return &conversationCaptureWriter{ResponseWriter: w, limit: limit}
}

func (w *conversationCaptureWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil {
		w.disconnect = true
	}
	if n > 0 {
		w.appendLocked(data[:n])
	}
	return n, err
}

func (w *conversationCaptureWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil {
		w.disconnect = true
	}
	if n > 0 {
		w.appendLocked([]byte(s[:n]))
	}
	return n, err
}

func (w *conversationCaptureWriter) Snapshot() ([]byte, bool, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf...), w.truncated, w.disconnect
}

func (w *conversationCaptureWriter) appendLocked(data []byte) {
	if w.limit <= 0 || len(w.buf) >= w.limit {
		w.truncated = true
		return
	}
	remain := w.limit - len(w.buf)
	if len(data) > remain {
		w.buf = append(w.buf, data[:remain]...)
		w.truncated = true
		return
	}
	w.buf = append(w.buf, data...)
}

func (h *OpenAIGatewayHandler) ChatCompletionsWithConversationCapture(c *gin.Context) {
	h.withConversationCapture(c, service.ConversationCaptureEndpointChatCompletions, h.ChatCompletions)
}

func (h *OpenAIGatewayHandler) ResponsesWithConversationCapture(c *gin.Context) {
	h.withConversationCapture(c, service.ConversationCaptureEndpointResponses, h.Responses)
}

func (h *OpenAIGatewayHandler) withConversationCapture(c *gin.Context, endpointKind string, next gin.HandlerFunc) {
	captureSvc := h.conversationCaptureService
	if captureSvc == nil {
		next(c)
		return
	}
	apiKey, _ := middleware2.GetAPIKeyFromContext(c)
	subject, subjectOK := middleware2.GetAuthSubjectFromContext(c)
	if !subjectOK {
		next(c)
		return
	}
	apiKeyID := int64(0)
	if apiKey != nil {
		apiKeyID = apiKey.ID
	}
	decision := captureSvc.DecideForEndpoint(c.Request.Context(), service.ConversationCaptureSubject{
		UserID:   subject.UserID,
		APIKeyID: apiKeyID,
	}, endpointKind)
	if !decision.Capture {
		next(c)
		return
	}

	original := c.Writer
	cw := newConversationCaptureWriter(original, decision.MaxTurnPayloadBytes)
	state := &conversationCaptureState{svc: captureSvc, decision: decision}
	c.Set(conversationCaptureStateKey, state)
	c.Writer = cw
	next(c)
	c.Writer = original

	responseRaw, truncated, disconnect := cw.Snapshot()
	state.setResponse(responseRaw, truncated, disconnect)
}

func publishConversationCaptureMetaToState(state *conversationCaptureState, meta service.ConversationCaptureMeta) {
	if state == nil {
		return
	}
	state.setMeta(meta)
}

func requestContextValue(ctx context.Context, key any) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func clientRequestIDFromContext(ctx context.Context) string {
	return requestContextValue(ctx, ctxkey.ClientRequestID)
}

func (s *conversationCaptureState) setMeta(meta service.ConversationCaptureMeta) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.meta = meta
	s.meta.RequestBody = append([]byte(nil), meta.RequestBody...)
	s.metaReady = true
	s.maybeSubmitLocked()
	s.mu.Unlock()
}

func (s *conversationCaptureState) setResponse(responseRaw []byte, truncated, disconnect bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.responseRaw = append([]byte(nil), responseRaw...)
	s.truncated = truncated
	s.disconnect = disconnect
	s.responseDone = true
	s.maybeSubmitLocked()
	s.mu.Unlock()
}

func (s *conversationCaptureState) maybeSubmitLocked() {
	if s.submitted || !s.metaReady || !s.responseDone || s.svc == nil {
		return
	}
	s.submitted = true
	s.svc.Submit(s.decision, service.ConversationCaptureInput{
		Meta:             s.meta,
		ResponseRaw:      append([]byte(nil), s.responseRaw...),
		ClientDisconnect: s.disconnect,
		Truncated:        s.truncated,
	})
}
