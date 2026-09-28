package provider

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	bepusdtHTTPTimeout       = 15 * time.Second
	bepusdtMaxResponseSize   = 1 << 20
	bepusdtMaxErrorSummary   = 512
	bepusdtDefaultFiat       = "CNY"
	bepusdtDefaultTimeoutSec = 1200
	bepusdtMinTimeoutSec     = 180
	bepusdtMaxTimeoutSec     = 3600
	bepusdtStatusWaiting     = 1
	bepusdtStatusSuccess     = 2
	bepusdtStatusExpired     = 3
	bepusdtStatusCanceled    = 4
	bepusdtStatusConfirming  = 5
)

var bepusdtFiats = map[string]struct{}{
	"CNY": {},
	"USD": {},
	"EUR": {},
	"GBP": {},
	"JPY": {},
}

// BEpusdt implements payment.Provider for the BEpusdt USDT gateway.
// The checkout shows one USDT method. The selected network is sent as trade_type.
type BEpusdt struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

// NewBEpusdt creates a BEpusdt provider.
// Required config: apiBase, apiToken, notifyUrl, returnUrl.
// Optional: fiat (default CNY), timeout seconds (default 1200).
func NewBEpusdt(instanceID string, config map[string]string) (*BEpusdt, error) {
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = strings.TrimSpace(v)
	}
	for _, key := range []string{"apiBase", "apiToken", "notifyUrl", "returnUrl"} {
		if cfg[key] == "" {
			return nil, fmt.Errorf("bepusdt config missing required key: %s", key)
		}
	}
	base, err := normalizeBEpusdtAPIBase(cfg["apiBase"])
	if err != nil {
		return nil, err
	}
	cfg["apiBase"] = base
	fiat := strings.ToUpper(cfg["fiat"])
	if fiat == "" {
		fiat = bepusdtDefaultFiat
	}
	if _, ok := bepusdtFiats[fiat]; !ok {
		return nil, fmt.Errorf("bepusdt fiat must be CNY, USD, EUR, GBP, or JPY")
	}
	cfg["fiat"] = fiat
	timeout := bepusdtDefaultTimeoutSec
	if cfg["timeout"] != "" {
		parsed, err := strconv.Atoi(cfg["timeout"])
		if err != nil || parsed < bepusdtMinTimeoutSec || parsed > bepusdtMaxTimeoutSec {
			return nil, fmt.Errorf("bepusdt timeout must be between %d and %d seconds", bepusdtMinTimeoutSec, bepusdtMaxTimeoutSec)
		}
		timeout = parsed
	}
	cfg["timeout"] = strconv.Itoa(timeout)
	return &BEpusdt{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: bepusdtHTTPTimeout},
	}, nil
}

func normalizeBEpusdtAPIBase(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("bepusdt apiBase must be an http or https URL")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (b *BEpusdt) Name() string        { return "BEpusdt" }
func (b *BEpusdt) ProviderKey() string { return payment.TypeBEpusdt }
func (b *BEpusdt) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeUSDT}
}

func (b *BEpusdt) MerchantIdentityMetadata() map[string]string {
	if b == nil {
		return nil
	}
	return map[string]string{"fiat": b.config["fiat"]}
}

func (b *BEpusdt) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	tradeType, ok := payment.BEpusdtTradeType(req.Network)
	if !ok {
		return nil, fmt.Errorf("bepusdt network must be tron, ethereum, or bsc")
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(req.Amount), 64)
	if err != nil || amount <= 0 {
		return nil, fmt.Errorf("bepusdt amount must be greater than 0")
	}
	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = b.config["notifyUrl"]
	}
	returnURL := strings.TrimSpace(req.ReturnURL)
	if returnURL == "" {
		returnURL = b.config["returnUrl"]
	}
	timeout, _ := strconv.Atoi(b.config["timeout"])
	payload := map[string]any{
		"order_id":     req.OrderID,
		"amount":       amount,
		"notify_url":   notifyURL,
		"redirect_url": returnURL,
		"trade_type":   tradeType,
		"fiat":         b.config["fiat"],
		"timeout":      timeout,
	}
	if name := strings.TrimSpace(req.Subject); name != "" {
		payload["name"] = name
	}
	signature, err := bepusdtSign(payload, b.config["apiToken"])
	if err != nil {
		return nil, err
	}
	payload["signature"] = signature

	var resp struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID    string `json:"trade_id"`
			PaymentURL string `json:"payment_url"`
		} `json:"data"`
	}
	if err := b.post(ctx, "/api/v1/order/create-transaction", payload, &resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(resp.Data.PaymentURL) == "" || strings.TrimSpace(resp.Data.TradeID) == "" {
		if resp.Message == "" {
			resp.Message = "create transaction failed"
		}
		return nil, fmt.Errorf("bepusdt create: %s", resp.Message)
	}
	return &payment.CreatePaymentResponse{
		TradeNo: resp.Data.TradeID,
		PayURL:  resp.Data.PaymentURL,
	}, nil
}

