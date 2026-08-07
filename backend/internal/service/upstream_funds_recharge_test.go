package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type upstreamRechargeRepoStub struct {
	*upstreamFundsRepositoryStub
	wallet               UpstreamWallet
	order                *UpstreamRechargeOrder
	completeCalled       bool
	completeFromStatus   string
	completeReason       string
	completeBalanceAfter float64
}

func newUpstreamRechargeRepoStub() *upstreamRechargeRepoStub {
	balance := 25.0
	return &upstreamRechargeRepoStub{
		upstreamFundsRepositoryStub: &upstreamFundsRepositoryStub{},
		wallet:                      UpstreamWallet{ID: 9, Name: "provider wallet", Currency: "USD", RechargeMode: "direct", Balance: &balance},
	}
}

func (r *upstreamRechargeRepoStub) GetWallet(_ context.Context, id int64) (*UpstreamWallet, error) {
	if id != r.wallet.ID {
		return nil, ErrUpstreamWalletNotFound
	}
	wallet := r.wallet
	return &wallet, nil
}

func (r *upstreamRechargeRepoStub) GetRechargeOrder(_ context.Context, id int64) (*UpstreamRechargeOrder, error) {
	if r.order == nil || r.order.ID != id {
		return nil, ErrUpstreamRechargeOrderNotFound
	}
	order := *r.order
	return &order, nil
}

func (r *upstreamRechargeRepoStub) GetRechargeOrderByIdempotency(_ context.Context, walletID int64, key string) (*UpstreamRechargeOrder, error) {
	if r.order != nil && r.order.WalletID == walletID && r.order.IdempotencyKey == key {
		order := *r.order
		return &order, nil
	}
	return nil, ErrUpstreamRechargeOrderNotFound
}

func (r *upstreamRechargeRepoStub) CreateRechargeOrder(_ context.Context, order *UpstreamRechargeOrder, _ int64) (*UpstreamRechargeOrder, error) {
	stored := *order
	stored.ID = 71
	stored.CreatedAt = time.Now().UTC()
	stored.UpdatedAt = stored.CreatedAt
	r.order = &stored
	result := stored
	return &result, nil
}

func (r *upstreamRechargeRepoStub) UpdateRechargeOrder(_ context.Context, order *UpstreamRechargeOrder, fromStatus, _, _ string, _ int64) (*UpstreamRechargeOrder, error) {
	if r.order == nil || r.order.Status != fromStatus {
		return nil, ErrUpstreamRechargeConflict
	}
	stored := *order
	stored.UpdatedAt = time.Now().UTC()
	r.order = &stored
	result := stored
	return &result, nil
}

func (r *upstreamRechargeRepoStub) CompleteRechargeOrder(_ context.Context, order *UpstreamRechargeOrder, fromStatus, reason string, _ int64) (*UpstreamRechargeOrder, error) {
	if r.order == nil || r.order.Status != fromStatus || order.BalanceAfter == nil {
		return nil, ErrUpstreamRechargeConflict
	}
	r.completeCalled = true
	r.completeFromStatus = fromStatus
	r.completeReason = reason
	r.completeBalanceAfter = *order.BalanceAfter
	stored := *order
	stored.Status = UpstreamRechargeStatusCompleted
	stored.ErrorCode = ""
	stored.ErrorMessage = ""
	now := time.Now().UTC()
	stored.CompletedAt = &now
	r.order = &stored
	result := stored
	return &result, nil
}

type upstreamRechargeProviderStub struct {
	channels   []UpstreamPaymentChannel
	create     *UpstreamProviderOrderUpdate
	createErr  error
	query      *UpstreamProviderOrderUpdate
	queryErr   error
	queryCalls int
}

func (p *upstreamRechargeProviderStub) RechargeConfigured(*UpstreamWallet, []*Account) bool {
	return true
}
func (p *upstreamRechargeProviderStub) ListPaymentChannels(context.Context, *UpstreamWallet, []*Account) ([]UpstreamPaymentChannel, error) {
	return append([]UpstreamPaymentChannel(nil), p.channels...), nil
}
func (p *upstreamRechargeProviderStub) CreateRechargeOrder(context.Context, *UpstreamWallet, []*Account, float64, string) (*UpstreamProviderOrderUpdate, error) {
	return p.create, p.createErr
}
func (p *upstreamRechargeProviderStub) QueryRechargeOrder(context.Context, *UpstreamWallet, []*Account, string) (*UpstreamProviderOrderUpdate, error) {
	p.queryCalls++
	return p.query, p.queryErr
}

func validRechargeChannel() UpstreamPaymentChannel {
	return UpstreamPaymentChannel{ID: "alipay", Name: "Alipay", Currency: "CNY", SingleMin: 1, SingleMax: 5000, DailyRemaining: 10000}
}

