-- 上游钱包使用软删除释放账号关联，同时保留充值订单与资金快照历史。
ALTER TABLE upstream_wallets
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_upstream_wallets_active
    ON upstream_wallets (enabled DESC, name, id)
    WHERE deleted_at IS NULL;
