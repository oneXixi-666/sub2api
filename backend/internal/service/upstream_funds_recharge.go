package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	UpstreamRechargeStatusCreating       = "creating"
	UpstreamRechargeStatusPendingPayment = "pending_payment"
	UpstreamRechargeStatusPaid           = "paid"
	UpstreamRechargeStatusVerifying      = "verifying"
	UpstreamRechargeStatusCompleted      = "completed"
	UpstreamRechargeStatusManualReview   = "manual_review"
	UpstreamRechargeStatusFailed         = "failed"
	UpstreamRechargeStatusExpired        = "expired"
	UpstreamRechargeStatusCancelled      = "cancelled"

	upstreamRechargeCreatingStaleAfter = 30 * time.Second
)

var (
	ErrUpstreamRechargeOrderNotFound = infraerrors.NotFound("UPSTREAM_RECHARGE_ORDER_NOT_FOUND", "upstream recharge order not found")
	ErrUpstreamRechargeConflict      = infraerrors.Conflict("UPSTREAM_RECHARGE_STATE_CONFLICT", "upstream recharge order state changed; reload and retry")
	ErrUpstreamRechargeUnavailable   = infraerrors.ServiceUnavailable("UPSTREAM_RECHARGE_UNAVAILABLE", "upstream recharge adapter is unavailable")
)

