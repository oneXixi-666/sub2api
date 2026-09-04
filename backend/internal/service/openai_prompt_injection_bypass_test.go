package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const promptInjectionBypassTestAPIKeyID int64 = 88001

func TestOpenAIPromptInjectionBypassMatchesAuthenticatedAPIKeyID(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		OpenAIPromptInjectionBypassAPIKeyIDs: []int64{promptInjectionBypassTestAPIKeyID},
	}}}

	require.True(t, svc.ShouldBypassOpenAIPromptInjection(promptInjectionBypassTestAPIKeyID))
	require.False(t, svc.ShouldBypassOpenAIPromptInjection(promptInjectionBypassTestAPIKeyID+1))
	require.False(t, svc.ShouldBypassOpenAIPromptInjection(0))
}

func TestOpenAIGatewayForwardPromptBypassDoesNotAddInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name             string
		body             string
		wantInstructions string
		wantExists       bool
	}{
		{
			name:       "missing instructions stay missing",
			body:       `{"model":"gpt-5.6-sol","input":"hello","stream":false}`,
			wantExists: false,
		},
		{
			name:             "client instructions are preserved",
			body:             `{"model":"gpt-5.6-sol","instructions":"client-only","input":"hello","stream":false}`,
			wantInstructions: "client-only",
			wantExists:       true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_prompt_bypass", "gpt-5.6-sol")}
			svc := newOpenAIImageGenerationControlTestService(upstream)
			svc.cfg.Gateway.OpenAIPromptInjectionBypassAPIKeyIDs = []int64{promptInjectionBypassTestAPIKeyID}
			c := newPromptInjectionBypassTestContext(tt.body)
			account := promptInjectionBypassOAuthAccount(nil)

			result, err := svc.Forward(context.Background(), c, account, []byte(tt.body))

			require.NoError(t, err)
			require.NotNil(t, result)
			instructions := gjson.GetBytes(upstream.lastBody, "instructions")
			require.Equal(t, tt.wantExists, instructions.Exists())
			if tt.wantExists {
				require.Equal(t, tt.wantInstructions, instructions.String())
			}
			require.NotContains(t, string(upstream.lastBody), "You are Codex")
		})
	}
}

func TestOpenAIGatewayPassthroughPromptBypassForwardsMissingInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"gpt-5.1-codex-max","input":"hello","stream":true}`
	upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_passthrough_prompt_bypass", "gpt-5.1-codex-max")}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	svc.cfg.Gateway.OpenAIPromptInjectionBypassAPIKeyIDs = []int64{promptInjectionBypassTestAPIKeyID}
	c := newPromptInjectionBypassTestContext(body)
	account := promptInjectionBypassOAuthAccount(map[string]any{
		"openai_passthrough":                        true,
		"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff,
	})

	result, err := svc.Forward(context.Background(), c, account, []byte(body))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
}

func TestOpenAIGatewayMessagesPromptBypassSkipsServerPrompts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"claude-sonnet-4-5","max_tokens":16,"system":"client-system","messages":[{"role":"user","content":"hello"}],"stream":false}`
	upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_messages_prompt_bypass", "gpt-5.5")}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForcedCodexInstructionsTemplate:      "server-prefix\n\n{{ .ExistingInstructions }}",
			OpenAIPromptInjectionBypassAPIKeyIDs: []int64{promptInjectionBypassTestAPIKeyID},
		}},
		httpUpstream: upstream,
	}
	c := newPromptInjectionBypassTestContext(body)
	account := promptInjectionBypassOAuthAccount(nil)

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, []byte(body), "", "gpt-5.5")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, gjson.GetBytes(upstream.lastBody, "instructions").String())
	require.NotContains(t, string(upstream.lastBody), "server-prefix")
	require.NotContains(t, string(upstream.lastBody), openAICompatClaudeCodeTodoGuardMarker)
	require.Contains(t, string(upstream.lastBody), "client-system")
}

func TestCodexTransformPromptBypassSkipsSparkInstructions(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": []any{map[string]any{"role": "user", "content": "hello"}},
	}

	result := applyCodexOAuthTransformWithOptions(reqBody, codexOAuthTransformOptions{
		SkipInjectedInstructions: true,
	})

	require.NoError(t, result.Error)
	_, exists := reqBody["instructions"]
	require.False(t, exists)
}

func TestSuppressCodexModelsManifestPromptInjection(t *testing.T) {
	manifest := &CodexModelsManifest{Body: []byte(`{"models":[{"slug":"gpt-5.6-sol","model_messages":{"instructions_template":"server prompt","approvals":{"ask":"prompt"}},"include_skills_usage_instructions":true,"include_plugin_usage_instructions":true,"include_apps_usage_instructions":true}]}`)}

	require.NoError(t, SuppressCodexModelsManifestPromptInjection(manifest, ""))
	require.NotEmpty(t, manifest.ETag)
	require.False(t, manifest.NotModified)
	require.Equal(t, "", gjson.GetBytes(manifest.Body, "models.0.model_messages.instructions_template").String())
	require.Equal(t, gjson.Null, gjson.GetBytes(manifest.Body, "models.0.model_messages.approvals").Type)
	require.False(t, gjson.GetBytes(manifest.Body, "models.0.include_skills_usage_instructions").Bool())
	require.False(t, gjson.GetBytes(manifest.Body, "models.0.include_plugin_usage_instructions").Bool())
	require.False(t, gjson.GetBytes(manifest.Body, "models.0.include_apps_usage_instructions").Bool())

	etag := manifest.ETag
	manifest = &CodexModelsManifest{Body: []byte(`{"models":[{"slug":"gpt-5.6-sol","model_messages":{"instructions_template":"server prompt"}}]}`)}
	require.NoError(t, SuppressCodexModelsManifestPromptInjection(manifest, etag))
	require.True(t, manifest.NotModified)
	require.Nil(t, manifest.Body)
}

func newPromptInjectionBypassTestContext(body string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: promptInjectionBypassTestAPIKeyID})
	return c
}

func promptInjectionBypassOAuthAccount(extra map[string]any) *Account {
	return &Account{
		ID:          88002,
		Name:        "prompt-bypass-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-account",
		},
		Extra:          extra,
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}
}
