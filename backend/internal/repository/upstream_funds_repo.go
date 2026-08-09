package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type upstreamFundsRepository struct {
	db *sql.DB
}

func NewUpstreamFundsRepository(db *sql.DB) service.UpstreamFundsRepository {
	return &upstreamFundsRepository{db: db}
}

func (r *upstreamFundsRepository) ListWallets(ctx context.Context, search string, groupID int64) ([]service.UpstreamWallet, error) {
	return r.queryWallets(ctx, nil, search, groupID)
}

func (r *upstreamFundsRepository) GetWallet(ctx context.Context, id int64) (*service.UpstreamWallet, error) {
	wallets, err := r.queryWallets(ctx, &id, "", 0)
	if err != nil {
		return nil, err
	}
	if len(wallets) == 0 {
		return nil, service.ErrUpstreamWalletNotFound
	}
	return &wallets[0], nil
}

func (r *upstreamFundsRepository) queryWallets(ctx context.Context, id *int64, search string, groupID int64) ([]service.UpstreamWallet, error) {
	query := `
		SELECT
			w.id, w.name, w.provider, COALESCE(w.extra->>'sync_domain', ''), w.currency, w.recharge_mode,
				COALESCE(w.extra->>'card_site_url', ''),
				COALESCE(w.extra->>'panel_session_ciphertext', ''),
				COALESCE(w.extra->'panel_session_state', '{}'::jsonb),
				w.panel_account_id, w.panel_login_identity, w.panel_login_password_ciphertext,
				w.enabled, w.balance, w.balance_updated_at, w.balance_error,
				w.created_at, w.updated_at,
				COALESCE(consumption.consumption_1h, 0), COALESCE(consumption.consumption_today, 0),
				COALESCE(consumption.consumption_24h, 0),
			COALESCE(accounts.items, '[]'::jsonb),
			COALESCE(configured_groups.items, '[]'::jsonb),
			COALESCE(actual_groups.items, '[]'::jsonb)
		FROM upstream_wallets w
		LEFT JOIN LATERAL (
			SELECT
					COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)) FILTER (WHERE ul.created_at >= NOW() - INTERVAL '1 hour'), 0) AS consumption_1h,
					COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)) FILTER (WHERE ul.created_at >= DATE_TRUNC('day', NOW())), 0) AS consumption_today,
					COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS consumption_24h
				FROM upstream_wallet_accounts uwa
				JOIN usage_logs ul ON ul.account_id = uwa.account_id
				WHERE uwa.wallet_id = w.id
				  AND ul.created_at >= NOW() - INTERVAL '24 hours'
			) consumption ON TRUE
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'id', a.id, 'name', a.name, 'platform', a.platform, 'type', a.type
			) ORDER BY a.name, a.id) AS items
			FROM upstream_wallet_accounts uwa
			JOIN accounts a ON a.id = uwa.account_id AND a.deleted_at IS NULL
			WHERE uwa.wallet_id = w.id
		) accounts ON TRUE
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object('id', grouped.id, 'name', grouped.name) ORDER BY grouped.name, grouped.id) AS items
			FROM (
				SELECT DISTINCT g.id, g.name
				FROM upstream_wallet_accounts uwa
				JOIN account_groups ag ON ag.account_id = uwa.account_id
				JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
				WHERE uwa.wallet_id = w.id
			) grouped
		) configured_groups ON TRUE
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object('id', grouped.id, 'name', grouped.name) ORDER BY grouped.name, grouped.id) AS items
			FROM (
				SELECT DISTINCT g.id, g.name
					FROM upstream_wallet_accounts uwa
					JOIN LATERAL (
						SELECT ul.group_id
						FROM usage_logs ul
						WHERE ul.account_id = uwa.account_id
						ORDER BY ul.created_at DESC, ul.id DESC
						LIMIT 1
					) latest_usage ON TRUE
					JOIN groups g ON g.id = latest_usage.group_id AND g.deleted_at IS NULL
					WHERE uwa.wallet_id = w.id
				) grouped
		) actual_groups ON TRUE
	`

	args := make([]any, 0, 2)
	conditions := []string{"w.deleted_at IS NULL"}
	if id != nil {
		args = append(args, *id)
		conditions = append(conditions, fmt.Sprintf("w.id = $%d", len(args)))
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		args = append(args, "%"+trimmed+"%")
		conditions = append(conditions, fmt.Sprintf("(w.name ILIKE $%d OR w.provider ILIKE $%d)", len(args), len(args)))
	}
	if groupID > 0 {
		args = append(args, groupID)
		placeholder := fmt.Sprintf("$%d", len(args))
		conditions = append(conditions, fmt.Sprintf(`(
			EXISTS (
				SELECT 1
				FROM upstream_wallet_accounts filter_uwa
				JOIN account_groups filter_ag ON filter_ag.account_id = filter_uwa.account_id
				WHERE filter_uwa.wallet_id = w.id AND filter_ag.group_id = %s
			)
			OR EXISTS (
				SELECT 1
				FROM upstream_wallet_accounts filter_uwa
				JOIN LATERAL (
					SELECT ul.group_id
					FROM usage_logs ul
					WHERE ul.account_id = filter_uwa.account_id
					ORDER BY ul.created_at DESC, ul.id DESC
					LIMIT 1
				) latest_usage ON latest_usage.group_id = %s
				WHERE filter_uwa.wallet_id = w.id
			)
		)`, placeholder, placeholder))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY w.enabled DESC, w.name ASC, w.id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query upstream wallets: %w", err)
	}
	defer rows.Close()

	wallets := make([]service.UpstreamWallet, 0)
	for rows.Next() {
		var wallet service.UpstreamWallet
		var balance sql.NullFloat64
		var balanceUpdatedAt sql.NullTime
		var panelAccountID sql.NullInt64
		var panelSessionJSON, accountsJSON, configuredGroupsJSON, actualGroupsJSON []byte
		if err := rows.Scan(
			&wallet.ID, &wallet.Name, &wallet.Provider, &wallet.SyncDomain, &wallet.Currency, &wallet.RechargeMode, &wallet.CardSiteURL,
			&wallet.PanelSessionCiphertext, &panelSessionJSON,
			&panelAccountID, &wallet.PanelCredentialIdentity, &wallet.PanelCredentialCiphertext,
			&wallet.Enabled, &balance, &balanceUpdatedAt, &wallet.BalanceError,
			&wallet.CreatedAt, &wallet.UpdatedAt,
			&wallet.Consumption1H, &wallet.ConsumptionToday, &wallet.Consumption24H,
			&accountsJSON, &configuredGroupsJSON, &actualGroupsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan upstream wallet: %w", err)
		}
		if balance.Valid {
			value := balance.Float64
			wallet.Balance = &value
		}
		if balanceUpdatedAt.Valid {
			value := balanceUpdatedAt.Time
			wallet.BalanceUpdatedAt = &value
		}
		if panelAccountID.Valid {
			wallet.PanelCredentialAccountID = panelAccountID.Int64
		}
		if err := json.Unmarshal(panelSessionJSON, &wallet.PanelSession); err != nil {
			return nil, fmt.Errorf("decode upstream panel session state: %w", err)
		}
		if err := json.Unmarshal(accountsJSON, &wallet.Accounts); err != nil {
			return nil, fmt.Errorf("decode wallet accounts: %w", err)
		}
		if err := json.Unmarshal(configuredGroupsJSON, &wallet.ConfiguredGroups); err != nil {
			return nil, fmt.Errorf("decode configured groups: %w", err)
		}
		if err := json.Unmarshal(actualGroupsJSON, &wallet.ActualGroups); err != nil {
			return nil, fmt.Errorf("decode actual groups: %w", err)
		}
		wallet.AccountIDs = make([]int64, 0, len(wallet.Accounts))
		for _, account := range wallet.Accounts {
			wallet.AccountIDs = append(wallet.AccountIDs, account.ID)
		}
		wallets = append(wallets, wallet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upstream wallets: %w", err)
	}
	return wallets, nil
}

func (r *upstreamFundsRepository) CreateWallet(ctx context.Context, input service.UpstreamWalletInput) (*service.UpstreamWallet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create upstream wallet: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := validateWalletAccounts(ctx, tx, 0, input.AccountIDs); err != nil {
		return nil, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
			INSERT INTO upstream_wallets (
				name, provider, currency, recharge_mode, enabled, extra
			) VALUES ($1, $2, $3, $4, $5, jsonb_build_object('card_site_url', $6::text))
			RETURNING id
		`, input.Name, input.Provider, input.Currency, input.RechargeMode, input.Enabled, input.CardSiteURL).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert upstream wallet: %w", err)
	}
	if err := replaceWalletAccounts(ctx, tx, id, input.AccountIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create upstream wallet: %w", err)
	}
	return r.GetWallet(ctx, id)
}

func (r *upstreamFundsRepository) UpdateWallet(ctx context.Context, id int64, input service.UpstreamWalletInput) (*service.UpstreamWallet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update upstream wallet: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := validateWalletAccounts(ctx, tx, id, input.AccountIDs); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE upstream_wallets
			SET name = $2, provider = $3, currency = $4, recharge_mode = $5,
				enabled = $6,
				extra = jsonb_set(extra, '{card_site_url}', to_jsonb($7::text), true), updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
			`, id, input.Name, input.Provider, input.Currency, input.RechargeMode, input.Enabled, input.CardSiteURL)
	if err != nil {
		return nil, fmt.Errorf("update upstream wallet: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		if err != nil {
			return nil, fmt.Errorf("read upstream wallet update result: %w", err)
		}
		return nil, service.ErrUpstreamWalletNotFound
	}
	if err := replaceWalletAccounts(ctx, tx, id, input.AccountIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update upstream wallet: %w", err)
	}
	return r.GetWallet(ctx, id)
}

func (r *upstreamFundsRepository) DeleteWallet(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete upstream wallet: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM upstream_wallets
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, id).Scan(&lockedID)
	if err == sql.ErrNoRows {
		return service.ErrUpstreamWalletNotFound
	}
	if err != nil {
		return fmt.Errorf("lock upstream wallet for deletion: %w", err)
	}

	var hasActiveRecharge bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM upstream_recharge_orders
			WHERE wallet_id = $1
			  AND status NOT IN ('completed', 'failed', 'expired', 'cancelled')
		)
	`, id).Scan(&hasActiveRecharge)
	if err != nil {
		return fmt.Errorf("check active upstream recharge orders: %w", err)
	}
	if hasActiveRecharge {
		return service.ErrUpstreamWalletHasActiveRecharge
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE upstream_wallets
		SET deleted_at = NOW(), enabled = FALSE,
			panel_account_id = NULL, panel_login_identity = '', panel_login_password_ciphertext = '',
			extra = COALESCE(extra, '{}'::jsonb) - 'panel_session_ciphertext' - 'panel_session_state',
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete upstream wallet: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read upstream wallet delete result: %w", err)
	}
	if affected == 0 {
		return service.ErrUpstreamWalletNotFound
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM upstream_wallet_accounts WHERE wallet_id = $1", id); err != nil {
		return fmt.Errorf("release deleted upstream wallet accounts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete upstream wallet: %w", err)
	}
	return nil
}

func (r *upstreamFundsRepository) ApplyWalletSync(
	ctx context.Context,
	plans []service.UpstreamWalletSyncPlan,
) (*service.UpstreamWalletSyncResult, error) {
	result := &service.UpstreamWalletSyncResult{}
	if len(plans) == 0 {
		return result, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin sync upstream wallets: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize catalog syncs so concurrent clicks cannot create duplicate empty wallets.
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(0x5550574C)); err != nil {
		return nil, fmt.Errorf("lock upstream wallet sync: %w", err)
	}

	for _, plan := range plans {
		domain := strings.TrimSpace(plan.Domain)
		if domain == "" {
			continue
		}
		walletID := plan.WalletID
		if walletID > 0 {
			var lockedID int64
			if err := tx.QueryRowContext(ctx, `
				SELECT id FROM upstream_wallets
				WHERE id = $1 AND deleted_at IS NULL
				FOR UPDATE
			`, walletID).Scan(&lockedID); err != nil {
				if err == sql.ErrNoRows {
					return nil, service.ErrUpstreamWalletNotFound
				}
				return nil, fmt.Errorf("lock upstream wallet for sync: %w", err)
			}
			updated, err := tx.ExecContext(ctx, `
				UPDATE upstream_wallets
				SET extra = jsonb_set(COALESCE(extra, '{}'::jsonb), '{sync_domain}', to_jsonb($2::text), true),
					updated_at = NOW()
				WHERE id = $1 AND deleted_at IS NULL
				  AND COALESCE(extra->>'sync_domain', '') IS DISTINCT FROM $2
			`, walletID, domain)
			if err != nil {
				return nil, fmt.Errorf("classify existing upstream wallet: %w", err)
			}
			classified, err := updated.RowsAffected()
			if err != nil {
				return nil, fmt.Errorf("read upstream wallet classification result: %w", err)
			}
			result.ClassifiedWallets += int(classified)
		}

		accountIDs, err := lockUnassignedWalletAccountIDs(ctx, tx, plan.AccountIDs)
		if err != nil {
			return nil, err
		}
		if walletID == 0 {
			if len(accountIDs) == 0 {
				continue
			}
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO upstream_wallets (
					name, provider, currency, recharge_mode, enabled, extra
				) VALUES (
					$1, 'sub2api', 'USD', 'direct', TRUE,
					jsonb_build_object('card_site_url', '', 'sync_domain', $1::text)
				)
				RETURNING id
			`, domain).Scan(&walletID); err != nil {
				return nil, fmt.Errorf("create domain upstream wallet: %w", err)
			}
			result.CreatedWallets++
		}
		if len(accountIDs) == 0 {
			continue
		}
		linked, err := tx.ExecContext(ctx, `
			INSERT INTO upstream_wallet_accounts (wallet_id, account_id)
			SELECT $1, account_id FROM UNNEST($2::bigint[]) AS account_id
			ON CONFLICT (account_id) DO NOTHING
		`, walletID, pq.Array(accountIDs))
		if err != nil {
			return nil, fmt.Errorf("link synced upstream wallet accounts: %w", err)
		}
		linkedCount, err := linked.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("read synced upstream account result: %w", err)
		}
		result.LinkedAccounts += int(linkedCount)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit sync upstream wallets: %w", err)
	}
	return result, nil
}

