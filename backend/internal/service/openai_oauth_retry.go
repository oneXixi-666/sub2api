package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	openAIOAuthRetryEnabledKey              = "openai_oauth_retry_enabled"
	openAIOAuthRetryCountKey                = "openai_oauth_retry_count"
	openAIOAuthRetryStatusCodesKey          = "openai_oauth_retry_status_codes"
	openAIOAuthRetryKeywordsKey             = "openai_oauth_retry_keywords"
	openAIOAuthRateLimitRetryEnabledKey     = "openai_oauth_rate_limit_retry_enabled"
	openAIOAuthRateLimitRetryCountKey       = "openai_oauth_rate_limit_retry_count"
	openAIOAuthRateLimitRetryStatusCodesKey = "openai_oauth_rate_limit_status_codes"
	openAIOAuthRateLimitRetryKeywordsKey    = "openai_oauth_rate_limit_retry_keywords"

	defaultOpenAIOAuthRetryCount     = 2
	maxOpenAIOAuthRetryCount         = 10
	defaultOpenAIHTTPRateLimitStatus = http.StatusTooManyRequests
)

// These are upstream capacity-shed signals. They are deliberately built in:
// the upstream may omit error.code and only return the human-readable message.
var openAIOAuthBuiltInRetryKeywords = []string{
	"server_is_overloaded",
	"slow_down",
	"our servers are currently overloaded. please try again later.",
}

type openAIOAuthRetryDecision struct {
	matched                bool
	retryableOnSameAccount bool
	retryLimit             int
	deferAccountState      bool
	requestScopedTransient bool
}

func openAIOAuthCredentialBool(account *Account, key string) bool {
	if !isOpenAIOAuthAccount(account) || account.Credentials == nil {
		return false
	}
	enabled, _ := account.Credentials[key].(bool)
	return enabled
}

func openAIOAuthRetryCount(account *Account, key string) int {
	count := defaultOpenAIOAuthRetryCount
	if account != nil && account.Credentials != nil {
		count = parseOpenAIOAuthRetryInt(account.Credentials[key], count)
	}
	if count < 1 {
		return 1
	}
	if count > maxOpenAIOAuthRetryCount {
		return maxOpenAIOAuthRetryCount
	}
	return count
}

func parseOpenAIOAuthRetryInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func openAIOAuthRetryStatusCodes(account *Account, key string, fallback []int) []int {
	if account == nil || account.Credentials == nil {
		return append([]int(nil), fallback...)
	}
	raw, exists := account.Credentials[key]
	if !exists || raw == nil {
		return append([]int(nil), fallback...)
	}

	var values []any
	switch v := raw.(type) {
	case []any:
		values = v
	case []int:
		values = make([]any, len(v))
		for i, code := range v {
			values[i] = code
		}
	case []string:
		values = make([]any, len(v))
		for i, code := range v {
			values[i] = code
		}
	default:
		return []int{}
	}

	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		code := parseOpenAIOAuthRetryInt(value, 0)
		if code < 100 || code > 599 {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func openAIOAuthRetryKeywords(account *Account, key string) []string {
	if account == nil || account.Credentials == nil {
		return nil
	}
	raw, exists := account.Credentials[key]
	if !exists || raw == nil {
		return nil
	}
	var values []any
	switch v := raw.(type) {
	case []any:
		values = v
	case []string:
		values = make([]any, len(v))
		for i, keyword := range v {
			values[i] = keyword
		}
	default:
		return nil
	}
	keywords := make([]string, 0, len(values))
	for _, value := range values {
		keyword := strings.ToLower(strings.TrimSpace(toOpenAIOAuthRetryString(value)))
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}

func toOpenAIOAuthRetryString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func openAIErrorMatchTexts(upstreamMessage string, responseBody []byte) []string {
	texts := []string{strings.ToLower(strings.TrimSpace(upstreamMessage))}
	if len(responseBody) > 0 {
		for _, path := range []string{
			"error.type",
			"error.code",
			"error.message",
			"error.param",
			"response.error.type",
			"response.error.code",
			"response.error.message",
			"response.error.param",
			"type",
			"code",
			"message",
			"param",
		} {
			if value := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, path).String())); value != "" {
				texts = append(texts, value)
			}
		}
		texts = append(texts, strings.ToLower(string(responseBody)))
	}
	return texts
}

