-- 上游面板登录凭据由服务端加密保存；旧资金策略列保留以兼容已有数据，但应用不再读写。
ALTER TABLE upstream_wallets
    ADD COLUMN IF NOT EXISTS panel_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS panel_login_identity VARCHAR(320) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS panel_login_password_ciphertext TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_upstream_wallets_panel_probe_due
    ON upstream_wallets (enabled, id)
    WHERE COALESCE(extra->>'panel_session_ciphertext', '') <> ''
       OR panel_login_password_ciphertext <> '';