func createRechargeForTest(t *testing.T, provider *upstreamRechargeProviderStub) (*UpstreamRechargeOrder, *upstreamRechargeRepoStub) {
	t.Helper()
	repo := newUpstreamRechargeRepoStub()
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.rechargeProvider = provider
	order, err := svc.CreateRechargeOrder(context.Background(), repo.wallet.ID, UpstreamRechargeOrderInput{
		Amount: 100, PaymentChannelID: "alipay", IdempotencyKey: "test-key",
	}, 3)
	require.NoError(t, err)
	return order, repo
}

func TestCreateRechargeOrderUsesProviderInitialStatusAndPaymentCurrency(t *testing.T) {
	provider := &upstreamRechargeProviderStub{
		channels: []UpstreamPaymentChannel{validRechargeChannel()},
		create: &UpstreamProviderOrderUpdate{
			ProviderOrderID: "provider-1", Status: "completed", PayAmount: 98,
			Currency: "cny", PaymentURL: "https://pay.example.com/order/1",
		},
	}
	order, _ := createRechargeForTest(t, provider)
	require.Equal(t, UpstreamRechargeStatusVerifying, order.Status)
	require.Equal(t, "CNY", order.Currency)
	require.Equal(t, "https://pay.example.com/order/1", order.PaymentURL)
}

func TestCreateRechargeOrderFailsClosedForUnsafeProviderFields(t *testing.T) {
	tests := []struct {
		name   string
		update UpstreamProviderOrderUpdate
	}{
		{
			name:   "unsafe payment URL",
			update: UpstreamProviderOrderUpdate{ProviderOrderID: "provider-1", Status: "pending", PayAmount: 98, Currency: "CNY", PaymentURL: "javascript:alert(1)"},
		},
		{
			name:   "currency differs from channel",
			update: UpstreamProviderOrderUpdate{ProviderOrderID: "provider-1", Status: "pending", PayAmount: 98, Currency: "USD", PaymentQR: "qr"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &upstreamRechargeProviderStub{channels: []UpstreamPaymentChannel{validRechargeChannel()}, create: &test.update}
			order, _ := createRechargeForTest(t, provider)
			require.Equal(t, UpstreamRechargeStatusManualReview, order.Status)
			require.Equal(t, "invalid_provider_order", order.ErrorCode)
			require.Equal(t, "provider-1", order.ProviderOrderID)
			require.Empty(t, order.PaymentURL)
		})
	}
}

func TestCreateRechargeOrderRejectsInvalidNumericInputsAndChannels(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.rechargeProvider = &upstreamRechargeProviderStub{channels: []UpstreamPaymentChannel{validRechargeChannel()}}

	_, err := svc.CreateRechargeOrder(context.Background(), repo.wallet.ID, UpstreamRechargeOrderInput{
		Amount: math.Inf(1), PaymentChannelID: "alipay", IdempotencyKey: "infinite",
	}, 3)
	require.Error(t, err)

	svc.rechargeProvider = &upstreamRechargeProviderStub{channels: []UpstreamPaymentChannel{{
		ID: "bad", Name: "Bad", Currency: "CNY", SingleMin: math.NaN(),
	}}}
	_, err = svc.ListPaymentChannels(context.Background(), repo.wallet.ID)
	require.ErrorIs(t, err, ErrUpstreamRechargeUnavailable)
}

func TestCreateRechargeOrderUnknownStatusRequiresManualReview(t *testing.T) {
	provider := &upstreamRechargeProviderStub{
		channels: []UpstreamPaymentChannel{validRechargeChannel()},
		create:   &UpstreamProviderOrderUpdate{ProviderOrderID: "provider-1", Status: "mystery", PayAmount: 98, Currency: "CNY", PaymentQR: "qr"},
	}
	order, _ := createRechargeForTest(t, provider)
	require.Equal(t, UpstreamRechargeStatusManualReview, order.Status)
	require.Equal(t, "unknown_provider_status", order.ErrorCode)
}

func TestPollRechargeOrderMovesInterruptedCreatingToManualReview(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	repo.order = &UpstreamRechargeOrder{
		ID: 71, WalletID: repo.wallet.ID, Status: UpstreamRechargeStatusCreating,
		UpdatedAt: time.Now().Add(-upstreamRechargeCreatingStaleAfter - time.Second),
	}
	svc := NewUpstreamFundsService(repo, nil, nil)
	order, err := svc.PollRechargeOrder(context.Background(), repo.order.ID, 3)
	require.NoError(t, err)
	require.Equal(t, UpstreamRechargeStatusManualReview, order.Status)
	require.Equal(t, "create_interrupted", order.ErrorCode)
}

func TestPollRechargeOrderRetriesTransientProviderFailureWithoutChangingState(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	repo.order = &UpstreamRechargeOrder{
		ID: 71, WalletID: repo.wallet.ID, ProviderOrderID: "provider-1", PaymentChannelID: "alipay",
		Currency: "CNY", Status: UpstreamRechargeStatusPendingPayment, UpdatedAt: time.Now(),
	}
	provider := &upstreamRechargeProviderStub{queryErr: errors.New("temporary")}
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.rechargeProvider = provider
	_, err := svc.PollRechargeOrder(context.Background(), repo.order.ID, 3)
	require.ErrorIs(t, err, ErrUpstreamRechargeUnavailable)
	require.Equal(t, UpstreamRechargeStatusPendingPayment, repo.order.Status)
}

