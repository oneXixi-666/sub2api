package service

import (
	"context"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

var (
	ErrUpstreamWalletNotFound  = infraerrors.NotFound("UPSTREAM_WALLET_NOT_FOUND", "upstream wallet not found")
	ErrUpstreamAccountAssigned = infraerrors.Conflict(
		"UPSTREAM_ACCOUNT_ALREADY_ASSIGNED",
		"one or more accounts already belong to another upstream wallet",
	)
	ErrUpstreamAccountNotFound = infraerrors.BadRequest(
		"UPSTREAM_ACCOUNT_NOT_FOUND",
		"one or more selected accounts do not exist",
	)
	ErrUpstreamBalanceRefreshUnavailable = infraerrors.ServiceUnavailable(
		"UPSTREAM_BALANCE_REFRESH_UNAVAILABLE",
		"upstream balance refresh is unavailable",
	)
	ErrUpstreamRedeemUnavailable = infraerrors.ServiceUnavailable(
		"UPSTREAM_REDEEM_UNAVAILABLE",
		"upstream redeem adapter is unavailable",
	)
)

type UpstreamFundsAccount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type,omitempty"`
	WalletID *int64 `json:"wallet_id,omitempty"`
	Wallet   string `json:"wallet_name,omitempty"`
}

type UpstreamFundsGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type UpstreamWallet struct {
	ID                     int64                     `json:"id"`
	Name                   string                    `json:"name"`
	Provider               string                    `json:"provider"`
	Currency               string                    `json:"currency"`
	CostCurrency           string                    `json:"cost_currency"`
	RechargeMode           string                    `json:"recharge_mode"`
	CardSiteURL            string                    `json:"card_site_url"`
	Tier                   string                    `json:"tier"`
	Enabled                bool                      `json:"enabled"`
	Balance                *float64                  `json:"balance"`
	BalanceUpdatedAt       *time.Time                `json:"balance_updated_at"`
	BalanceError           string                    `json:"balance_error"`
	AlertDays              int                       `json:"alert_days"`
	TargetDays             int                       `json:"target_days"`
	AccountIDs             []int64                   `json:"account_ids"`
	Accounts               []UpstreamFundsAccount    `json:"accounts"`
	ConfiguredGroups       []UpstreamFundsGroup      `json:"configured_groups"`
	ActualGroups           []UpstreamFundsGroup      `json:"actual_groups"`
	Cost1H                 float64                   `json:"cost_1h"`
	CostToday              float64                   `json:"cost_today"`
	Cost24H                float64                   `json:"cost_24h"`
	Cost7D                 float64                   `json:"cost_7d"`
	DailyCost7D            float64                   `json:"daily_cost_7d"`
	RunwayDays             *float64                  `json:"runway_days"`
	RecommendedTopUp       *float64                  `json:"recommended_top_up"`
	NeedsAttention         bool                      `json:"needs_attention"`
	AdapterConfigured      bool                      `json:"adapter_configured"`
	RedeemConfigured       bool                      `json:"redeem_configured"`
	RechargeConfigured     bool                      `json:"recharge_configured"`
	PanelSession           UpstreamPanelSessionState `json:"panel_session"`
	PanelSessionCiphertext string                    `json:"-"`
	CreatedAt              time.Time                 `json:"created_at"`
	UpdatedAt              time.Time                 `json:"updated_at"`
}

type UpstreamWalletInput struct {
	Name         string
	Provider     string
	Currency     string
	RechargeMode string
	CardSiteURL  string
	Tier         string
	Enabled      bool
	AlertDays    int
	TargetDays   int
	AccountIDs   []int64
}

type UpstreamFundsSummary struct {
	WalletCount       int                `json:"wallet_count"`
	EnabledCount      int                `json:"enabled_count"`
	AttentionCount    int                `json:"attention_count"`
	CostToday         float64            `json:"cost_today"`
	Cost24H           float64            `json:"cost_24h"`
	BalanceByCurrency map[string]float64 `json:"balance_by_currency"`
}

type UpstreamFundsOverview struct {
	Summary UpstreamFundsSummary `json:"summary"`
	Wallets []UpstreamWallet     `json:"wallets"`
}