func (b *BEpusdt) QueryOrder(_ context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	// BEpusdt has no merchant order-query API. Payment is confirmed by webhook.
	return &payment.QueryOrderResponse{
		TradeNo: tradeNo,
		Status:  payment.ProviderStatusPending,
	}, nil
}

func (b *BEpusdt) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
		return nil, fmt.Errorf("bepusdt parse notify: %w", err)
	}
	signature, _ := payload["signature"].(string)
	if signature == "" {
		return nil, fmt.Errorf("bepusdt notify missing signature")
	}
	expected, err := bepusdtSign(payload, b.config["apiToken"])
	if err != nil {
		return nil, err
	}
	if !bepusdtSignatureEqual(expected, signature) {
		return nil, fmt.Errorf("bepusdt notify signature mismatch")
	}

	status := bepusdtInt(payload["status"])
	switch status {
	case bepusdtStatusSuccess:
	case bepusdtStatusWaiting, bepusdtStatusExpired, bepusdtStatusCanceled, bepusdtStatusConfirming:
		return nil, nil
	default:
		return nil, nil
	}

	amount, err := bepusdtFloat(payload["amount"])
	if err != nil || amount <= 0 {
		return nil, fmt.Errorf("bepusdt notify amount is invalid")
	}
	orderID, _ := payload["order_id"].(string)
	tradeID, _ := payload["trade_id"].(string)
	if strings.TrimSpace(orderID) == "" {
		return nil, fmt.Errorf("bepusdt notify missing order_id")
	}
	metadata := map[string]string{
		"actual_amount":        bepusdtString(payload["actual_amount"]),
		"token":                bepusdtString(payload["token"]),
		"block_transaction_id": bepusdtString(payload["block_transaction_id"]),
		"fiat":                 b.config["fiat"],
	}
	return &payment.PaymentNotification{
		TradeNo:  tradeID,
		OrderID:  orderID,
		Amount:   amount,
		Status:   payment.NotificationStatusSuccess,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

func (b *BEpusdt) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("bepusdt does not support refunds")
}

func (b *BEpusdt) CancelPayment(ctx context.Context, tradeNo string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return fmt.Errorf("bepusdt cancel missing trade id")
	}
	payload := map[string]any{"trade_id": tradeNo}
	signature, err := bepusdtSign(payload, b.config["apiToken"])
	if err != nil {
		return err
	}
	payload["signature"] = signature
	var resp struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
	}
	if err := b.post(ctx, "/api/v1/order/cancel-transaction", payload, &resp); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		if resp.Message == "" {
			resp.Message = "cancel transaction failed"
		}
		return fmt.Errorf("bepusdt cancel: %s", resp.Message)
	}
	return nil
}

func (b *BEpusdt) post(ctx context.Context, path string, payload any, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bepusdt encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bepusdt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := b.httpClient
	if client == nil {
		client = &http.Client{Timeout: bepusdtHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("bepusdt request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, bepusdtMaxResponseSize))
	if err != nil {
		return fmt.Errorf("bepusdt read: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("bepusdt HTTP %d: %s", resp.StatusCode, summarizeBEpusdtBody(raw))
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("bepusdt parse: %w", err)
	}
	return nil
}

func summarizeBEpusdtBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > bepusdtMaxErrorSummary {
		return text[:bepusdtMaxErrorSummary]
	}
	if text == "" {
		return "empty response"
	}
	return text
}

// bepusdtSign reproduces BEpusdt EpusdtSign: drop empty values and signature,
// sort keys, join key=value with '&', append the token, then lowercase MD5.
// Values are formatted after a JSON round-trip so numbers match the verifier.
func bepusdtSign(payload map[string]any, token string) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("bepusdt sign: %w", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return "", fmt.Errorf("bepusdt sign: %w", err)
	}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		if key == "signature" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for _, key := range keys {
		value := decoded[key]
		if value == nil || value == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		fmt.Fprintf(&buf, "%s=%v", key, value)
	}
	sum := md5.Sum([]byte(buf.String() + token))
	return hex.EncodeToString(sum[:]), nil
}

func bepusdtSignatureEqual(expected, actual string) bool {
	if len(expected) != len(actual) {
		return false
	}
	var diff byte
	for i := 0; i < len(expected); i++ {
		diff |= expected[i] ^ actual[i]
	}
	return diff == 0
}

func bepusdtInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func bepusdtFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case json.Number:
		return typed.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(typed), 64)
	default:
		return 0, fmt.Errorf("unsupported amount type %T", value)
	}
}

func bepusdtString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}
