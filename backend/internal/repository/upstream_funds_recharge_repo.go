package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *upstreamFundsRepository) ListRechargeProducts(ctx context.Context, walletID int64) ([]service.UpstreamRechargeProduct, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, wallet_id, external_ref, name, face_value, pay_amount, currency,
		       stock, enabled, last_synced_at
		FROM upstream_recharge_products
		WHERE wallet_id = $1
		ORDER BY enabled DESC, face_value, id
	`, walletID)
	if err != nil {
		return nil, fmt.Errorf("list upstream recharge products: %w", err)
	}
	defer rows.Close()
	products := make([]service.UpstreamRechargeProduct, 0)
	for rows.Next() {
		var item service.UpstreamRechargeProduct
		var stock sql.NullInt64
		var synced sql.NullTime
		if err := rows.Scan(&item.ID, &item.WalletID, &item.ExternalRef, &item.Name, &item.FaceValue, &item.PayAmount, &item.Currency, &stock, &item.Enabled, &synced); err != nil {
			return nil, fmt.Errorf("scan upstream recharge product: %w", err)
		}
		if stock.Valid {
			value := int(stock.Int64)
			item.Stock = &value
		}
		if synced.Valid {
			value := synced.Time
			item.LastSyncedAt = &value
		}
		products = append(products, item)
	}
	return products, rows.Err()
}

func (r *upstreamFundsRepository) ReplaceRechargeProducts(ctx context.Context, walletID int64, products []service.UpstreamRechargeProduct) ([]service.UpstreamRechargeProduct, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace upstream recharge products: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM upstream_recharge_products WHERE wallet_id = $1", walletID); err != nil {
		return nil, fmt.Errorf("clear upstream recharge products: %w", err)
	}
	for _, product := range products {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_recharge_products (
				wallet_id, external_ref, name, face_value, pay_amount, currency, stock, enabled, last_synced_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, walletID, product.ExternalRef, product.Name, product.FaceValue, product.PayAmount, product.Currency, product.Stock, product.Enabled, product.LastSyncedAt); err != nil {
			return nil, fmt.Errorf("insert upstream recharge product: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit upstream recharge products: %w", err)
	}
	return r.ListRechargeProducts(ctx, walletID)
}

func (r *upstreamFundsRepository) GetRechargeOrder(ctx context.Context, id int64) (*service.UpstreamRechargeOrder, error) {
	return r.queryRechargeOrder(ctx, `WHERE id = $1`, id)
}

func (r *upstreamFundsRepository) GetRechargeOrderByIdempotency(ctx context.Context, walletID int64, idempotencyKey string) (*service.UpstreamRechargeOrder, error) {
	return r.queryRechargeOrder(ctx, `WHERE wallet_id = $1 AND idempotency_key = $2`, walletID, idempotencyKey)
}

func (r *upstreamFundsRepository) queryRechargeOrder(ctx context.Context, clause string, args ...any) (*service.UpstreamRechargeOrder, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, order_no, idempotency_key, wallet_id, product_id, provider_order_id,
		       payment_channel_id, face_value, pay_amount, currency, status, payment_qr,
		       payment_url, payment_expires_at, balance_before, balance_after,
		       error_code, error_message, created_at, updated_at, completed_at
		FROM upstream_recharge_orders `+clause, args...)
	order, err := scanUpstreamRechargeOrder(row)
	if err == sql.ErrNoRows {
		return nil, service.ErrUpstreamRechargeOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get upstream recharge order: %w", err)
	}
	return order, nil
}

func (r *upstreamFundsRepository) CreateRechargeOrder(ctx context.Context, order *service.UpstreamRechargeOrder, actorID int64) (*service.UpstreamRechargeOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create upstream recharge order: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var lockedWalletID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM upstream_wallets
		WHERE id = $1 AND deleted_at IS NULL
		FOR SHARE
	`, order.WalletID).Scan(&lockedWalletID)
	if err == sql.ErrNoRows {
		return nil, service.ErrUpstreamWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock upstream wallet for recharge order: %w", err)
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO upstream_recharge_orders (
			order_no, idempotency_key, wallet_id, product_id, provider_order_id,
			payment_channel_id, face_value, pay_amount, currency, status, payment_qr,
			payment_url, payment_expires_at, balance_before, balance_after,
			error_code, error_message, created_by, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id
	`, order.OrderNo, order.IdempotencyKey, order.WalletID, order.ProductID, order.ProviderOrderID,
		order.PaymentChannelID, order.FaceValue, order.PayAmount, order.Currency, order.Status,
		order.PaymentQR, order.PaymentURL, order.PaymentExpiresAt, order.BalanceBefore,
		order.BalanceAfter, order.ErrorCode, order.ErrorMessage, nullableActorID(actorID), order.CompletedAt,
	).Scan(&order.ID)
	if err != nil {
		return nil, fmt.Errorf("insert upstream recharge order: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_recharge_order_events (order_id, from_status, to_status, event_type, actor_id, summary)
		VALUES ($1, '', $2, 'created', $3, 'local idempotent order created')
	`, order.ID, order.Status, nullableActorID(actorID)); err != nil {
		return nil, fmt.Errorf("insert upstream recharge event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit upstream recharge order: %w", err)
	}
	return r.GetRechargeOrder(ctx, order.ID)
}

func (r *upstreamFundsRepository) UpdateRechargeOrder(
	ctx context.Context,
	order *service.UpstreamRechargeOrder,
	fromStatus, eventType, summary string,
	actorID int64,
) (*service.UpstreamRechargeOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update upstream recharge order: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE upstream_recharge_orders
		SET provider_order_id=$3, pay_amount=$4, currency=$5, status=$6,
		    payment_qr=$7, payment_url=$8, payment_expires_at=$9,
		    balance_after=$10, error_code=$11, error_message=$12,
		    completed_at=$13, updated_at=NOW()
		WHERE id=$1 AND status=$2
	`, order.ID, fromStatus, order.ProviderOrderID, order.PayAmount, order.Currency,
		order.Status, order.PaymentQR, order.PaymentURL, order.PaymentExpiresAt,
		order.BalanceAfter, order.ErrorCode, order.ErrorMessage, order.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("update upstream recharge order: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read upstream recharge update result: %w", err)
	}
	if affected != 1 {
		return nil, service.ErrUpstreamRechargeConflict
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_recharge_order_events (order_id, from_status, to_status, event_type, actor_id, summary)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, order.ID, fromStatus, order.Status, eventType, nullableActorID(actorID), summary); err != nil {
		return nil, fmt.Errorf("insert upstream recharge event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit upstream recharge update: %w", err)
	}
	return r.GetRechargeOrder(ctx, order.ID)
}

