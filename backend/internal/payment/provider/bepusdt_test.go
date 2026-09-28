//go:build unit

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func testBEpusdtConfig(apiBase string) map[string]string {
	return map[string]string{
		"apiBase":   apiBase,
		"apiToken":  "epusdt_password_xasddawqe",
		"notifyUrl": "https://merchant.example.com/api/v1/payment/webhook/bepusdt",
		"returnUrl": "https://merchant.example.com/payment/result",
		"fiat":      "CNY",
		"timeout":   "1200",
	}
}

func TestBEpusdtSignMatchesDocumentedVector(t *testing.T) {
	t.Parallel()
	signature, err := bepusdtSign(map[string]any{
		"order_id":     "20220201030210321",
		"amount":       42,
		"notify_url":   "http://example.com/notify",
		"redirect_url": "http://example.com/redirect",
	}, "epusdt_password_xasddawqe")
	require.NoError(t, err)
	require.Equal(t, "1cd4b52df5587cfb1968b0c0c6e156cd", signature)
}

func TestNewBEpusdtRejectsIncompleteConfig(t *testing.T) {
	t.Parallel()
	_, err := NewBEpusdt("1", map[string]string{"apiBase": "https://pay.example.com"})
	require.ErrorContains(t, err, "apiToken")
	_, err = NewBEpusdt("1", map[string]string{
		"apiBase":   "ftp://pay.example.com",
		"apiToken":  "token",
		"notifyUrl": "https://merchant.example.com/notify",
		"returnUrl": "https://merchant.example.com/result",
	})
	require.ErrorContains(t, err, "apiBase")
}

func TestBEpusdtCreatePaymentUsesSelectedNetwork(t *testing.T) {
	t.Parallel()
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/order/create-transaction", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"trade-1","payment_url":"https://pay.example.com/pay/checkout-counter/trade-1"}}`))
	}))
	defer server.Close()

	prov, err := NewBEpusdt("9", testBEpusdtConfig(server.URL))
	require.NoError(t, err)
	require.Equal(t, []payment.PaymentType{payment.TypeUSDT}, prov.SupportedTypes())

	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "order-1",
		Amount:  "28.88",
		Network: payment.USDTNetworkEthereum,
		Subject: "balance",
	})
	require.NoError(t, err)
	require.Equal(t, "trade-1", resp.TradeNo)
	require.Equal(t, "https://pay.example.com/pay/checkout-counter/trade-1", resp.PayURL)
	require.Equal(t, "usdt.erc20", got["trade_type"])
	require.Equal(t, "order-1", got["order_id"])

	signature, err := bepusdtSign(got, "epusdt_password_xasddawqe")
	require.NoError(t, err)
	require.Equal(t, signature, got["signature"])
}

func TestBEpusdtVerifyNotificationCreditsFiatAmount(t *testing.T) {
	t.Parallel()
	prov, err := NewBEpusdt("9", testBEpusdtConfig("https://pay.example.com"))
	require.NoError(t, err)

	payload := map[string]any{
		"trade_id":             "trade-1",
		"order_id":             "order-1",
		"amount":               28.88,
		"actual_amount":        "4.01",
		"token":                "TAddress",
		"block_transaction_id": "hash",
		"status":               bepusdtStatusSuccess,
	}
	signature, err := bepusdtSign(payload, "epusdt_password_xasddawqe")
	require.NoError(t, err)
	payload["signature"] = signature
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	notice, err := prov.VerifyNotification(context.Background(), string(raw), nil)
	require.NoError(t, err)
	require.Equal(t, "order-1", notice.OrderID)
	require.Equal(t, "trade-1", notice.TradeNo)
	require.InDelta(t, 28.88, notice.Amount, 0.000001)
	require.Equal(t, payment.NotificationStatusSuccess, notice.Status)
	require.Equal(t, "4.01", notice.Metadata["actual_amount"])

	payload["status"] = bepusdtStatusWaiting
	delete(payload, "signature")
	signature, err = bepusdtSign(payload, "epusdt_password_xasddawqe")
	require.NoError(t, err)
	payload["signature"] = signature
	raw, err = json.Marshal(payload)
	require.NoError(t, err)
	notice, err = prov.VerifyNotification(context.Background(), string(raw), nil)
	require.NoError(t, err)
	require.Nil(t, notice)

	payload["signature"] = "deadbeef"
	raw, err = json.Marshal(payload)
	require.NoError(t, err)
	_, err = prov.VerifyNotification(context.Background(), string(raw), nil)
	require.ErrorContains(t, err, "signature")
}

func TestBEpusdtCancelPayment(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/order/cancel-transaction", r.URL.Path)
		var got map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		require.Equal(t, "trade-1", got["trade_id"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"trade-1"}}`))
	}))
	defer server.Close()

	prov, err := NewBEpusdt("9", testBEpusdtConfig(server.URL))
	require.NoError(t, err)
	require.NoError(t, prov.CancelPayment(context.Background(), "trade-1"))
	_, err = prov.Refund(context.Background(), payment.RefundRequest{OrderID: "order-1"})
	require.ErrorContains(t, err, "does not support refunds")
}
