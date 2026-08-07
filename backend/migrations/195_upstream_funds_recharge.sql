-- 上游资金中心充值商品、订单与不可变状态事件
CREATE TABLE IF NOT EXISTS upstream_recharge_products (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES upstream_wallets(id) ON DELETE CASCADE,
    external_ref VARCHAR(128) NOT NULL DEFAULT '',
    name VARCHAR(160) NOT NULL,
    face_value NUMERIC(20, 8) NOT NULL CHECK (face_value > 0),
    pay_amount NUMERIC(20, 8) NOT NULL CHECK (pay_amount >= 0),
    currency VARCHAR(8) NOT NULL,
    stock INTEGER,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (wallet_id, external_ref)
);

CREATE TABLE IF NOT EXISTS upstream_recharge_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(40) NOT NULL UNIQUE,
    idempotency_key VARCHAR(128) NOT NULL,
    wallet_id BIGINT NOT NULL REFERENCES upstream_wallets(id) ON DELETE RESTRICT,
    product_id BIGINT REFERENCES upstream_recharge_products(id) ON DELETE SET NULL,
    provider_order_id VARCHAR(128) NOT NULL DEFAULT '',
    payment_channel_id VARCHAR(64) NOT NULL,
    face_value NUMERIC(20, 8) NOT NULL CHECK (face_value > 0),
    pay_amount NUMERIC(20, 8) NOT NULL CHECK (pay_amount >= 0),
    currency VARCHAR(8) NOT NULL,
    status VARCHAR(32) NOT NULL,
    payment_qr TEXT NOT NULL DEFAULT '',
    payment_url TEXT NOT NULL DEFAULT '',
    payment_expires_at TIMESTAMPTZ,
    balance_before NUMERIC(20, 8),
    balance_after NUMERIC(20, 8),
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE (wallet_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS upstream_recharge_order_events (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES upstream_recharge_orders(id) ON DELETE CASCADE,
    from_status VARCHAR(32) NOT NULL DEFAULT '',
    to_status VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_recharge_products_wallet_enabled
    ON upstream_recharge_products (wallet_id, enabled DESC, id);
CREATE INDEX IF NOT EXISTS idx_upstream_recharge_orders_wallet_created
    ON upstream_recharge_orders (wallet_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_recharge_orders_status
    ON upstream_recharge_orders (status, updated_at, id);
CREATE INDEX IF NOT EXISTS idx_upstream_recharge_events_order
    ON upstream_recharge_order_events (order_id, id);