func (r *upstreamFundsRepository) CompleteRechargeOrder(
	ctx context.Context,
	order *service.UpstreamRechargeOrder,
	fromStatus, reason string,
	actorID int64,
) (*service.UpstreamRechargeOrder, error) {
	if order == nil || order.BalanceAfter == nil {
		return nil, fmt.Errorf("complete upstream recharge order: balance after is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin complete upstream recharge order: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var walletID int64
	err = tx.QueryRowContext(ctx, `
		UPDATE upstream_recharge_orders
		SET status='completed', balance_after=$3, error_code='', error_message='',
		    completed_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status=$2
		RETURNING wallet_id
	`, order.ID, fromStatus, order.BalanceAfter).Scan(&walletID)
	if err == sql.ErrNoRows {
		return nil, service.ErrUpstreamRechargeConflict
	}
	if err != nil {
		return nil, fmt.Errorf("complete upstream recharge order: %w", err)
	}

	var currency string
	err = tx.QueryRowContext(ctx, `
		UPDATE upstream_wallets
		SET balance=$2, balance_updated_at=NOW(), balance_error='', updated_at=NOW()
			WHERE id=$1 AND deleted_at IS NULL
		RETURNING currency
	`, walletID, order.BalanceAfter).Scan(&currency)
	if err == sql.ErrNoRows {
		return nil, service.ErrUpstreamWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update manually verified upstream balance: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_balance_snapshots (wallet_id, balance, currency, status, source)
		VALUES ($1,$2,$3,'success','manual_recharge')
	`, walletID, order.BalanceAfter, currency); err != nil {
		return nil, fmt.Errorf("insert manually verified balance snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_recharge_order_events (order_id, from_status, to_status, event_type, actor_id, summary)
		VALUES ($1,$2,'completed','manual_complete',$3,$4)
	`, order.ID, fromStatus, nullableActorID(actorID), reason); err != nil {
		return nil, fmt.Errorf("insert manual completion event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit manual upstream recharge completion: %w", err)
	}
	return r.GetRechargeOrder(ctx, order.ID)
}

type rechargeOrderScanner interface {
	Scan(dest ...any) error
}

func scanUpstreamRechargeOrder(row rechargeOrderScanner) (*service.UpstreamRechargeOrder, error) {
	order := &service.UpstreamRechargeOrder{}
	var productID sql.NullInt64
	var paymentExpiresAt, completedAt sql.NullTime
	var balanceBefore, balanceAfter sql.NullFloat64
	err := row.Scan(
		&order.ID, &order.OrderNo, &order.IdempotencyKey, &order.WalletID, &productID,
		&order.ProviderOrderID, &order.PaymentChannelID, &order.FaceValue, &order.PayAmount,
		&order.Currency, &order.Status, &order.PaymentQR, &order.PaymentURL, &paymentExpiresAt,
		&balanceBefore, &balanceAfter, &order.ErrorCode, &order.ErrorMessage,
		&order.CreatedAt, &order.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if productID.Valid {
		value := productID.Int64
		order.ProductID = &value
	}
	if paymentExpiresAt.Valid {
		value := paymentExpiresAt.Time
		order.PaymentExpiresAt = &value
	}
	if balanceBefore.Valid {
		value := balanceBefore.Float64
		order.BalanceBefore = &value
	}
	if balanceAfter.Valid {
		value := balanceAfter.Float64
		order.BalanceAfter = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		order.CompletedAt = &value
	}
	return order, nil
}

func nullableActorID(actorID int64) any {
	if actorID <= 0 {
		return nil
	}
	return actorID
}
