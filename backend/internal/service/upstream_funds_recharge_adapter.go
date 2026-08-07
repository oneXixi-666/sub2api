package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type UpstreamPaymentChannel struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Currency       string  `json:"currency"`
	SingleMin      float64 `json:"single_min"`
	SingleMax      float64 `json:"single_max"`
	FeeRate        float64 `json:"fee_rate"`
	DailyRemaining float64 `json:"daily_remaining"`
}

type UpstreamProviderOrderUpdate struct {
	ProviderOrderID string
	Status          string
	PayAmount       float64
	Currency        string
	PaymentQR       string
	PaymentURL      string
	ExpiresAt       *time.Time
}

type UpstreamRechargeProvider interface {
	RechargeConfigured(wallet *UpstreamWallet, accounts []*Account) bool
	ListPaymentChannels(ctx context.Context, wallet *UpstreamWallet, accounts []*Account) ([]UpstreamPaymentChannel, error)
	CreateRechargeOrder(ctx context.Context, wallet *UpstreamWallet, accounts []*Account, amount float64, channelID string) (*UpstreamProviderOrderUpdate, error)
	QueryRechargeOrder(ctx context.Context, wallet *UpstreamWallet, accounts []*Account, providerOrderID string) (*UpstreamProviderOrderUpdate, error)
}

type sub2APIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type sub2APICheckoutInfo struct {
	Methods         map[string]sub2APIMethodLimit `json:"methods"`
	BalanceDisabled bool                          `json:"balance_disabled"`
}

type sub2APIMethodLimit struct {
	Currency       string  `json:"currency"`
	DisplayName    string  `json:"display_name"`
	DailyRemaining float64 `json:"daily_remaining"`
	SingleMin      float64 `json:"single_min"`
	SingleMax      float64 `json:"single_max"`
	FeeRate        float64 `json:"fee_rate"`
	Available      bool    `json:"available"`
}

