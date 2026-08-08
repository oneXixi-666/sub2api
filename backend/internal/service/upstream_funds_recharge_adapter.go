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

type UpstreamPaymentChannel struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Currency       string  `json:"currency"`
	SingleMin      float64 `json:"single_min"`
	SingleMax      float64 `json:"single_max"`
	FeeRate        float64 `json:"fee_rate"`
	DailyRemaining float64 `json:"daily_remaining"`
}

const (
	upstreamFundsAlipayChannelID       = "alipay"
	upstreamFundsAlipayChannelCurrency = "CNY"
)

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
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	PayAmount   float64   `json:"pay_amount"`
	Currency    string    `json:"currency"`
	PaymentType string    `json:"payment_type"`
	OutTradeNo  string    `json:"out_trade_no"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (p *sub2APIUsageBalanceProvider) RechargeConfigured(wallet *UpstreamWallet, accounts []*Account) bool {
	if p == nil || p.accountTestService == nil || p.accountTestService.httpUpstream == nil {
		return false
	}
	if p.panelSessions != nil && p.panelSessions.panelSessionConfigured(wallet, accounts) {
		return true
	}
	for _, account := range accounts {
		if upstreamLegacyPanelAccountConfigured(account) {
			return true
		}
	}
	return false
}

func (p *sub2APIUsageBalanceProvider) ListPaymentChannels(
	_ context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
) ([]UpstreamPaymentChannel, error) {
	if !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	// The funds center deliberately exposes one stable method. Provider-side
	// checkout configuration can change independently and must not make the
	// admin flow disappear or select a different payment rail. Alipay settles
	// in CNY independently of the wallet balance currency.
	return []UpstreamPaymentChannel{{
		ID: upstreamFundsAlipayChannelID, Name: "支付宝", Currency: upstreamFundsAlipayChannelCurrency,
	}}, nil
}

func (p *sub2APIUsageBalanceProvider) CreateRechargeOrder(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	amount float64,
	_ string,
) (*UpstreamProviderOrderUpdate, error) {
	if !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	payload := map[string]any{
		"amount": amount, "payment_type": upstreamFundsAlipayChannelID, "order_type": "balance",
		"is_mobile": false, "payment_source": "upstream_funds",
	}
	var result sub2APICreateOrderResponse
	if err := p.doPanelJSON(ctx, wallet, accounts, http.MethodPost, "/api/v1/payment/orders", payload, &result); err != nil {
		return nil, err
	}
	if result.OrderID <= 0 || !isFinitePositive(result.PayAmount) || !isUpstreamAlipayPaymentType(result.PaymentType) ||
		(strings.TrimSpace(result.QRCode) == "" && strings.TrimSpace(result.PayURL) == "") {
		return nil, &upstreamBalanceAdapterError{code: "invalid_order_response"}
	}
	return &UpstreamProviderOrderUpdate{
		ProviderOrderID: fmt.Sprintf("%d", result.OrderID), Status: result.Status,
		PayAmount: result.PayAmount, Currency: normalizeUpstreamAlipayCurrency(result.Currency),
		PaymentQR: result.QRCode, PaymentURL: result.PayURL, ExpiresAt: timePointer(result.ExpiresAt),
	}, nil
}

func (p *sub2APIUsageBalanceProvider) QueryRechargeOrder(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	providerOrderID string,
) (*UpstreamProviderOrderUpdate, error) {
	if !p.RechargeConfigured(wallet, accounts) {
		return nil, &upstreamBalanceAdapterError{code: "recharge_not_configured"}
	}
	providerOrderID = strings.TrimSpace(providerOrderID)
	expectedID, err := strconv.ParseInt(providerOrderID, 10, 64)
	if err != nil || expectedID <= 0 || strconv.FormatInt(expectedID, 10) != providerOrderID {
		return nil, &upstreamBalanceAdapterError{code: "invalid_provider_order_id"}
	}
	var result sub2APIPaymentOrder
	path := "/api/v1/payment/orders/" + providerOrderID
	if err := p.doPanelJSON(ctx, wallet, accounts, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	if !validSub2APIPaymentOrder(result, expectedID) {
		return nil, &upstreamBalanceAdapterError{code: "invalid_order_response"}
	}
	if shouldVerifySub2APIPaymentOrder(result) {
		var verified sub2APIPaymentOrder
		verifyErr := p.doPanelJSON(ctx, wallet, accounts, http.MethodPost, "/api/v1/payment/orders/verify", map[string]any{
			"out_trade_no": strings.TrimSpace(result.OutTradeNo),
		}, &verified)
		// A third-party Sub2API may be an older build without the verify route.
		// Only explicit route absence is compatible with passive polling; auth,
		// transport, and malformed-response failures must remain visible.
		if verifyErr != nil {
			if !isMissingUpstreamVerifyEndpointError(verifyErr) {
				return nil, verifyErr
			}
		} else {
			if !validSub2APIPaymentOrder(verified, result.ID) ||
				!strings.EqualFold(strings.TrimSpace(verified.OutTradeNo), strings.TrimSpace(result.OutTradeNo)) ||
				!strings.EqualFold(strings.TrimSpace(verified.PaymentType), strings.TrimSpace(result.PaymentType)) ||
				!strings.EqualFold(normalizeUpstreamAlipayCurrency(verified.Currency), normalizeUpstreamAlipayCurrency(result.Currency)) ||
				math.Abs(verified.PayAmount-result.PayAmount) > 0.01 {
				return nil, &upstreamBalanceAdapterError{code: "invalid_verify_response"}
			}
			// The verify response is authoritative for payment status, while the
			// initial GET anchors immutable order identity and amount.
			result.Status = verified.Status
			if !verified.ExpiresAt.IsZero() {
				result.ExpiresAt = verified.ExpiresAt
			}
		}
	}
	return &UpstreamProviderOrderUpdate{
		ProviderOrderID: fmt.Sprintf("%d", result.ID), Status: result.Status,
		PayAmount: result.PayAmount, Currency: normalizeUpstreamAlipayCurrency(result.Currency), ExpiresAt: timePointer(result.ExpiresAt),
	}, nil
}

func validSub2APIPaymentOrder(order sub2APIPaymentOrder, expectedID int64) bool {
	return order.ID > 0 &&
		(expectedID == 0 || order.ID == expectedID) &&
		isFinitePositive(order.PayAmount) &&
		isUpstreamAlipayPaymentType(order.PaymentType) &&
		strings.TrimSpace(order.Status) != ""
}

func isMissingUpstreamVerifyEndpointError(err error) bool {
	var adapterErr *upstreamBalanceAdapterError
	if !errors.As(err, &adapterErr) {
		return false
	}
	return adapterErr.code == "http_404" || adapterErr.code == "http_405"
}

func shouldVerifySub2APIPaymentOrder(order sub2APIPaymentOrder) bool {
	outTradeNo := strings.TrimSpace(order.OutTradeNo)
	if outTradeNo == "" || len(outTradeNo) > 64 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(order.Status)) {
	case "pending", "pending_payment", "expired":
		return true
	default:
		return false
	}
}

func isUpstreamAlipayPaymentType(paymentType string) bool {
	paymentType = strings.ToLower(strings.TrimSpace(paymentType))
	return paymentType == upstreamFundsAlipayChannelID || strings.HasPrefix(paymentType, upstreamFundsAlipayChannelID+"_")
}

func normalizeUpstreamAlipayCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return upstreamFundsAlipayChannelCurrency
	}
	return currency
}

func (p *sub2APIUsageBalanceProvider) doPanelJSON(
	ctx context.Context,
	wallet *UpstreamWallet,
	accounts []*Account,
	method, endpoint string,
	payload any,
	destination any,
) error {
	account, token, err := p.resolvePanelCredential(ctx, wallet, accounts)
	if err != nil {
		return err
	}
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
	req.Header.Set("Authorization", "Bearer "+token)
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
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil || envelope.Code != 0 {
		return &upstreamBalanceAdapterError{code: "invalid_response"}
	}
	if destination == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &upstreamBalanceAdapterError{code: "invalid_response"}
	}
	if err := json.Unmarshal(envelope.Data, destination); err != nil {
		return &upstreamBalanceAdapterError{code: "invalid_response"}
	}
	return nil
}

func (p *sub2APIUsageBalanceProvider) resolvePanelCredential(ctx context.Context, wallet *UpstreamWallet, accounts []*Account) (*Account, string, error) {
	if p.panelSessions != nil && p.panelSessions.panelSessionConfigured(wallet, accounts) {
		return p.panelSessions.resolvePanelCredential(ctx, wallet, accounts)
	}
	for _, account := range accounts {
		if upstreamLegacyPanelAccountConfigured(account) {
			return account, strings.TrimSpace(account.GetCredential("upstream_funds_panel_token")), nil
		}
	}
	return nil, "", &upstreamBalanceAdapterError{code: "recharge_not_configured"}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
