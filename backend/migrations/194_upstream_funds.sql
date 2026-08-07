-- 上游采购资金中心（独立于用户充值与用户计费）
CREATE TABLE IF NOT EXISTS upstream_wallets (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    recharge_mode VARCHAR(32) NOT NULL DEFAULT 'manual',
    tier VARCHAR(32) NOT NULL DEFAULT 'primary',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    balance NUMERIC(20, 8),
    balance_updated_at TIMESTAMPTZ,
    balance_error TEXT NOT NULL DEFAULT '',
    alert_days INTEGER NOT NULL DEFAULT 2 CHECK (alert_days >= 0),
    target_days INTEGER NOT NULL DEFAULT 7 CHECK (target_days >= 0),
    extra JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_wallets_balance_nonnegative CHECK (balance IS NULL OR balance >= 0)
);

CREATE TABLE IF NOT EXISTS upstream_wallet_accounts (
    wallet_id BIGINT NOT NULL REFERENCES upstream_wallets(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (wallet_id, account_id),
    CONSTRAINT upstream_wallet_accounts_account_unique UNIQUE (account_id)
);

CREATE TABLE IF NOT EXISTS upstream_balance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES upstream_wallets(id) ON DELETE CASCADE,
    balance NUMERIC(20, 8),
    currency VARCHAR(8) NOT NULL,
    status VARCHAR(24) NOT NULL,
    error_summary TEXT NOT NULL DEFAULT '',
    source VARCHAR(24) NOT NULL DEFAULT 'manual',
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_wallets_enabled_tier
    ON upstream_wallets (enabled DESC, tier, id);
CREATE INDEX IF NOT EXISTS idx_upstream_wallet_accounts_account
    ON upstream_wallet_accounts (account_id);
CREATE INDEX IF NOT EXISTS idx_upstream_balance_snapshots_wallet_fetched
    ON upstream_balance_snapshots (wallet_id, fetched_at DESC, id DESC);
