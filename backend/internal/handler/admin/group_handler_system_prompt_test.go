package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupRequestsDecodeSystemPromptInjection(t *testing.T) {
	var create CreateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"system_prompt_injection":{"enabled":true,"mode":"prepend","content":"custom"}}`), &create))
	require.Equal(t, service.GroupSystemPromptInjection{Enabled: true, Mode: "prepend", Content: "custom"}, create.SystemPromptInjection)
	var update UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"system_prompt_injection":{"enabled":false,"mode":"prepend","content":"custom"}}`), &update))
	require.NotNil(t, update.SystemPromptInjection)
	require.False(t, update.SystemPromptInjection.Enabled)
	require.Equal(t, "custom", update.SystemPromptInjection.Content)
	var omitted UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.SystemPromptInjection)
}

func TestGroupHandlerSystemPromptInjectionSimpleMode(t *testing.T) {
	svc := newStubAdminService()
	want := service.GroupSystemPromptInjection{Enabled: true, Mode: "replace", Content: "  private prompt\n"}
	svc.groups = []service.Group{{ID: 1, Name: "simple", Platform: service.PlatformOpenAI, SystemPromptInjection: want}}
	router := newSimpleModeGroupRouter(svc)
	configJSON, err := json.Marshal(want)
	require.NoError(t, err)
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		path := "/groups"
		if method == http.MethodPut {
			path += "/1"
		}
		body := append([]byte(`{"name":"simple","platform":"openai","system_prompt_injection":`), configJSON...)
		body = append(body, '}')
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	require.Len(t, svc.createdGroups, 1)
	require.Equal(t, want, svc.createdGroups[0].SystemPromptInjection)
	require.Len(t, svc.updatedGroups, 1)
	require.NotNil(t, svc.updatedGroups[0].SystemPromptInjection)
	require.Equal(t, want, *svc.updatedGroups[0].SystemPromptInjection)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/all", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Data []struct {
			SystemPromptInjection service.GroupSystemPromptInjection `json:"system_prompt_injection"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.Equal(t, want, payload.Data[0].SystemPromptInjection, "简单模式重新编辑时必须能读回完整配置")
	var omitted UpdateGroupRequest
	sanitizeUpdateGroupRequestForSimpleMode(&omitted)
	require.Nil(t, omitted.SystemPromptInjection)
}
