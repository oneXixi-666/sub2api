package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type upstreamFundsRepositoryStub struct {
	wallets      []UpstreamWallet
	createdInput UpstreamWalletInput
}

func (s *upstreamFundsRepositoryStub) ListWallets(context.Context, string) ([]UpstreamWallet, error) {
	return append([]UpstreamWallet(nil), s.wallets...), nil
}

func (s *upstreamFundsRepositoryStub) GetWallet(context.Context, int64) (*UpstreamWallet, error) {
	return nil, ErrUpstreamWalletNotFound
}

func (s *upstreamFundsRepositoryStub) CreateWallet(_ context.Context, input UpstreamWalletInput) (*UpstreamWallet, error) {
	s.createdInput = input
	return &UpstreamWallet{ID: 1, Name: input.Name, Currency: input.Currency, AlertDays: input.AlertDays, TargetDays: input.TargetDays, AccountIDs: input.AccountIDs}, nil
}

func (s *upstreamFundsRepositoryStub) UpdateWallet(context.Context, int64, UpstreamWalletInput) (*UpstreamWallet, error) {
	return nil, ErrUpstreamWalletNotFound
}

func (s *upstreamFundsRepositoryStub) RecordBalanceSuccess(context.Context, int64, float64, string, string) (*UpstreamWallet, error) {
	return nil, ErrUpstreamWalletNotFound
}

func (s *upstreamFundsRepositoryStub) RecordBalanceFailure(context.Context, int64, string, string) error {
	return nil
}

func (s *upstreamFundsRepositoryStub) ListRechargeProducts(context.Context, int64) ([]UpstreamRechargeProduct, error) {
	return nil, nil
}
func (s *upstreamFundsRepositoryStub) ReplaceRechargeProducts(context.Context, int64, []UpstreamRechargeProduct) ([]UpstreamRechargeProduct, error) {
	return nil, nil
}
func (s *upstreamFundsRepositoryStub) GetRechargeOrder(context.Context, int64) (*UpstreamRechargeOrder, error) {
	return nil, ErrUpstreamRechargeOrderNotFound
}
func (s *upstreamFundsRepositoryStub) GetRechargeOrderByIdempotency(context.Context, int64, string) (*UpstreamRechargeOrder, error) {
	return nil, ErrUpstreamRechargeOrderNotFound
}
func (s *upstreamFundsRepositoryStub) CreateRechargeOrder(context.Context, *UpstreamRechargeOrder, int64) (*UpstreamRechargeOrder, error) {
	return nil, ErrUpstreamRechargeOrderNotFound
}
func (s *upstreamFundsRepositoryStub) UpdateRechargeOrder(context.Context, *UpstreamRechargeOrder, string, string, string, int64) (*UpstreamRechargeOrder, error) {
	return nil, ErrUpstreamRechargeOrderNotFound
}
func (s *upstreamFundsRepositoryStub) CompleteRechargeOrder(context.Context, *UpstreamRechargeOrder, string, string, int64) (*UpstreamRechargeOrder, error) {
	return nil, ErrUpstreamRechargeOrderNotFound
}

func (s *upstreamFundsRepositoryStub) ListAccountOptions(context.Context) ([]UpstreamFundsAccount, error) {
	return nil, nil
}

