package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthRetryDecision_BuiltInCapacityMessageWithoutCode(t *testing.T) {
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{},
	}
	body := []byte(`{"error":{"message":"Our servers are currently overloaded. Please try again later."}}`)

	decision := openAIOAuthRetryDecisionForError(account, http.StatusBadRequest, "", body)

	require.True(t, decision.matched)
	require.True(t, decision.retryableOnSameAccount)
	require.Equal(t, defaultOpenAIOAuthRetryCount, decision.retryLimit)
	require.True(t, decision.deferAccountState)
	require.True(t, decision.requestScopedTransient)
	require.True(t, isOpenAIUpstreamCapacityShedPayload(body))
}

func TestOpenAIOAuthRetryDecision_RateLimitKeywordAndStatus(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			openAIOAuthRateLimitRetryEnabledKey:     true,
			openAIOAuthRateLimitRetryCountKey:       4,
			openAIOAuthRateLimitRetryStatusCodesKey: []any{},
			openAIOAuthRateLimitRetryKeywordsKey:    []any{"rate_limit_error"},
		},
	}

	decision := openAIOAuthRetryDecisionForError(
		account,
		http.StatusBadGateway,
		`{"error":{"type":"rate_limit_error"}}`,
		[]byte(`{"error":{"type":"rate_limit_error"}}`),
	)

	require.True(t, decision.matched)
	require.True(t, decision.retryableOnSameAccount)
	require.Equal(t, 4, decision.retryLimit)
	require.False(t, decision.requestScopedTransient)
}

func TestOpenAIOAuthRetryDecision_DeterministicKeywordFailsOverWithoutSameAccountRetry(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			openAIOAuthRetryEnabledKey:     true,
			openAIOAuthRetryStatusCodesKey: []any{400},
			openAIOAuthRetryKeywordsKey:    []any{"unsupported_value"},
		},
	}

	decision := openAIOAuthRetryDecisionForError(
		account,
		http.StatusBadRequest,
		`This model is not supported when using X-OpenAI-Internal-Codex-Responses-Lite.`,
		[]byte(`{"error":{"code":"unsupported_value","message":"This model is not supported when using X-OpenAI-Internal-Codex-Responses-Lite."}}`),
	)

	require.True(t, decision.matched)
	require.False(t, decision.retryableOnSameAccount)
	require.False(t, decision.deferAccountState)
}

func TestOpenAIOAuthRetryDecision_DoesNotChangeOrdinaryBadRequest(t *testing.T) {
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{},
	}

	require.False(t, openAIOAuthRetryDecisionForError(
		account,
		http.StatusBadRequest,
		"invalid parameter",
		[]byte(`{"error":{"message":"invalid parameter"}}`),
	).matched)
	require.False(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponseForAccount(
		&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		http.StatusBadRequest,
		"invalid parameter",
		[]byte(`{"error":{"message":"invalid parameter"}}`),
	))
}
