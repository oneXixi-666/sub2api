package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	UpstreamBalanceSourceManual       = "manual"
	UpstreamBalanceSourceSub2APIUsage = "sub2api_usage"

	upstreamBalanceRequestTimeout = 8 * time.Second
	upstreamBalanceMaxBodyBytes   = 64 * 1024
)

type BalanceSnapshot struct {
	Balance   float64
	Currency  string
	FetchedAt time.Time
}

type UpstreamBalanceProvider interface {
	BalanceConfigured(wallet *UpstreamWallet, accounts []*Account) bool
	RefreshBalance(ctx context.Context, wallet *UpstreamWallet, accounts []*Account) (*BalanceSnapshot, error)
}

type UpstreamCodeRedeemer interface {
	RedeemConfigured(wallet *UpstreamWallet, accounts []*Account) bool
	RedeemCode(ctx context.Context, wallet *UpstreamWallet, accounts []*Account, code string) error
}

type sub2APIUsageBalanceProvider struct {
	accountTestService *AccountTestService
}

type upstreamBalanceAdapterError struct {
	code string
}

func (e *upstreamBalanceAdapterError) Error() string {
	return e.code
}

func NewSub2APIUsageBalanceProvider(accountTestService *AccountTestService) *sub2APIUsageBalanceProvider {
	return &sub2APIUsageBalanceProvider{accountTestService: accountTestService}
}

func (p *sub2APIUsageBalanceProvider) BalanceConfigured(_ *UpstreamWallet, accounts []*Account) bool {
	if p == nil || p.accountTestService == nil || p.accountTestService.httpUpstream == nil {
		return false
	}
	for _, account := range accounts {
		if upstreamBalanceAccountConfigured(account) {
			return true
		}
	}
	return false
}

func (p *sub2APIUsageBalanceProvider) RefreshBalance(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
) (*BalanceSnapshot, error) {
	if !p.BalanceConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "adapter_not_configured"}
	}

	failures := make([]string, 0, len(accounts))
	seenFailures := make(map[string]struct{})
	for _, account := range accounts {
		if !upstreamBalanceAccountConfigured(account) {
			continue
		}
		snapshot, err := p.refreshAccountBalance(ctx, wallet, account)
		if err == nil {
			return snapshot, nil
		}
		code := upstreamBalanceErrorSummary(err)
		if _, exists := seenFailures[code]; !exists {
			seenFailures[code] = struct{}{}
			failures = append(failures, code)
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			break
		}
	}
	if len(failures) == 0 {
		failures = append(failures, "adapter_not_configured")
	}
	return nil, &upstreamBalanceAdapterError{code: strings.Join(failures, ",")}
}

func (p *sub2APIUsageBalanceProvider) RedeemConfigured(_ *UpstreamWallet, accounts []*Account) bool {
	if p == nil || p.accountTestService == nil || p.accountTestService.httpUpstream == nil {
		return false
	}
	for _, account := range accounts {
		if upstreamRedeemAccountConfigured(account) {
			return true
		}
	}
	return false
}

func (p *sub2APIUsageBalanceProvider) RedeemCode(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	code string,
) error {
	if !p.RedeemConfigured(wallet, accounts) {
		return &upstreamBalanceAdapterError{code: "redeem_not_configured"}
	}
	for _, account := range accounts {
		if !upstreamRedeemAccountConfigured(account) {
			continue
		}
		if err := p.redeemCodeWithAccount(ctx, account, code); err == nil {
			return nil
		} else {
			return err
		}
	}
	return &upstreamBalanceAdapterError{code: "redeem_not_configured"}
}

func (p *sub2APIUsageBalanceProvider) redeemCodeWithAccount(ctx context.Context, account *Account, code string) error {
	baseURL, err := p.accountTestService.validateUpstreamBaseURL(strings.TrimSpace(account.GetCredential("base_url")))
	if err != nil {
		return &upstreamBalanceAdapterError{code: "invalid_base_url"}
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return &upstreamBalanceAdapterError{code: "proxy_unavailable"}
		}
		proxyURL = account.Proxy.URL()
	}
	payload, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return &upstreamBalanceAdapterError{code: "request_build_failed"}
	}
	requestCtx, cancel := context.WithTimeout(ctx, upstreamBalanceRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		buildSub2APIPanelEndpointURL(baseURL, "/api/v1/redeem"),
		bytes.NewReader(payload),
	)
	if err != nil {
		return &upstreamBalanceAdapterError{code: "request_build_failed"}
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileDefault)))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("upstream_funds_panel_token")))

	var tlsProfile *tlsfingerprint.Profile
	if p.accountTestService.tlsFPProfileService != nil {
		tlsProfile = p.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := p.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return &upstreamBalanceAdapterError{code: "redeem_request_failed"}
	}
	if resp == nil || resp.Body == nil {
		return &upstreamBalanceAdapterError{code: "redeem_empty_response"}
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceMaxBodyBytes+1))
	if readErr != nil || len(bodyBytes) > upstreamBalanceMaxBodyBytes {
		return &upstreamBalanceAdapterError{code: "redeem_response_read_failed"}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &upstreamBalanceAdapterError{code: fmt.Sprintf("redeem_http_%d", resp.StatusCode)}
	}
	return validateSub2APIRedeemResponse(bodyBytes)
}

