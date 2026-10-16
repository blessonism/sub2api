//go:build unit

package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/typesafe"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGroupSystemPromptSystemOneFinalBody(t *testing.T) {
	for _, instructions := range []string{`"client-system-marker"`, `{"text":"client-system-marker","metadata":{"keep":true}}`, `["client-system-marker",{"data":"keep"}]`, `null`} {
		for _, mode := range []string{"prepend", "append", "replace"} {
			t.Run(fmt.Sprintf("%s/%s", instructions, mode), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"jev-latest","state":{"instructions":"user data","text":"hello"},"questions":{"first":{"type":"choice","instructions":%s,"criteria":{"a":"yes","b":"no"}},"second":{"type":"noul","instructions":"client-second"}}}`, instructions))
				original := string(body)
				ctx, c, _ := groupPromptForwardContext(PlatformTypeSafe, mode, "/v1/systemone", body)
				for attempt := 0; attempt < 2; attempt++ {
					upstream := &httpUpstreamRecorder{resp: groupPromptJSONResponse(`{"answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`)}
					svc := newSystemOneTestService(upstream)
					account := &Account{ID: 1, Platform: PlatformTypeSafe, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "http://typesafe.test"}}
					_, err := svc.ForwardSystemOne(ctx, c, account, body)
					require.NoError(t, err)
					_, err = typesafe.ValidateSystemOneRequest(upstream.lastBody)
					require.NoError(t, err)
					require.Equal(t, gjson.GetBytes(body, "state").Raw, gjson.GetBytes(upstream.lastBody, "state").Raw)
					require.Equal(t, gjson.GetBytes(body, "questions.first.criteria").Raw, gjson.GetBytes(upstream.lastBody, "questions.first.criteria").Raw)
					for _, question := range []string{"first", "second"} {
						got := gjson.GetBytes(upstream.lastBody, "questions."+question+".instructions").Raw
						require.Equal(t, 1, strings.Count(got, groupForwardPrompt), "每个独立问题仅注入一次")
						if mode == "replace" {
							require.NotContains(t, got, "client-")
						}
						if instructions != "null" && question == "first" && mode != "replace" {
							require.Contains(t, got, "client-system-marker")
							group, client := strings.Index(got, groupForwardPrompt), strings.Index(got, "client-system-marker")
							if mode == "prepend" {
								require.Less(t, group, client)
							} else {
								require.Greater(t, group, client)
							}
						}
					}
				}
				require.Equal(t, original, string(body))
			})
		}
	}
}