func TestPollRechargeOrderRejectsProviderStateRegression(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	repo.order = &UpstreamRechargeOrder{
		ID: 71, WalletID: repo.wallet.ID, ProviderOrderID: "provider-1", PaymentChannelID: "alipay",
		Currency: "CNY", PayAmount: 98, Status: UpstreamRechargeStatusPaid, UpdatedAt: time.Now(),
	}
	provider := &upstreamRechargeProviderStub{query: &UpstreamProviderOrderUpdate{
		ProviderOrderID: "provider-1", Status: "pending", PayAmount: 98, Currency: "CNY",
	}}
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.rechargeProvider = provider
	order, err := svc.PollRechargeOrder(context.Background(), repo.order.ID, 3)
	require.NoError(t, err)
	require.Equal(t, UpstreamRechargeStatusManualReview, order.Status)
	require.Equal(t, "invalid_provider_transition", order.ErrorCode)
}

func TestPollRechargeOrderRejectsProviderOrderReferenceChange(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	repo.order = &UpstreamRechargeOrder{
		ID: 71, WalletID: repo.wallet.ID, ProviderOrderID: "provider-1", PaymentChannelID: "alipay",
		Currency: "CNY", PayAmount: 98, Status: UpstreamRechargeStatusPendingPayment, UpdatedAt: time.Now(),
	}
	provider := &upstreamRechargeProviderStub{query: &UpstreamProviderOrderUpdate{
		ProviderOrderID: "provider-2", Status: "paid", PayAmount: 98, Currency: "CNY",
	}}
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.rechargeProvider = provider
	order, err := svc.PollRechargeOrder(context.Background(), repo.order.ID, 3)
	require.NoError(t, err)
	require.Equal(t, UpstreamRechargeStatusManualReview, order.Status)
	require.Equal(t, "invalid_provider_order", order.ErrorCode)
	require.Equal(t, "provider-1", order.ProviderOrderID)
}

func TestManualCompleteRechargeOrderDelegatesAtomicCompletion(t *testing.T) {
	repo := newUpstreamRechargeRepoStub()
	repo.order = &UpstreamRechargeOrder{ID: 71, WalletID: repo.wallet.ID, Status: UpstreamRechargeStatusManualReview}
	svc := NewUpstreamFundsService(repo, nil, nil)
	order, err := svc.ManualCompleteRechargeOrder(context.Background(), repo.order.ID, 125.5, "verified in provider panel", 3)
	require.NoError(t, err)
	require.Equal(t, UpstreamRechargeStatusCompleted, order.Status)
	require.True(t, repo.completeCalled)
	require.Equal(t, UpstreamRechargeStatusManualReview, repo.completeFromStatus)
	require.Equal(t, "verified in provider panel", repo.completeReason)
	require.InDelta(t, 125.5, repo.completeBalanceAfter, 0.000001)
}

func TestUpstreamRechargeAdapterExposesOnlyFixedAlipayChannel(t *testing.T) {
	upstream := &httpUpstreamRecorder{}
	provider := &sub2APIUsageBalanceProvider{accountTestService: &AccountTestService{httpUpstream: upstream}}
	wallet := &UpstreamWallet{Currency: "USD"}
	account := &Account{Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://panel.example.com/v1", "api_key": "api-key", "upstream_funds_panel_token": "panel-token",
	}}

	channels, err := provider.ListPaymentChannels(context.Background(), wallet, []*Account{account})
	require.NoError(t, err)
	require.Equal(t, []UpstreamPaymentChannel{{ID: "alipay", Name: "支付宝", Currency: "USD"}}, channels)
	require.Empty(t, upstream.requests, "fixed channel discovery must not depend on checkout-info")
}

func TestUpstreamRechargeAdapterAlwaysCreatesAlipayOrder(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
		`{"code":0,"data":{"order_id":9,"pay_amount":100,"status":"pending","payment_type":"alipay","pay_url":"https://pay.example.com/9","currency":"USD"}}`,
	))}}
	provider := &sub2APIUsageBalanceProvider{accountTestService: &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}}
	wallet := &UpstreamWallet{Currency: "USD"}
	account := &Account{ID: 7, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
		"base_url": "https://panel.example.com/v1", "api_key": "api-key", "upstream_funds_panel_token": "panel-token",
	}}

	order, err := provider.CreateRechargeOrder(context.Background(), wallet, []*Account{account}, 100, "wxpay")
	require.NoError(t, err)
	require.Equal(t, "9", order.ProviderOrderID)
	require.Len(t, upstream.bodies, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.bodies[0], &payload))
	require.Equal(t, "alipay", payload["payment_type"])
}
