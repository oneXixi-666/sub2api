-- Preserve bounded, reviewable request evidence for future cyber_policy hits.
-- Existing rows keep empty/default values and therefore remain explicitly
-- distinguishable from newly captured evidence.

ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS input_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS input_hash VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS input_length INT NOT NULL DEFAULT 0;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS message_count INT NOT NULL DEFAULT 0;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS input_truncated BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS protocol VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS audit_stage VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS turn_number INT NOT NULL DEFAULT 0;
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS cyber_policy_mode VARCHAR(16) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_cyber_input_hash
    ON content_moderation_logs(input_hash)
    WHERE action = 'cyber_policy' AND input_hash <> '';