func lockUnassignedWalletAccountIDs(ctx context.Context, tx *sql.Tx, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return []int64{}, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id
		FROM accounts a
		WHERE a.id = ANY($1)
		  AND a.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM upstream_wallet_accounts uwa WHERE uwa.account_id = a.id
		  )
		ORDER BY a.id
		FOR UPDATE
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, fmt.Errorf("lock upstream accounts for sync: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, len(accountIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan upstream account for sync: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upstream accounts for sync: %w", err)
	}
	return ids, nil
}

func (r *upstreamFundsRepository) RecordBalanceSuccess(
	ctx context.Context,
	id int64,
	balance float64,
	currency string,
	source string,
) (*service.UpstreamWallet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin record upstream balance: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var walletCurrency string
	err = tx.QueryRowContext(ctx, `
		UPDATE upstream_wallets
		SET balance = $2, balance_updated_at = NOW(), balance_error = '', updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING currency
	`, id, balance).Scan(&walletCurrency)
	if err == sql.ErrNoRows {
		return nil, service.ErrUpstreamWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update upstream balance: %w", err)
	}
	if !strings.EqualFold(walletCurrency, currency) {
		return nil, fmt.Errorf("record upstream balance: snapshot currency does not match wallet currency")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_balance_snapshots (wallet_id, balance, currency, status, source)
		VALUES ($1, $2, $3, 'success', $4)
	`, id, balance, walletCurrency, source); err != nil {
		return nil, fmt.Errorf("insert upstream balance snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit upstream balance: %w", err)
	}
	return r.GetWallet(ctx, id)
}

func (r *upstreamFundsRepository) RecordBalanceFailure(
	ctx context.Context,
	id int64,
	errorSummary string,
	source string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin record upstream balance failure: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currency string
	err = tx.QueryRowContext(ctx, `
		UPDATE upstream_wallets
		SET balance_error = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING currency
	`, id, strings.TrimSpace(errorSummary)).Scan(&currency)
	if err == sql.ErrNoRows {
		return service.ErrUpstreamWalletNotFound
	}
	if err != nil {
		return fmt.Errorf("update upstream balance failure: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_balance_snapshots (
			wallet_id, balance, currency, status, error_summary, source
		) VALUES ($1, NULL, $2, 'failed', $3, $4)
	`, id, currency, strings.TrimSpace(errorSummary), source); err != nil {
		return fmt.Errorf("insert upstream balance failure snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upstream balance failure: %w", err)
	}
	return nil
}