type sub2APICreateOrderResponse struct {
	OrderID     int64     `json:"order_id"`
	PayAmount   float64   `json:"pay_amount"`
	Status      string    `json:"status"`
	PaymentType string    `json:"payment_type"`
	PayURL      string    `json:"pay_url"`
	QRCode      string    `json:"qr_code"`
	Currency    string    `json:"currency"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type sub2APIPaymentOrder struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	PayAmount float64   `json:"pay_amount"`
	Currency  string    `json:"currency"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (p *sub2APIUsageBalanceProvider) RechargeConfigured(_ *UpstreamWallet, accounts []*Account) bool {
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

func (p *sub2APIUsageBalanceProvider) ListPaymentChannels(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
) ([]UpstreamPaymentChannel, error) {
	account := firstRechargeAccount(accounts)
	if account == nil || !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	var checkout sub2APICheckoutInfo
	if err := p.doPanelJSON(ctx, account, http.MethodGet, "/api/v1/payment/checkout-info", nil, &checkout); err != nil {
		return nil, err
	}
	if checkout.BalanceDisabled {
		return []UpstreamPaymentChannel{}, nil
	}
	channels := make([]UpstreamPaymentChannel, 0, len(checkout.Methods))
	for id, method := range checkout.Methods {
		if !method.Available {
			continue
		}
		name := strings.TrimSpace(method.DisplayName)
		if name == "" {
			name = id
		}
		currency := strings.ToUpper(strings.TrimSpace(method.Currency))
		if currency == "" {
			currency = wallet.Currency
		}
		channels = append(channels, UpstreamPaymentChannel{
			ID: strings.TrimSpace(id), Name: name, Currency: currency, SingleMin: method.SingleMin,
			SingleMax: method.SingleMax, FeeRate: method.FeeRate, DailyRemaining: method.DailyRemaining,
		})
	}
	return channels, nil
}

func (p *sub2APIUsageBalanceProvider) CreateRechargeOrder(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	amount float64,
	channelID string,
) (*UpstreamProviderOrderUpdate, error) {
	account := firstRechargeAccount(accounts)
	if account == nil || !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	payload := map[string]any{
		"amount": amount, "payment_type": channelID, "order_type": "balance",
		"is_mobile": false, "payment_source": "upstream_funds",
	}
	var result sub2APICreateOrderResponse
	if err := p.doPanelJSON(ctx, account, http.MethodPost, "/api/v1/payment/orders", payload, &result); err != nil {
		return nil, err
	}
	if result.OrderID <= 0 || (strings.TrimSpace(result.QRCode) == "" && strings.TrimSpace(result.PayURL) == "") {
		return nil, &upstreamBalanceAdapterError{code: "invalid_order_response"}
	}
	return &UpstreamProviderOrderUpdate{
		ProviderOrderID: fmt.Sprintf("%d", result.OrderID), Status: result.Status,
		PayAmount: result.PayAmount, Currency: strings.ToUpper(result.Currency),
		PaymentQR: result.QRCode, PaymentURL: result.PayURL, ExpiresAt: timePointer(result.ExpiresAt),
	}, nil
}

func (p *sub2APIUsageBalanceProvider) QueryRechargeOrder(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	providerOrderID string,
) (*UpstreamProviderOrderUpdate, error) {
	account := firstRechargeAccount(accounts)
	if account == nil || !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	var result sub2APIPaymentOrder
	path := "/api/v1/payment/orders/" + strings.TrimSpace(providerOrderID)
	if err := p.doPanelJSON(ctx, account, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	if result.ID <= 0 {
		return nil, &upstreamBalanceAdapterError{code: "invalid_order_response"}
	}
	return &UpstreamProviderOrderUpdate{
		ProviderOrderID: fmt.Sprintf("%d", result.ID), Status: result.Status,
		PayAmount: result.PayAmount, Currency: strings.ToUpper(result.Currency), ExpiresAt: timePointer(result.ExpiresAt),
	}, nil
}

func (p *sub2APIUsageBalanceProvider) doPanelJSON(
	ctx context.Context,
	account *Account,
	method, endpoint string,
	payload any,
	destination any,
) error {
	baseURL, err := p.accountTestService.validateUpstreamBaseURL(strings.TrimSpace(account.GetCredential("base_url")))
	if err != nil {
		return &upstreamBalanceAdapterError{code: "invalid_base_url"}
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return &upstreamBalanceAdapterError{code: "request_build_failed"}
		}
		body = bytes.NewReader(encoded)
	}
	requestCtx, cancel := context.WithTimeout(ctx, upstreamBalanceRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, buildSub2APIPanelEndpointURL(baseURL, endpoint), body)
	if err != nil {
		return &upstreamBalanceAdapterError{code: "request_build_failed"}
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileDefault)))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("upstream_funds_panel_token")))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return &upstreamBalanceAdapterError{code: "proxy_unavailable"}
		}
		proxyURL = account.Proxy.URL()
	}
	var tlsProfile *tlsfingerprint.Profile
	if p.accountTestService.tlsFPProfileService != nil {
		tlsProfile = p.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := p.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return &upstreamBalanceAdapterError{code: "request_failed"}
	}
	if resp == nil || resp.Body == nil {
		return &upstreamBalanceAdapterError{code: "empty_response"}
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceMaxBodyBytes+1))
	if err != nil || len(bodyBytes) > upstreamBalanceMaxBodyBytes {
		return &upstreamBalanceAdapterError{code: "response_read_failed"}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &upstreamBalanceAdapterError{code: fmt.Sprintf("http_%d", resp.StatusCode)}
	}
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		return &upstreamBalanceAdapterError{code: "invalid_response"}
	}
	if err := json.Unmarshal(envelope.Data, destination); err != nil {
		return &upstreamBalanceAdapterError{code: "invalid_response"}
	}
	return nil
}

func firstRechargeAccount(accounts []*Account) *Account {
	for _, account := range accounts {
		if upstreamRedeemAccountConfigured(account) {
			return account
		}
	}
	return nil
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
