package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUpstreamFundsWalletConsumptionUsesAccountCostFormula(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	expr := regexp.QuoteMeta("COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)")
	mock.ExpectQuery("(?s)" + expr + ".*" + expr + ".*" + expr).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	repo := &upstreamFundsRepository{db: db}
	wallets, err := repo.ListWallets(context.Background(), "", 0)
	require.NoError(t, err)
	require.Empty(t, wallets)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamFundsWalletsFilterByConfiguredOrLatestActualGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)filter_ag\.group_id = \$1.*latest_usage\.group_id = \$1`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	repo := &upstreamFundsRepository{db: db}
	wallets, err := repo.ListWallets(context.Background(), "", 17)
	require.NoError(t, err)
	require.Empty(t, wallets)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteUpstreamWalletSoftDeletesAndReleasesAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id FROM upstream_wallets.*WHERE id = \$1 AND deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(`(?s)SELECT EXISTS.*upstream_recharge_orders.*status NOT IN`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)UPDATE upstream_wallets.*deleted_at = NOW\(\).*WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM upstream_wallet_accounts WHERE wallet_id = \$1`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	repo := &upstreamFundsRepository{db: db}
	require.NoError(t, repo.DeleteWallet(context.Background(), 9))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteUpstreamWalletRejectsActiveRechargeOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id FROM upstream_wallets.*WHERE id = \$1 AND deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(`(?s)SELECT EXISTS.*upstream_recharge_orders.*status NOT IN`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	repo := &upstreamFundsRepository{db: db}
	err = repo.DeleteWallet(context.Background(), 9)
	require.ErrorIs(t, err, service.ErrUpstreamWalletHasActiveRecharge)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUpstreamRechargeOrderRejectsDeletedWalletUnderLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id FROM upstream_wallets.*WHERE id = \$1 AND deleted_at IS NULL.*FOR SHARE`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	repo := &upstreamFundsRepository{db: db}
	_, err = repo.CreateRechargeOrder(context.Background(), &service.UpstreamRechargeOrder{WalletID: 9}, 1)
	require.ErrorIs(t, err, service.ErrUpstreamWalletNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyWalletSyncCreatesDomainNamedWalletAndLinksAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(\$1\)`).
		WithArgs(int64(0x5550574C)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT a.id.*NOT EXISTS.*FOR UPDATE`).
		WithArgs(pq.Array([]int64{3, 9})).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)).AddRow(int64(9)))
	mock.ExpectQuery(`(?s)INSERT INTO upstream_wallets.*name, provider, currency, recharge_mode.*RETURNING id`).
		WithArgs("relay.example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(12)))
	mock.ExpectExec(`(?s)INSERT INTO upstream_wallet_accounts.*ON CONFLICT \(account_id\) DO NOTHING`).
		WithArgs(int64(12), pq.Array([]int64{3, 9})).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repo := &upstreamFundsRepository{db: db}
	result, err := repo.ApplyWalletSync(context.Background(), []service.UpstreamWalletSyncPlan{{
		Domain: "relay.example.com", AccountIDs: []int64{3, 9},
	}})
	require.NoError(t, err)
	require.Equal(t, 1, result.CreatedWallets)
	require.Equal(t, 2, result.LinkedAccounts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveUpstreamPanelCredentialsDoesNotStorePlainPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`(?s)UPDATE upstream_wallets.*panel_login_password_ciphertext`).
		WithArgs(int64(9), int64(42), "owner@example.com", "encrypted-password").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &upstreamFundsRepository{db: db}
	err = repo.SaveUpstreamPanelCredentials(context.Background(), 9, 42, "owner@example.com", "encrypted-password")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClearUpstreamPanelCredentialsRemovesOnlySavedLogin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`(?s)UPDATE upstream_wallets.*panel_login_password_ciphertext = ''`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &upstreamFundsRepository{db: db}
	err = repo.ClearUpstreamPanelCredentials(context.Background(), 9)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCompleteRechargeOrderUpdatesWalletSnapshotAndEventInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	balanceAfter := 125.5
	order := &service.UpstreamRechargeOrder{ID: 71, BalanceAfter: &balanceAfter}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE upstream_recharge_orders.*RETURNING wallet_id`).
		WithArgs(int64(71), service.UpstreamRechargeStatusManualReview, balanceAfter).
		WillReturnRows(sqlmock.NewRows([]string{"wallet_id"}).AddRow(int64(9)))
	mock.ExpectQuery(`(?s)UPDATE upstream_wallets.*RETURNING currency`).
		WithArgs(int64(9), balanceAfter).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("USD"))
	mock.ExpectExec(`(?s)INSERT INTO upstream_balance_snapshots`).
		WithArgs(int64(9), balanceAfter, "USD").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO upstream_recharge_order_events`).
		WithArgs(int64(71), service.UpstreamRechargeStatusManualReview, int64(3), "verified in provider panel").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`(?s)SELECT id, order_no, idempotency_key.*FROM upstream_recharge_orders WHERE id = \$1`).
		WithArgs(int64(71)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_no", "idempotency_key", "wallet_id", "product_id", "provider_order_id",
			"payment_channel_id", "face_value", "pay_amount", "currency", "status", "payment_qr",
			"payment_url", "payment_expires_at", "balance_before", "balance_after", "error_code",
			"error_message", "created_at", "updated_at", "completed_at",
		}).AddRow(
			int64(71), "UR1", "key", int64(9), nil, "provider-1", "alipay", 100.0, 98.0,
			"CNY", service.UpstreamRechargeStatusCompleted, "qr", "https://pay.example.com/1", nil,
			25.0, balanceAfter, "", "", now, now, now,
		))

	repo := &upstreamFundsRepository{db: db}
	completed, err := repo.CompleteRechargeOrder(
		context.Background(), order, service.UpstreamRechargeStatusManualReview,
		"verified in provider panel", 3,
	)
	require.NoError(t, err)
	require.Equal(t, service.UpstreamRechargeStatusCompleted, completed.Status)
	require.NotNil(t, completed.BalanceAfter)
	require.InDelta(t, balanceAfter, *completed.BalanceAfter, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}