func (r *upstreamFundsRepository) ListAccountOptions(ctx context.Context) ([]service.UpstreamFundsAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.name, a.platform, a.type, w.id, COALESCE(w.name, '')
		FROM accounts a
		LEFT JOIN upstream_wallet_accounts uwa ON uwa.account_id = a.id
		LEFT JOIN upstream_wallets w ON w.id = uwa.wallet_id AND w.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
		ORDER BY a.platform, a.name, a.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list upstream account options: %w", err)
	}
	defer rows.Close()

	items := make([]service.UpstreamFundsAccount, 0)
	for rows.Next() {
		var item service.UpstreamFundsAccount
		var walletID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Name, &item.Platform, &item.Type, &walletID, &item.Wallet); err != nil {
			return nil, fmt.Errorf("scan upstream account option: %w", err)
		}
		if walletID.Valid {
			value := walletID.Int64
			item.WalletID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func validateWalletAccounts(ctx context.Context, tx *sql.Tx, walletID int64, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM accounts WHERE deleted_at IS NULL AND id = ANY($1)
	`, pq.Array(accountIDs)).Scan(&existing); err != nil {
		return fmt.Errorf("validate upstream accounts: %w", err)
	}
	if existing != len(accountIDs) {
		return service.ErrUpstreamAccountNotFound
	}

	var assigned int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM upstream_wallet_accounts
		WHERE account_id = ANY($1) AND wallet_id <> $2
	`, pq.Array(accountIDs), walletID).Scan(&assigned); err != nil {
		return fmt.Errorf("check upstream account assignments: %w", err)
	}
	if assigned > 0 {
		return service.ErrUpstreamAccountAssigned
	}
	return nil
}

func replaceWalletAccounts(ctx context.Context, tx *sql.Tx, walletID int64, accountIDs []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM upstream_wallet_accounts WHERE wallet_id = $1", walletID); err != nil {
		return fmt.Errorf("clear upstream wallet accounts: %w", err)
	}
	if len(accountIDs) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO upstream_wallet_accounts (wallet_id, account_id)
		SELECT $1, account_id FROM UNNEST($2::bigint[]) AS account_id
	`, walletID, pq.Array(accountIDs)); err != nil {
		if isUniqueViolation(err) {
			return service.ErrUpstreamAccountAssigned
		}
		return fmt.Errorf("assign upstream wallet accounts: %w", err)
	}
	return nil
}