type UpstreamRedeemResult struct {
	Status string          `json:"status"`
	Wallet *UpstreamWallet `json:"wallet"`
}

type UpstreamFundsRepository interface {
	ListWallets(ctx context.Context, search string) ([]UpstreamWallet, error)
	GetWallet(ctx context.Context, id int64) (*UpstreamWallet, error)
	CreateWallet(ctx context.Context, input UpstreamWalletInput) (*UpstreamWallet, error)
	UpdateWallet(ctx context.Context, id int64, input UpstreamWalletInput) (*UpstreamWallet, error)
	RecordBalanceSuccess(ctx context.Context, id int64, balance float64, currency, source string) (*UpstreamWallet, error)
	RecordBalanceFailure(ctx context.Context, id int64, errorSummary, source string) error
	ListRechargeProducts(ctx context.Context, walletID int64) ([]UpstreamRechargeProduct, error)
	ReplaceRechargeProducts(ctx context.Context, walletID int64, products []UpstreamRechargeProduct) ([]UpstreamRechargeProduct, error)
	GetRechargeOrder(ctx context.Context, id int64) (*UpstreamRechargeOrder, error)
	GetRechargeOrderByIdempotency(ctx context.Context, walletID int64, idempotencyKey string) (*UpstreamRechargeOrder, error)
	CreateRechargeOrder(ctx context.Context, order *UpstreamRechargeOrder, actorID int64) (*UpstreamRechargeOrder, error)
	UpdateRechargeOrder(ctx context.Context, order *UpstreamRechargeOrder, fromStatus, eventType, summary string, actorID int64) (*UpstreamRechargeOrder, error)
	CompleteRechargeOrder(ctx context.Context, order *UpstreamRechargeOrder, fromStatus, reason string, actorID int64) (*UpstreamRechargeOrder, error)
	ListAccountOptions(ctx context.Context) ([]UpstreamFundsAccount, error)
}

type UpstreamFundsService struct {
	repo             UpstreamFundsRepository
	accountRepo      AccountRepository
	balanceProvider  UpstreamBalanceProvider
	codeRedeemer     UpstreamCodeRedeemer
	rechargeProvider UpstreamRechargeProvider
	refreshGroup     singleflight.Group
	orderPollGroup   singleflight.Group
	panelSessions    *upstreamPanelSessionRuntime
}

func NewUpstreamFundsService(
	repo UpstreamFundsRepository,
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
) *UpstreamFundsService {
	provider := NewSub2APIUsageBalanceProvider(accountTestService)
	svc := &UpstreamFundsService{
		repo:             repo,
		accountRepo:      accountRepo,
		balanceProvider:  provider,
		codeRedeemer:     provider,
		rechargeProvider: provider,
	}
	provider.panelSessions = svc
	return svc
}

func (s *UpstreamFundsService) ListWallets(ctx context.Context, search string) (*UpstreamFundsOverview, error) {
	wallets, err := s.repo.ListWallets(ctx, strings.TrimSpace(search))
	if err != nil {
		return nil, err
	}
	if err := s.attachAdapterCapabilities(ctx, wallets); err != nil {
		return nil, err
	}

	summary := UpstreamFundsSummary{BalanceByCurrency: map[string]float64{}}
	for i := range wallets {
		calculateUpstreamWalletMetrics(&wallets[i])
		wallet := &wallets[i]
		summary.WalletCount++
		if wallet.Enabled {
			summary.EnabledCount++
		}
		if wallet.NeedsAttention {
			summary.AttentionCount++
		}
		summary.CostToday += wallet.CostToday
		summary.Cost24H += wallet.Cost24H
		if wallet.Balance != nil {
			summary.BalanceByCurrency[wallet.Currency] += *wallet.Balance
		}
	}
	if wallets == nil {
		wallets = []UpstreamWallet{}
	}
	return &UpstreamFundsOverview{Summary: summary, Wallets: wallets}, nil
}