func TestUpstreamFundsListCalculatesRunwayAndSummary(t *testing.T) {
	healthyBalance := 70.0
	lowBalance := 10.0
	repo := &upstreamFundsRepositoryStub{wallets: []UpstreamWallet{
		{ID: 1, Currency: "USD", Enabled: true, Balance: &healthyBalance, AlertDays: 2, TargetDays: 7, CostToday: 8, Cost24H: 10, Cost7D: 70},
		{ID: 2, Currency: "USD", Enabled: true, Balance: &lowBalance, AlertDays: 2, TargetDays: 7, CostToday: 2, Cost24H: 4, Cost7D: 70},
		{ID: 3, Currency: "CNY", Enabled: false, Balance: &healthyBalance, AlertDays: 2, TargetDays: 7, Cost7D: 70},
	}}

	overview, err := NewUpstreamFundsService(repo, nil, nil).ListWallets(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, 3, overview.Summary.WalletCount)
	require.Equal(t, 2, overview.Summary.EnabledCount)
	require.Equal(t, 1, overview.Summary.AttentionCount)
	require.InDelta(t, 10, overview.Summary.CostToday, 0.000001)
	require.InDelta(t, 80, overview.Summary.BalanceByCurrency["USD"], 0.000001)
	require.InDelta(t, 70, overview.Summary.BalanceByCurrency["CNY"], 0.000001)
	require.NotNil(t, overview.Wallets[0].RunwayDays)
	require.InDelta(t, 7, *overview.Wallets[0].RunwayDays, 0.000001)
	require.False(t, overview.Wallets[0].NeedsAttention)
	require.InDelta(t, 60, *overview.Wallets[1].RecommendedTopUp, 0.000001)
	require.True(t, overview.Wallets[1].NeedsAttention)
	require.Nil(t, overview.Wallets[2].RunwayDays)
}

func TestUpstreamFundsCreateNormalizesAndDeduplicatesAccounts(t *testing.T) {
	repo := &upstreamFundsRepositoryStub{}
	_, err := NewUpstreamFundsService(repo, nil, nil).CreateWallet(context.Background(), UpstreamWalletInput{
		Name:         "  Primary wallet  ",
		Provider:     " Provider_A ",
		Currency:     "usd",
		RechargeMode: "MANUAL",
		CardSiteURL:  "https://cards.example.com/buy",
		Tier:         "PRIMARY",
		Enabled:      true,
		AlertDays:    2,
		TargetDays:   7,
		AccountIDs:   []int64{9, 3, 9},
	})
	require.NoError(t, err)
	require.Equal(t, "Primary wallet", repo.createdInput.Name)
	require.Equal(t, "provider_a", repo.createdInput.Provider)
	require.Equal(t, "USD", repo.createdInput.Currency)
	require.Equal(t, "https://cards.example.com/buy", repo.createdInput.CardSiteURL)
	require.Equal(t, []int64{3, 9}, repo.createdInput.AccountIDs)
}

func TestUpstreamFundsCreateRejectsInvalidReserveAndCurrency(t *testing.T) {
	svc := NewUpstreamFundsService(&upstreamFundsRepositoryStub{}, nil, nil)
	base := UpstreamWalletInput{Name: "wallet", Provider: "provider", Currency: "USD", RechargeMode: "manual", Tier: "primary", AlertDays: 3, TargetDays: 2}

	_, err := svc.CreateWallet(context.Background(), base)
	require.Error(t, err)

	base.AlertDays = 2
	base.TargetDays = 7
	base.Currency = "USDT"
	_, err = svc.CreateWallet(context.Background(), base)
	require.Error(t, err)
}

func TestUpstreamFundsCreateRejectsUnsafeCardSiteURL(t *testing.T) {
	svc := NewUpstreamFundsService(&upstreamFundsRepositoryStub{}, nil, nil)
	_, err := svc.CreateWallet(context.Background(), UpstreamWalletInput{
		Name:         "voucher wallet",
		Provider:     "provider",
		Currency:     "USD",
		RechargeMode: "product",
		CardSiteURL:  "https://user:password@cards.example.com/buy",
		Tier:         "primary",
		AlertDays:    2,
		TargetDays:   7,
	})
	require.Error(t, err)
}

func TestMapProviderRechargeStatusFailsClosed(t *testing.T) {
	require.Equal(t, UpstreamRechargeStatusPendingPayment, mapProviderRechargeStatus("PENDING"))
	require.Equal(t, UpstreamRechargeStatusPaid, mapProviderRechargeStatus("RECHARGING"))
	require.Equal(t, UpstreamRechargeStatusVerifying, mapProviderRechargeStatus("COMPLETED"))
	require.Equal(t, UpstreamRechargeStatusManualReview, mapProviderRechargeStatus("unexpected-provider-state"))
}
