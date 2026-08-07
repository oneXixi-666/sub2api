package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamFundsRefreshRepo struct {
	*upstreamFundsRepositoryStub
	mu           sync.Mutex
	wallets      map[int64]UpstreamWallet
	successCount int
	failureCount int
}

func newUpstreamFundsRefreshRepo(wallets ...UpstreamWallet) *upstreamFundsRefreshRepo {
	items := make(map[int64]UpstreamWallet, len(wallets))
	for _, wallet := range wallets {
		items[wallet.ID] = wallet
	}
	return &upstreamFundsRefreshRepo{upstreamFundsRepositoryStub: &upstreamFundsRepositoryStub{}, wallets: items}
}

func (r *upstreamFundsRefreshRepo) GetWallet(_ context.Context, id int64) (*UpstreamWallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wallet, ok := r.wallets[id]
	if !ok {
		return nil, ErrUpstreamWalletNotFound
	}
	return &wallet, nil
}

func (r *upstreamFundsRefreshRepo) RecordBalanceSuccess(_ context.Context, id int64, balance float64, _, _ string) (*UpstreamWallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wallet := r.wallets[id]
	wallet.Balance = &balance
	r.wallets[id] = wallet
	r.successCount++
	return &wallet, nil
}

func (r *upstreamFundsRefreshRepo) RecordBalanceFailure(_ context.Context, id int64, summary, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	wallet := r.wallets[id]
	wallet.BalanceError = summary
	r.wallets[id] = wallet
	r.failureCount++
	return nil
}

type upstreamBalanceProviderStub struct {
	calls   atomic.Int32
	entered chan int64
	release <-chan struct{}
	err     error
}

func (p *upstreamBalanceProviderStub) BalanceConfigured(*UpstreamWallet, []*Account) bool {
	return true
}

func (p *upstreamBalanceProviderStub) RefreshBalance(_ context.Context, wallet *UpstreamWallet, _ []*Account) (*BalanceSnapshot, error) {
	p.calls.Add(1)
	if p.entered != nil {
		p.entered <- wallet.ID
	}
	if p.release != nil {
		<-p.release
	}
	if p.err != nil {
		return nil, p.err
	}
	return &BalanceSnapshot{Balance: float64(wallet.ID) * 10, Currency: wallet.Currency}, nil
}

func TestUpstreamFundsRefreshBalanceCoalescesSameWallet(t *testing.T) {
	release := make(chan struct{})
	provider := &upstreamBalanceProviderStub{entered: make(chan int64, 2), release: release}
	repo := newUpstreamFundsRefreshRepo(UpstreamWallet{ID: 1, Currency: "USD"})
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.balanceProvider = provider

	results := make(chan error, 2)
	go func() { _, err := svc.RefreshBalance(context.Background(), 1); results <- err }()
	require.Equal(t, int64(1), <-provider.entered)
	go func() { _, err := svc.RefreshBalance(context.Background(), 1); results <- err }()
	time.Sleep(20 * time.Millisecond)
	close(release)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	require.Equal(t, int32(1), provider.calls.Load())
	require.Equal(t, 1, repo.successCount)
}

func TestUpstreamFundsRefreshBalanceAllowsDifferentWalletsConcurrently(t *testing.T) {
	release := make(chan struct{})
	provider := &upstreamBalanceProviderStub{entered: make(chan int64, 2), release: release}
	repo := newUpstreamFundsRefreshRepo(
		UpstreamWallet{ID: 1, Currency: "USD"},
		UpstreamWallet{ID: 2, Currency: "USD"},
	)
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.balanceProvider = provider

	results := make(chan error, 2)
	go func() { _, err := svc.RefreshBalance(context.Background(), 1); results <- err }()
	go func() { _, err := svc.RefreshBalance(context.Background(), 2); results <- err }()
	seen := map[int64]bool{}
	for range 2 {
		select {
		case id := <-provider.entered:
			seen[id] = true
		case <-time.After(time.Second):
			t.Fatal("different wallets did not enter the provider concurrently")
		}
	}
	close(release)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	require.Equal(t, map[int64]bool{1: true, 2: true}, seen)
	require.Equal(t, int32(2), provider.calls.Load())
}

func TestUpstreamFundsRefreshFailurePreservesLastValidBalance(t *testing.T) {
	balance := 88.5
	repo := newUpstreamFundsRefreshRepo(UpstreamWallet{ID: 1, Currency: "USD", Balance: &balance})
	svc := NewUpstreamFundsService(repo, nil, nil)
	svc.balanceProvider = &upstreamBalanceProviderStub{err: errors.New("network details must not be persisted")}

	_, err := svc.RefreshBalance(context.Background(), 1)
	require.ErrorIs(t, err, ErrUpstreamBalanceRefreshUnavailable)
	stored, getErr := repo.GetWallet(context.Background(), 1)
	require.NoError(t, getErr)
	require.NotNil(t, stored.Balance)
	require.InDelta(t, 88.5, *stored.Balance, 0.000001)
	require.Equal(t, "refresh_failed", stored.BalanceError)
	require.Equal(t, 0, repo.successCount)
	require.Equal(t, 1, repo.failureCount)
}

func TestParseSub2APIUsageBalanceUsesCCSwitchPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		balance  float64
		currency string
	}{
		{name: "remaining first", body: `{"remaining":12.5,"quota":{"remaining":8,"unit":"EUR"},"balance":3,"unit":"USD"}`, balance: 12.5, currency: "USD"},
		{name: "quota remaining second", body: `{"quota":{"remaining":8,"unit":"EUR"},"balance":3}`, balance: 8, currency: "EUR"},
		{name: "balance fallback", body: `{"balance":"3.25"}`, balance: 3.25, currency: "USD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			balance, currency, err := parseSub2APIUsageBalance([]byte(test.body), "USD")
			require.NoError(t, err)
			require.InDelta(t, test.balance, balance, 0.000001)
			require.Equal(t, test.currency, currency)
		})
	}
}

func TestValidateSub2APIRedeemResponseRequiresBusinessSuccess(t *testing.T) {
	require.NoError(t, validateSub2APIRedeemResponse([]byte(`{"code":0,"message":"ok","data":null}`)))

	err := validateSub2APIRedeemResponse([]byte(`{"code":4001,"message":"invalid code","data":null}`))
	require.Equal(t, "redeem_rejected", upstreamBalanceErrorSummary(err))

	err = validateSub2APIRedeemResponse([]byte(`not-json`))
	require.Equal(t, "redeem_invalid_response", upstreamBalanceErrorSummary(err))
}
