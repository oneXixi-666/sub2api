package service

import (
	"context"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	Provider          string                 `json:"provider"`
	Currency          string                 `json:"currency"`
	CostCurrency      string                 `json:"cost_currency"`
	RechargeMode      string                 `json:"recharge_mode"`
	Tier              string                 `json:"tier"`
	Enabled           bool                   `json:"enabled"`
	Balance           *float64               `json:"balance"`
	BalanceUpdatedAt  *time.Time             `json:"balance_updated_at"`
	BalanceError      string                 `json:"balance_error"`
	AlertDays         int                    `json:"alert_days"`
	TargetDays        int                    `json:"target_days"`
	AccountIDs        []int64                `json:"account_ids"`
	Accounts          []UpstreamFundsAccount `json:"accounts"`
	ConfiguredGroups  []UpstreamFundsGroup   `json:"configured_groups"`
	ActualGroups      []UpstreamFundsGroup   `json:"actual_groups"`
	Cost1H            float64                `json:"cost_1h"`
	CostToday         float64                `json:"cost_today"`
	Cost24H           float64                `json:"cost_24h"`
	Cost7D            float64                `json:"cost_7d"`
	DailyCost7D       float64                `json:"daily_cost_7d"`
	RunwayDays        *float64               `json:"runway_days"`
	RecommendedTopUp  *float64               `json:"recommended_top_up"`
	NeedsAttention    bool                   `json:"needs_attention"`
	AdapterConfigured bool                   `json:"adapter_configured"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type UpstreamWalletInput struct {
	Name         string
	Provider     string
	Currency     string
	RechargeMode string
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

type UpstreamFundsRepository interface {
	ListWallets(ctx context.Context, search string) ([]UpstreamWallet, error)
	GetWallet(ctx context.Context, id int64) (*UpstreamWallet, error)
	CreateWallet(ctx context.Context, input UpstreamWalletInput) (*UpstreamWallet, error)
	UpdateWallet(ctx context.Context, id int64, input UpstreamWalletInput) (*UpstreamWallet, error)
	RecordBalance(ctx context.Context, id int64, balance float64) (*UpstreamWallet, error)
	ListAccountOptions(ctx context.Context) ([]UpstreamFundsAccount, error)
}

type UpstreamFundsService struct {
	repo UpstreamFundsRepository
}

func NewUpstreamFundsService(repo UpstreamFundsRepository) *UpstreamFundsService {
	return &UpstreamFundsService{repo: repo}
}

func (s *UpstreamFundsService) ListWallets(ctx context.Context, search string) (*UpstreamFundsOverview, error) {
	wallets, err := s.repo.ListWallets(ctx, strings.TrimSpace(search))
	if err != nil {
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
	calculateUpstreamWalletMetrics(wallet)
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
	calculateUpstreamWalletMetrics(wallet)
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
	calculateUpstreamWalletMetrics(wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) RecordBalance(ctx context.Context, id int64, balance float64) (*UpstreamWallet, error) {
	if balance < 0 {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_INVALID", "balance must be greater than or equal to zero")
	}
	wallet, err := s.repo.RecordBalance(ctx, id, balance)
	if err != nil {
		return nil, err
	}
	calculateUpstreamWalletMetrics(wallet)
	return wallet, nil
}

func (s *UpstreamFundsService) ListAccountOptions(ctx context.Context) ([]UpstreamFundsAccount, error) {
	accounts, err := s.repo.ListAccountOptions(ctx)
	if accounts == nil {
		accounts = []UpstreamFundsAccount{}
	}
	return accounts, err
}

func normalizeUpstreamWalletInput(input *UpstreamWalletInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.RechargeMode = strings.ToLower(strings.TrimSpace(input.RechargeMode))
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
	wallet.AdapterConfigured = false
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