func (s *UpstreamFundsService) GetWallet(ctx context.Context, id int64) (*UpstreamWallet, error) {
	wallet, err := s.repo.GetWallet(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichWallet(ctx, wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) CreateWallet(ctx context.Context, input UpstreamWalletInput) (*UpstreamWallet, error) {
	if err := normalizeUpstreamWalletInput(&input); err != nil {
		return nil, err
	}
	wallet, err := s.repo.CreateWallet(ctx, input)
	if err != nil {
		return nil, err
	}
	s.enrichWallet(ctx, wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) UpdateWallet(ctx context.Context, id int64, input UpstreamWalletInput) (*UpstreamWallet, error) {
	if err := normalizeUpstreamWalletInput(&input); err != nil {
		return nil, err
	}
	wallet, err := s.repo.UpdateWallet(ctx, id, input)
	if err != nil {
		return nil, err
	}
	s.enrichWallet(ctx, wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) RecordManualBalance(ctx context.Context, id int64, balance float64) (*UpstreamWallet, error) {
	if !isFiniteNonnegative(balance) {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_INVALID", "balance must be greater than or equal to zero")
	}
	wallet, err := s.repo.GetWallet(ctx, id)
	if err != nil {
		return nil, err
	}
	wallet, err = s.repo.RecordBalanceSuccess(ctx, id, balance, wallet.Currency, UpstreamBalanceSourceManual)
	if err != nil {
		return nil, err
	}
	s.enrichWallet(ctx, wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) RefreshBalance(ctx context.Context, id int64) (*UpstreamWallet, error) {
	value, err, _ := s.refreshGroup.Do(strconv.FormatInt(id, 10), func() (any, error) {
		wallet, err := s.repo.GetWallet(ctx, id)
		if err != nil {
			return nil, err
		}
		accounts, err := s.loadWalletAccounts(ctx, wallet)
		if err != nil {
			return nil, err
		}
		if s.balanceProvider == nil || !s.balanceProvider.BalanceConfigured(wallet, accounts) {
			_ = s.repo.RecordBalanceFailure(ctx, id, "adapter_not_configured", UpstreamBalanceSourceSub2APIUsage)
			return nil, ErrUpstreamBalanceRefreshUnavailable
		}

		snapshot, refreshErr := s.balanceProvider.RefreshBalance(ctx, wallet, accounts)
		if refreshErr != nil {
			_ = s.repo.RecordBalanceFailure(ctx, id, upstreamBalanceErrorSummary(refreshErr), UpstreamBalanceSourceSub2APIUsage)
			return nil, ErrUpstreamBalanceRefreshUnavailable
		}
		if snapshot == nil || !isFiniteNonnegative(snapshot.Balance) || !strings.EqualFold(snapshot.Currency, wallet.Currency) {
			_ = s.repo.RecordBalanceFailure(ctx, id, "invalid_balance_response", UpstreamBalanceSourceSub2APIUsage)
			return nil, ErrUpstreamBalanceRefreshUnavailable
		}

		updated, err := s.repo.RecordBalanceSuccess(
			ctx,
			id,
			snapshot.Balance,
			strings.ToUpper(snapshot.Currency),
			UpstreamBalanceSourceSub2APIUsage,
		)
		if err != nil {
			return nil, err
		}
		updated.AdapterConfigured = true
		calculateUpstreamWalletMetrics(updated)
		return updated, nil
	})
	if err != nil {
		return nil, err
	}
	wallet, ok := value.(*UpstreamWallet)
	if !ok || wallet == nil {
		return nil, ErrUpstreamBalanceRefreshUnavailable
	}
	return wallet, nil
}

func (s *UpstreamFundsService) RedeemCode(ctx context.Context, id int64, code string) (*UpstreamRedeemResult, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 512 {
		return nil, infraerrors.BadRequest("UPSTREAM_REDEEM_CODE_INVALID", "redeem code is required")
	}
	wallet, err := s.repo.GetWallet(ctx, id)
	if err != nil {
		return nil, err
	}
	if wallet.RechargeMode != "product" {
		return nil, infraerrors.BadRequest("UPSTREAM_REDEEM_MODE_INVALID", "wallet is not configured for product or voucher recharge")
	}
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, err
	}
	if s.codeRedeemer == nil || !s.codeRedeemer.RedeemConfigured(wallet, accounts) {
		return nil, ErrUpstreamRedeemUnavailable
	}

	var balanceBefore *float64
	if wallet.Balance != nil {
		value := *wallet.Balance
		balanceBefore = &value
	}
	if err := s.codeRedeemer.RedeemCode(ctx, wallet, accounts, code); err != nil {
		return nil, ErrUpstreamRedeemUnavailable
	}
	updated, refreshErr := s.RefreshBalance(ctx, id)
	if refreshErr != nil {
		latest, _ := s.repo.GetWallet(ctx, id)
		if latest == nil {
			latest = wallet
		}
		calculateUpstreamWalletMetrics(latest)
		return &UpstreamRedeemResult{Status: "manual_review", Wallet: latest}, nil
	}
	status := "manual_review"
	if balanceBefore != nil && updated.Balance != nil && *updated.Balance > *balanceBefore {
		status = "verified"
	}
	return &UpstreamRedeemResult{Status: status, Wallet: updated}, nil
}

func (s *UpstreamFundsService) ListAccountOptions(ctx context.Context) ([]UpstreamFundsAccount, error) {
	accounts, err := s.repo.ListAccountOptions(ctx)
	if accounts == nil {
		accounts = []UpstreamFundsAccount{}
	}
	return accounts, err
}

func (s *UpstreamFundsService) attachAdapterCapabilities(ctx context.Context, wallets []UpstreamWallet) error {
	for i := range wallets {
		s.normalizePanelSessionState(&wallets[i])
	}
	if len(wallets) == 0 || s.accountRepo == nil || s.balanceProvider == nil {
		return nil
	}
	accountIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	for i := range wallets {
		for _, id := range wallets[i].AccountIDs {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			accountIDs = append(accountIDs, id)
		}
	}
	if len(accountIDs) == 0 {
		return nil
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil {
		return err
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, account := range accounts {
		if account != nil {
			byID[account.ID] = account
		}
	}
	for i := range wallets {
		linked := make([]*Account, 0, len(wallets[i].AccountIDs))
		for _, id := range wallets[i].AccountIDs {
			if account := byID[id]; account != nil {
				linked = append(linked, account)
			}
		}
		wallets[i].AdapterConfigured = s.balanceProvider.BalanceConfigured(&wallets[i], linked)
		wallets[i].RedeemConfigured = s.codeRedeemer != nil && s.codeRedeemer.RedeemConfigured(&wallets[i], linked)
		wallets[i].RechargeConfigured = s.rechargeProvider != nil && s.rechargeProvider.RechargeConfigured(&wallets[i], linked)
	}
	return nil
}

func (s *UpstreamFundsService) enrichWallet(ctx context.Context, wallet *UpstreamWallet) {
	if wallet == nil {
		return
	}
	s.normalizePanelSessionState(wallet)
	wallet.AdapterConfigured = s.walletBalanceRefreshConfigured(ctx, wallet)
	wallet.RedeemConfigured = s.walletRedeemConfigured(ctx, wallet)
	wallet.RechargeConfigured = s.walletRechargeConfigured(ctx, wallet)
	calculateUpstreamWalletMetrics(wallet)
}

func (s *UpstreamFundsService) walletBalanceRefreshConfigured(ctx context.Context, wallet *UpstreamWallet) bool {
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	return err == nil && s.balanceProvider != nil && s.balanceProvider.BalanceConfigured(wallet, accounts)
}

func (s *UpstreamFundsService) walletRedeemConfigured(ctx context.Context, wallet *UpstreamWallet) bool {
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	return err == nil && s.codeRedeemer != nil && s.codeRedeemer.RedeemConfigured(wallet, accounts)
}

func (s *UpstreamFundsService) walletRechargeConfigured(ctx context.Context, wallet *UpstreamWallet) bool {
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	return err == nil && s.rechargeProvider != nil && s.rechargeProvider.RechargeConfigured(wallet, accounts)
}

func (s *UpstreamFundsService) loadWalletAccounts(ctx context.Context, wallet *UpstreamWallet) ([]*Account, error) {
	if wallet == nil || len(wallet.AccountIDs) == 0 || s.accountRepo == nil {
		return nil, nil
	}
	return s.accountRepo.GetByIDs(ctx, wallet.AccountIDs)
}

func normalizeUpstreamWalletInput(input *UpstreamWalletInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.RechargeMode = strings.ToLower(strings.TrimSpace(input.RechargeMode))
	input.CardSiteURL = strings.TrimSpace(input.CardSiteURL)
	input.Tier = strings.ToLower(strings.TrimSpace(input.Tier))

	if input.Name == "" || input.Provider == "" || input.Currency == "" {
		return infraerrors.BadRequest("UPSTREAM_WALLET_INVALID", "name, provider and currency are required")
	}
	if len(input.Name) > 100 || len(input.Provider) > 64 {
		return infraerrors.BadRequest("UPSTREAM_WALLET_INVALID", "wallet fields exceed the maximum length")
	}
	if !isCurrencyCode(input.Currency) {
		return infraerrors.BadRequest("UPSTREAM_CURRENCY_INVALID", "currency must be a three-letter ISO code")
	}
	if !oneOf(input.RechargeMode, "direct", "product", "manual") {
		return infraerrors.BadRequest("UPSTREAM_RECHARGE_MODE_INVALID", "invalid recharge mode")
	}
	if input.CardSiteURL != "" {
		if !isSafeAbsoluteHTTPURL(input.CardSiteURL) {
			return infraerrors.BadRequest("UPSTREAM_CARD_SITE_URL_INVALID", "card site URL must be an absolute HTTP or HTTPS URL")
		}
		if len(input.CardSiteURL) > 2048 {
			return infraerrors.BadRequest("UPSTREAM_CARD_SITE_URL_INVALID", "card site URL exceeds the maximum length")
		}
	}
	if !oneOf(input.Tier, "primary", "hot_backup", "cold_backup") {
		return infraerrors.BadRequest("UPSTREAM_TIER_INVALID", "invalid wallet tier")
	}
	if input.AlertDays < 0 || input.AlertDays > 365 || input.TargetDays < 0 || input.TargetDays > 365 {
		return infraerrors.BadRequest("UPSTREAM_RESERVE_DAYS_INVALID", "reserve days must be between 0 and 365")
	}
	if input.TargetDays < input.AlertDays {
		return infraerrors.BadRequest("UPSTREAM_RESERVE_DAYS_INVALID", "target days must be greater than or equal to alert days")
	}

	seen := make(map[int64]struct{}, len(input.AccountIDs))
	cleanIDs := make([]int64, 0, len(input.AccountIDs))
	for _, id := range input.AccountIDs {
		if id <= 0 {
			return infraerrors.BadRequest("UPSTREAM_ACCOUNT_INVALID", "account IDs must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		cleanIDs = append(cleanIDs, id)
	}
	sort.Slice(cleanIDs, func(i, j int) bool { return cleanIDs[i] < cleanIDs[j] })
	input.AccountIDs = cleanIDs
	return nil
}

func calculateUpstreamWalletMetrics(wallet *UpstreamWallet) {
	if wallet == nil {
		return
	}
	wallet.CostCurrency = "USD"
	wallet.DailyCost7D = wallet.Cost7D / 7
	wallet.RunwayDays = nil
	wallet.RecommendedTopUp = nil

	if wallet.Balance != nil && wallet.Currency == wallet.CostCurrency && wallet.DailyCost7D > 0 {
		runway := *wallet.Balance / wallet.DailyCost7D
		topUp := float64(wallet.TargetDays)*wallet.DailyCost7D - *wallet.Balance
		if topUp < 0 {
			topUp = 0
		}
		wallet.RunwayDays = &runway
		wallet.RecommendedTopUp = &topUp
	}

	wallet.NeedsAttention = wallet.Enabled && (wallet.Balance == nil || wallet.BalanceError != "")
	if wallet.Enabled && wallet.RunwayDays != nil && *wallet.RunwayDays < float64(wallet.AlertDays) {
		wallet.NeedsAttention = true
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func isCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func isFiniteNonnegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func isSafeAbsoluteHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Host != "" && parsed.User == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