func openAIErrorContainsKeyword(keyword, upstreamMessage string, responseBody []byte) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return false
	}
	for _, text := range openAIErrorMatchTexts(upstreamMessage, responseBody) {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func openAIErrorMatches(statusCode int, upstreamMessage string, responseBody []byte, statusCodes []int, keywords []string) bool {
	for _, code := range statusCodes {
		if code == statusCode {
			return true
		}
	}
	for _, keyword := range keywords {
		if openAIErrorContainsKeyword(keyword, upstreamMessage, responseBody) {
			return true
		}
	}
	return false
}

func isOpenAIUpstreamCapacityShedPayload(payload []byte) bool {
	for _, keyword := range openAIOAuthBuiltInRetryKeywords {
		if openAIErrorContainsKeyword(keyword, "", payload) {
			return true
		}
	}
	return false
}

func isOpenAIOAuthBuiltInCapacityShedError(statusCode int, upstreamMessage string, responseBody []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusServiceUnavailable {
		return isOpenAIUpstreamCapacityShedPayload(responseBody)
	}
	for _, keyword := range openAIOAuthBuiltInRetryKeywords {
		if openAIErrorContainsKeyword(keyword, upstreamMessage, responseBody) {
			return true
		}
	}
	return false
}

func isOpenAIOAuthDeterministicRetryError(upstreamMessage string, responseBody []byte) bool {
	return openAIErrorContainsKeyword("unsupported_value", upstreamMessage, responseBody) ||
		openAIErrorContainsKeyword("x-openai-internal-codex-responses-lite", upstreamMessage, responseBody)
}

func openAIOAuthRetryDecisionForError(account *Account, statusCode int, upstreamMessage string, responseBody []byte) openAIOAuthRetryDecision {
	if !isOpenAIOAuthAccount(account) {
		return openAIOAuthRetryDecision{}
	}
	if isOpenAIOAuthBuiltInCapacityShedError(statusCode, upstreamMessage, responseBody) {
		return openAIOAuthRetryDecision{
			matched:                true,
			retryableOnSameAccount: true,
			retryLimit:             defaultOpenAIOAuthRetryCount,
			deferAccountState:      true,
			requestScopedTransient: true,
		}
	}

	if openAIOAuthCredentialBool(account, openAIOAuthRateLimitRetryEnabledKey) {
		statusCodes := openAIOAuthRetryStatusCodes(account, openAIOAuthRateLimitRetryStatusCodesKey, []int{defaultOpenAIHTTPRateLimitStatus})
		keywords := openAIOAuthRetryKeywords(account, openAIOAuthRateLimitRetryKeywordsKey)
		if openAIErrorMatches(statusCode, upstreamMessage, responseBody, statusCodes, keywords) {
			return openAIOAuthRetryDecision{
				matched:                true,
				retryableOnSameAccount: true,
				retryLimit:             openAIOAuthRetryCount(account, openAIOAuthRateLimitRetryCountKey),
				deferAccountState:      true,
			}
		}
	}

	if openAIOAuthCredentialBool(account, openAIOAuthRetryEnabledKey) {
		statusCodes := openAIOAuthRetryStatusCodes(account, openAIOAuthRetryStatusCodesKey, nil)
		keywords := openAIOAuthRetryKeywords(account, openAIOAuthRetryKeywordsKey)
		if openAIErrorMatches(statusCode, upstreamMessage, responseBody, statusCodes, keywords) {
			deterministic := isOpenAIOAuthDeterministicRetryError(upstreamMessage, responseBody)
			return openAIOAuthRetryDecision{
				matched:                true,
				retryableOnSameAccount: !deterministic,
				retryLimit:             openAIOAuthRetryCount(account, openAIOAuthRetryCountKey),
				deferAccountState:      !deterministic,
			}
		}
	}
	return openAIOAuthRetryDecision{}
}

func (s *OpenAIGatewayService) shouldFailoverOpenAIUpstreamResponseForAccount(account *Account, statusCode int, upstreamMessage string, responseBody []byte) bool {
	decision := openAIOAuthRetryDecisionForError(account, statusCode, upstreamMessage, responseBody)
	return decision.matched || s.shouldFailoverOpenAIUpstreamResponse(statusCode, upstreamMessage, responseBody)
}

func (s *OpenAIGatewayService) handleOpenAIFailoverSideEffects(ctx context.Context, resp *http.Response, account *Account, responseBody []byte, canonicalModel ...string) bool {
	if resp != nil {
		decision := openAIOAuthRetryDecisionForError(account, resp.StatusCode, extractUpstreamErrorMessage(responseBody), responseBody)
		if decision.deferAccountState {
			return false
		}
	}
	return s.handleFailoverSideEffects(ctx, resp, account, responseBody, canonicalModel...)
}

func (s *OpenAIGatewayService) newOpenAIOAuthAwareFailoverError(
	account *Account,
	statusCode int,
	responseHeaders http.Header,
	responseBody []byte,
	upstreamMessage string,
	fallbackRetryableOnSameAccount bool,
) *UpstreamFailoverError {
	failoverErr := newOpenAIUpstreamFailoverError(
		statusCode,
		responseHeaders,
		responseBody,
		upstreamMessage,
		fallbackRetryableOnSameAccount,
	)
	if failoverErr.IsOpenAIRequestBodyTooLarge() {
		return failoverErr
	}
	decision := openAIOAuthRetryDecisionForError(account, statusCode, upstreamMessage, responseBody)
	if !decision.matched {
		return failoverErr
	}
	failoverErr.RetryableOnSameAccount = decision.retryableOnSameAccount
	failoverErr.SameAccountRetryLimit = decision.retryLimit
	failoverErr.DeferAccountState = decision.deferAccountState
	failoverErr.RequestScopedTransient = decision.requestScopedTransient
	return failoverErr
}

// FinalizeOpenAIOAuthRetryState applies a deferred account-state update only
// after the request-local same-account retry budget is exhausted.
func (s *OpenAIGatewayService) FinalizeOpenAIOAuthRetryState(ctx context.Context, account *Account, failoverErr *UpstreamFailoverError) {
	if s == nil || account == nil || failoverErr == nil ||
		!failoverErr.RetryableOnSameAccount || !failoverErr.DeferAccountState ||
		failoverErr.RequestScopedTransient {
		return
	}
	_ = s.handleOpenAIAccountUpstreamError(
		ctx,
		account,
		failoverErr.StatusCode,
		failoverErr.ResponseHeaders,
		failoverErr.ResponseBody,
	)
}
