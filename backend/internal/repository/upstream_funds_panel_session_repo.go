package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *upstreamFundsRepository) SaveUpstreamPanelSession(
	ctx context.Context,
	walletID int64,
	ciphertext string,
	state service.UpstreamPanelSessionState,
) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal upstream panel session state: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE upstream_wallets
		SET extra = jsonb_set(
			jsonb_set(COALESCE(extra, '{}'::jsonb), '{panel_session_ciphertext}', to_jsonb($2::text), true),
			'{panel_session_state}', $3::jsonb, true
		), updated_at = NOW()
		WHERE id = $1
	`, walletID, ciphertext, string(payload))
	if err != nil {
		return fmt.Errorf("save upstream panel session: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read upstream panel session save result: %w", err)
	}
	if affected == 0 {
		return service.ErrUpstreamWalletNotFound
	}
	return nil
}

func (r *upstreamFundsRepository) CompareAndSwapUpstreamPanelSession(
	ctx context.Context,
	walletID int64,
	expectedCiphertext string,
	ciphertext string,
	state service.UpstreamPanelSessionState,
) (bool, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return false, fmt.Errorf("marshal upstream panel session state: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE upstream_wallets
		SET extra = jsonb_set(
			jsonb_set(COALESCE(extra, '{}'::jsonb), '{panel_session_ciphertext}', to_jsonb($3::text), true),
			'{panel_session_state}', $4::jsonb, true
		), updated_at = NOW()
		WHERE id = $1
		  AND COALESCE(extra->>'panel_session_ciphertext', '') = $2
	`, walletID, expectedCiphertext, ciphertext, string(payload))
	if err != nil {
		return false, fmt.Errorf("update upstream panel session: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read upstream panel session update result: %w", err)
	}
	return affected == 1, nil
}

func (r *upstreamFundsRepository) ClearUpstreamPanelSession(ctx context.Context, walletID int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE upstream_wallets
		SET extra = COALESCE(extra, '{}'::jsonb) - 'panel_session_ciphertext' - 'panel_session_state',
			updated_at = NOW()
		WHERE id = $1
	`, walletID)
	if err != nil {
		return fmt.Errorf("clear upstream panel session: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read upstream panel session clear result: %w", err)
	}
	if affected == 0 {
		return service.ErrUpstreamWalletNotFound
	}
	return nil
}

func (r *upstreamFundsRepository) ListDueUpstreamPanelSessionWalletIDs(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]int64, error) {
	if limit <= 0 {
		return []int64{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id
		FROM upstream_wallets
		WHERE enabled = TRUE
		  AND COALESCE(extra->>'panel_session_ciphertext', '') <> ''
		  AND (
			NULLIF(extra->'panel_session_state'->>'next_check_at', '') IS NULL
			OR NULLIF(extra->'panel_session_state'->>'next_check_at', '')::timestamptz <= $1
		  )
		ORDER BY COALESCE(NULLIF(extra->'panel_session_state'->>'next_check_at', '')::timestamptz, '-infinity'::timestamptz), id
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due upstream panel sessions: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan due upstream panel session: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due upstream panel sessions: %w", err)
	}
	return ids, nil
}