func validateSub2APIRedeemResponse(body []byte) error {
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &upstreamBalanceAdapterError{code: "redeem_invalid_response"}
	}
	if envelope.Code != 0 {
		return &upstreamBalanceAdapterError{code: "redeem_rejected"}
	}
	return nil
}

func (p *sub2APIUsageBalanceProvider) refreshAccountBalance(
	ctx context.Context,
	wallet *UpstreamWallet,
	account *Account,
) (*BalanceSnapshot, error) {
	baseURL, err := p.accountTestService.validateUpstreamBaseURL(strings.TrimSpace(account.GetCredential("base_url")))
	if err != nil {
		return nil, &upstreamBalanceAdapterError{code: "invalid_base_url"}
	}

	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return nil, &upstreamBalanceAdapterError{code: "proxy_unavailable"}
		}
		proxyURL = account.Proxy.URL()
	}

	requestCtx, cancel := context.WithTimeout(ctx, upstreamBalanceRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		buildOpenAIEndpointURL(baseURL, "/v1/usage"),
		bytes.NewReader(nil),
	)
	if err != nil {
		return nil, &upstreamBalanceAdapterError{code: "request_build_failed"}
	}
	profile := HTTPUpstreamProfileDefault
	if account.Platform == PlatformOpenAI {
		profile = HTTPUpstreamProfileOpenAI
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), profile)))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	account.ApplyHeaderOverrides(req.Header)

	var tlsProfile *tlsfingerprint.Profile
	if p.accountTestService.tlsFPProfileService != nil {
		tlsProfile = p.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := p.accountTestService.httpUpstream.DoWithTLS(
		req,
		proxyURL,
		account.ID,
		account.Concurrency,
		tlsProfile,
	)
	if err != nil {
		return nil, &upstreamBalanceAdapterError{code: "request_failed"}
	}
	if resp == nil || resp.Body == nil {
		return nil, &upstreamBalanceAdapterError{code: "empty_response"}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceMaxBodyBytes+1))
	if err != nil {
		return nil, &upstreamBalanceAdapterError{code: "response_read_failed"}
	}
	if len(body) > upstreamBalanceMaxBodyBytes {
		return nil, &upstreamBalanceAdapterError{code: "response_too_large"}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &upstreamBalanceAdapterError{code: fmt.Sprintf("http_%d", resp.StatusCode)}
	}

	balance, currency, err := parseSub2APIUsageBalance(body, wallet.Currency)
	if err != nil {
		return nil, err
	}
	return &BalanceSnapshot{
		Balance:   balance,
		Currency:  currency,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func upstreamBalanceAccountConfigured(account *Account) bool {
	return account != nil &&
		account.Type == AccountTypeAPIKey &&
		strings.TrimSpace(account.GetCredential("api_key")) != "" &&
		strings.TrimSpace(account.GetCredential("base_url")) != ""
}

func upstreamRedeemAccountConfigured(account *Account) bool {
	return upstreamBalanceAccountConfigured(account) &&
		strings.TrimSpace(account.GetCredential("upstream_funds_panel_token")) != ""
}

func buildSub2APIPanelEndpointURL(baseURL, endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/v1") {
		trimmed = strings.TrimSuffix(trimmed, "/v1")
	}
	return trimmed + "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
}

func parseSub2APIUsageBalance(body []byte, fallbackCurrency string) (float64, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return 0, "", &upstreamBalanceAdapterError{code: "invalid_response"}
	}

	balance, ok := upstreamUsageNumber(payload["remaining"])
	quota, _ := payload["quota"].(map[string]any)
	if !ok && quota != nil {
		balance, ok = upstreamUsageNumber(quota["remaining"])
	}
	if !ok {
		balance, ok = upstreamUsageNumber(payload["balance"])
	}
	if !ok || math.IsNaN(balance) || math.IsInf(balance, 0) || balance < 0 {
		return 0, "", &upstreamBalanceAdapterError{code: "invalid_balance"}
	}

	currency := upstreamUsageString(payload["unit"])
	if currency == "" && quota != nil {
		currency = upstreamUsageString(quota["unit"])
	}
	if currency == "" {
		currency = fallbackCurrency
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !isCurrencyCode(currency) {
		return 0, "", &upstreamBalanceAdapterError{code: "invalid_currency"}
	}
	return balance, currency, nil
}

func upstreamUsageNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func upstreamUsageString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func upstreamBalanceErrorSummary(err error) string {
	var adapterErr *upstreamBalanceAdapterError
	if errors.As(err, &adapterErr) && adapterErr.code != "" {
		return adapterErr.code
	}
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	return "refresh_failed"
}