type UpstreamRechargeProduct struct {
	ID           int64      `json:"id"`
	WalletID     int64      `json:"wallet_id"`
	ExternalRef  string     `json:"external_ref"`
	Name         string     `json:"name"`
	FaceValue    float64    `json:"face_value"`
	PayAmount    float64    `json:"pay_amount"`
	Currency     string     `json:"currency"`
	Stock        *int       `json:"stock"`
	Enabled      bool       `json:"enabled"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
}

type UpstreamRechargeOrder struct {
	ID               int64      `json:"id"`
	OrderNo          string     `json:"order_no"`
	IdempotencyKey   string     `json:"-"`
	WalletID         int64      `json:"wallet_id"`
	ProductID        *int64     `json:"product_id"`
	ProviderOrderID  string     `json:"provider_order_id"`
	PaymentChannelID string     `json:"payment_channel_id"`
	FaceValue        float64    `json:"face_value"`
	PayAmount        float64    `json:"pay_amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	PaymentQR        string     `json:"payment_qr"`
	PaymentURL       string     `json:"payment_url"`
	PaymentExpiresAt *time.Time `json:"payment_expires_at"`
	BalanceBefore    *float64   `json:"balance_before"`
	BalanceAfter     *float64   `json:"balance_after"`
	ErrorCode        string     `json:"error_code"`
	ErrorMessage     string     `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CompletedAt      *time.Time `json:"completed_at"`
}

type UpstreamRechargeOrderInput struct {
	Amount           float64
	PaymentChannelID string
	IdempotencyKey   string
}

func (s *UpstreamFundsService) ListRechargeProducts(ctx context.Context, walletID int64) ([]UpstreamRechargeProduct, error) {
	if _, err := s.repo.GetWallet(ctx, walletID); err != nil {
		return nil, err
	}
	products, err := s.repo.ListRechargeProducts(ctx, walletID)
	if products == nil {
		products = []UpstreamRechargeProduct{}
	}
	return products, err
}

func (s *UpstreamFundsService) ReplaceRechargeProducts(ctx context.Context, walletID int64, products []UpstreamRechargeProduct) ([]UpstreamRechargeProduct, error) {
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	for i := range products {
		products[i].WalletID = walletID
		products[i].Name = strings.TrimSpace(products[i].Name)
		products[i].ExternalRef = strings.TrimSpace(products[i].ExternalRef)
		products[i].Currency = strings.ToUpper(strings.TrimSpace(products[i].Currency))
		if products[i].Name == "" || len(products[i].Name) > 160 || len(products[i].ExternalRef) > 128 ||
			!isFinitePositive(products[i].FaceValue) || !isFiniteNonnegative(products[i].PayAmount) ||
			!isCurrencyCode(products[i].Currency) || (products[i].Stock != nil && *products[i].Stock < 0) {
			return nil, infraerrors.BadRequest("UPSTREAM_RECHARGE_PRODUCT_INVALID", "invalid recharge product")
		}
		if products[i].Currency != wallet.Currency {
			return nil, infraerrors.BadRequest("UPSTREAM_RECHARGE_PRODUCT_CURRENCY_INVALID", "product currency must match wallet currency")
		}
	}
	return s.repo.ReplaceRechargeProducts(ctx, walletID, products)
}

func (s *UpstreamFundsService) ListPaymentChannels(ctx context.Context, walletID int64) ([]UpstreamPaymentChannel, error) {
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, err
	}
	if s.rechargeProvider == nil || !s.rechargeProvider.RechargeConfigured(wallet, accounts) {
		return nil, ErrUpstreamRechargeUnavailable
	}
	channels, err := s.rechargeProvider.ListPaymentChannels(ctx, wallet, accounts)
	if err != nil {
		return nil, ErrUpstreamRechargeUnavailable
	}
	if err := normalizeUpstreamPaymentChannels(channels); err != nil {
		return nil, ErrUpstreamRechargeUnavailable
	}
	if channels == nil {
		channels = []UpstreamPaymentChannel{}
	}
	return channels, nil
}

func (s *UpstreamFundsService) CreateRechargeOrder(
	ctx context.Context,
	walletID int64,
	input UpstreamRechargeOrderInput,
	actorID int64,
) (*UpstreamRechargeOrder, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.PaymentChannelID = strings.TrimSpace(input.PaymentChannelID)
	if !isFinitePositive(input.Amount) || input.PaymentChannelID == "" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return nil, infraerrors.BadRequest("UPSTREAM_RECHARGE_ORDER_INVALID", "amount, payment channel and idempotency key are required")
	}
	if existing, err := s.repo.GetRechargeOrderByIdempotency(ctx, walletID, input.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrUpstreamRechargeOrderNotFound) {
		return nil, err
	}
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	if wallet.RechargeMode != "direct" {
		return nil, infraerrors.BadRequest("UPSTREAM_RECHARGE_MODE_INVALID", "wallet is not configured for direct recharge")
	}
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, err
	}
	if s.rechargeProvider == nil || !s.rechargeProvider.RechargeConfigured(wallet, accounts) {
		return nil, ErrUpstreamRechargeUnavailable
	}
	channels, err := s.rechargeProvider.ListPaymentChannels(ctx, wallet, accounts)
	if err != nil {
		return nil, ErrUpstreamRechargeUnavailable
	}
	if err := normalizeUpstreamPaymentChannels(channels); err != nil {
		return nil, ErrUpstreamRechargeUnavailable
	}
	var channel *UpstreamPaymentChannel
	for i := range channels {
		if channels[i].ID == input.PaymentChannelID {
			channel = &channels[i]
			break
		}
	}
	if channel == nil || (channel.SingleMin > 0 && input.Amount < channel.SingleMin) || (channel.SingleMax > 0 && input.Amount > channel.SingleMax) {
		return nil, infraerrors.BadRequest("UPSTREAM_PAYMENT_CHANNEL_INVALID", "payment channel is unavailable for this amount")
	}

	order := &UpstreamRechargeOrder{
		OrderNo:        "UR" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:24],
		IdempotencyKey: input.IdempotencyKey, WalletID: walletID,
		PaymentChannelID: input.PaymentChannelID, FaceValue: input.Amount,
		PayAmount: input.Amount, Currency: channel.Currency, Status: UpstreamRechargeStatusCreating,
		BalanceBefore: wallet.Balance,
	}
	order, err = s.repo.CreateRechargeOrder(ctx, order, actorID)
	if err != nil {
		if existing, lookupErr := s.repo.GetRechargeOrderByIdempotency(ctx, walletID, input.IdempotencyKey); lookupErr == nil {
			return existing, nil
		}
		return nil, err
	}
	providerUpdate, createErr := s.rechargeProvider.CreateRechargeOrder(ctx, wallet, accounts, input.Amount, input.PaymentChannelID)
	if createErr != nil {
		order.Status = UpstreamRechargeStatusManualReview
		order.ErrorCode = upstreamBalanceErrorSummary(createErr)
		order.ErrorMessage = "upstream order result requires manual review"
		return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusCreating, "create_uncertain", order.ErrorCode, actorID)
	}
	if err := applyProviderOrderUpdate(order, providerUpdate); err != nil {
		preserveProviderOrderReference(order, providerUpdate)
		order.Status = UpstreamRechargeStatusManualReview
		order.ErrorCode = "invalid_provider_order"
		order.ErrorMessage = "upstream order response requires manual review"
		return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusCreating, "create_invalid", order.ErrorCode, actorID)
	}
	order.Status = mapProviderRechargeStatus(providerUpdate.Status)
	if order.Status == UpstreamRechargeStatusManualReview {
		order.ErrorCode = "unknown_provider_status"
		order.ErrorMessage = "upstream returned an unknown order status"
	}
	return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusCreating, "provider_status", providerUpdate.Status, actorID)
}

func (s *UpstreamFundsService) GetRechargeOrder(ctx context.Context, id int64) (*UpstreamRechargeOrder, error) {
	return s.repo.GetRechargeOrder(ctx, id)
}

func (s *UpstreamFundsService) PollRechargeOrder(ctx context.Context, id int64, actorID int64) (*UpstreamRechargeOrder, error) {
	value, err, _ := s.orderPollGroup.Do(fmt.Sprintf("order:%d", id), func() (any, error) {
		order, err := s.repo.GetRechargeOrder(ctx, id)
		if err != nil {
			return nil, err
		}
		if rechargeOrderTerminal(order.Status) || order.Status == UpstreamRechargeStatusManualReview {
			return order, nil
		}
		if order.Status == UpstreamRechargeStatusCreating {
			if !order.UpdatedAt.IsZero() && time.Since(order.UpdatedAt) < upstreamRechargeCreatingStaleAfter {
				return order, nil
			}
			order.Status = UpstreamRechargeStatusManualReview
			order.ErrorCode = "create_interrupted"
			order.ErrorMessage = "local order creation was interrupted; verify upstream before retrying"
			return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusCreating, "create_interrupted", order.ErrorCode, actorID)
		}
		wallet, err := s.repo.GetWallet(ctx, order.WalletID)
		if err != nil {
			return nil, err
		}
		if order.Status == UpstreamRechargeStatusVerifying {
			updatedWallet, refreshErr := s.RefreshBalance(ctx, wallet.ID)
			if refreshErr != nil || updatedWallet.Balance == nil || order.BalanceBefore == nil || *updatedWallet.Balance <= *order.BalanceBefore {
				order.Status = UpstreamRechargeStatusManualReview
				order.ErrorCode = "balance_not_verified"
				order.ErrorMessage = "payment completed but balance increase was not verified"
				return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusVerifying, "verification_failed", order.ErrorCode, actorID)
			}
			value := *updatedWallet.Balance
			order.BalanceAfter = &value
			order.Status = UpstreamRechargeStatusCompleted
			now := time.Now().UTC()
			order.CompletedAt = &now
			return s.repo.UpdateRechargeOrder(ctx, order, UpstreamRechargeStatusVerifying, "balance_verified", "balance increase verified", actorID)
		}
		accounts, err := s.loadWalletAccounts(ctx, wallet)
		if err != nil {
			return nil, err
		}
		if s.rechargeProvider == nil || !s.rechargeProvider.RechargeConfigured(wallet, accounts) {
			return nil, ErrUpstreamRechargeUnavailable
		}
		providerUpdate, err := s.rechargeProvider.QueryRechargeOrder(ctx, wallet, accounts, order.ProviderOrderID)
		if err != nil {
			return nil, ErrUpstreamRechargeUnavailable
		}
		fromStatus := order.Status
		before := *order
		if err := applyProviderOrderUpdate(order, providerUpdate); err != nil {
			preserveProviderOrderReference(order, providerUpdate)
			order.Status = UpstreamRechargeStatusManualReview
			order.ErrorCode = "invalid_provider_order"
			order.ErrorMessage = "upstream order response requires manual review"
			return s.repo.UpdateRechargeOrder(ctx, order, fromStatus, "provider_invalid", order.ErrorCode, actorID)
		}
		order.Status = mapProviderRechargeStatus(providerUpdate.Status)
		if order.Status == UpstreamRechargeStatusManualReview {
			order.ErrorCode = "unknown_provider_status"
			order.ErrorMessage = "upstream returned an unknown order status"
		} else if !upstreamRechargeTransitionAllowed(fromStatus, order.Status) {
			order.Status = UpstreamRechargeStatusManualReview
			order.ErrorCode = "invalid_provider_transition"
			order.ErrorMessage = "upstream order status attempted an invalid transition"
		}
		if order.Status == fromStatus && !rechargeOrderProviderFieldsChanged(&before, order) {
			return order, nil
		}
		return s.repo.UpdateRechargeOrder(ctx, order, fromStatus, "provider_status", providerUpdate.Status, actorID)
	})
	if err != nil {
		return nil, err
	}
	return value.(*UpstreamRechargeOrder), nil
}

func (s *UpstreamFundsService) ManualCompleteRechargeOrder(ctx context.Context, id int64, balanceAfter float64, reason string, actorID int64) (*UpstreamRechargeOrder, error) {
	reason = strings.TrimSpace(reason)
	if !isFiniteNonnegative(balanceAfter) || reason == "" || len(reason) > 500 {
		return nil, infraerrors.BadRequest("UPSTREAM_MANUAL_COMPLETE_INVALID", "balance after and reason are required")
	}
	order, err := s.repo.GetRechargeOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.Status != UpstreamRechargeStatusManualReview && order.Status != UpstreamRechargeStatusVerifying {
		return nil, ErrUpstreamRechargeConflict
	}
	fromStatus := order.Status
	order.BalanceAfter = &balanceAfter
	return s.repo.CompleteRechargeOrder(ctx, order, fromStatus, reason, actorID)
}

func applyProviderOrderUpdate(order *UpstreamRechargeOrder, update *UpstreamProviderOrderUpdate) error {
	if order == nil || update == nil {
		return errors.New("missing provider order update")
	}
	update.ProviderOrderID = strings.TrimSpace(update.ProviderOrderID)
	update.Status = strings.TrimSpace(update.Status)
	update.Currency = strings.ToUpper(strings.TrimSpace(update.Currency))
	update.PaymentURL = strings.TrimSpace(update.PaymentURL)
	if update.ProviderOrderID == "" || len(update.ProviderOrderID) > 128 || len(update.Status) > 64 ||
		!isFiniteNonnegative(update.PayAmount) || !isCurrencyCode(update.Currency) || !strings.EqualFold(update.Currency, order.Currency) {
		return errors.New("invalid provider order fields")
	}
	if order.ProviderOrderID != "" && update.ProviderOrderID != order.ProviderOrderID {
		return errors.New("provider order reference changed")
	}
	if update.PaymentURL != "" && (len(update.PaymentURL) > 2048 || !isSafeAbsoluteHTTPURL(update.PaymentURL)) {
		return errors.New("unsafe provider payment URL")
	}
	order.ProviderOrderID = update.ProviderOrderID
	order.PayAmount = update.PayAmount
	order.Currency = update.Currency
	if update.PaymentQR != "" {
		order.PaymentQR = update.PaymentQR
	}
	if update.PaymentURL != "" {
		order.PaymentURL = update.PaymentURL
	}
	if update.ExpiresAt != nil {
		order.PaymentExpiresAt = update.ExpiresAt
	}
	return nil
}

func preserveProviderOrderReference(order *UpstreamRechargeOrder, update *UpstreamProviderOrderUpdate) {
	if order == nil || update == nil {
		return
	}
	if strings.TrimSpace(order.ProviderOrderID) != "" {
		return
	}
	reference := strings.TrimSpace(update.ProviderOrderID)
	if reference != "" && len(reference) <= 128 {
		order.ProviderOrderID = reference
	}
}

func normalizeUpstreamPaymentChannels(channels []UpstreamPaymentChannel) error {
	seen := make(map[string]struct{}, len(channels))
	for i := range channels {
		channel := &channels[i]
		channel.ID = strings.TrimSpace(channel.ID)
		channel.Name = strings.TrimSpace(channel.Name)
		channel.Currency = strings.ToUpper(strings.TrimSpace(channel.Currency))
		if channel.ID == "" || len(channel.ID) > 64 || channel.Name == "" || !isCurrencyCode(channel.Currency) {
			return errors.New("invalid payment channel identity")
		}
		if !isFiniteNonnegative(channel.SingleMin) || !isFiniteNonnegative(channel.SingleMax) ||
			!isFiniteNonnegative(channel.FeeRate) || !isFiniteNonnegative(channel.DailyRemaining) ||
			(channel.SingleMax > 0 && channel.SingleMin > channel.SingleMax) {
			return errors.New("invalid payment channel limits")
		}
		if _, exists := seen[channel.ID]; exists {
			return errors.New("duplicate payment channel")
		}
		seen[channel.ID] = struct{}{}
	}
	return nil
}

func isFinitePositive(value float64) bool {
	return isFiniteNonnegative(value) && value > 0
}

func rechargeOrderProviderFieldsChanged(before, after *UpstreamRechargeOrder) bool {
	if before == nil || after == nil {
		return before != after
	}
	if before.ProviderOrderID != after.ProviderOrderID || before.PayAmount != after.PayAmount || before.Currency != after.Currency ||
		before.PaymentQR != after.PaymentQR || before.PaymentURL != after.PaymentURL {
		return true
	}
	if before.PaymentExpiresAt == nil || after.PaymentExpiresAt == nil {
		return before.PaymentExpiresAt != after.PaymentExpiresAt
	}
	return !before.PaymentExpiresAt.Equal(*after.PaymentExpiresAt)
}

func upstreamRechargeTransitionAllowed(fromStatus, toStatus string) bool {
	switch fromStatus {
	case UpstreamRechargeStatusPendingPayment:
		return oneOf(
			toStatus,
			UpstreamRechargeStatusPendingPayment,
			UpstreamRechargeStatusPaid,
			UpstreamRechargeStatusVerifying,
			UpstreamRechargeStatusManualReview,
			UpstreamRechargeStatusFailed,
			UpstreamRechargeStatusExpired,
			UpstreamRechargeStatusCancelled,
		)
	case UpstreamRechargeStatusPaid:
		return oneOf(toStatus, UpstreamRechargeStatusPaid, UpstreamRechargeStatusVerifying, UpstreamRechargeStatusManualReview)
	default:
		return false
	}
}

func mapProviderRechargeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return UpstreamRechargeStatusVerifying
	case "paid", "recharging":
		return UpstreamRechargeStatusPaid
	case "expired":
		return UpstreamRechargeStatusExpired
	case "failed":
		return UpstreamRechargeStatusFailed
	case "cancelled":
		return UpstreamRechargeStatusCancelled
	case "pending", "pending_payment", "created":
		return UpstreamRechargeStatusPendingPayment
	default:
		return UpstreamRechargeStatusManualReview
	}
}

func rechargeOrderTerminal(status string) bool {
	return status == UpstreamRechargeStatusCompleted || status == UpstreamRechargeStatusFailed || status == UpstreamRechargeStatusExpired || status == UpstreamRechargeStatusCancelled
}
