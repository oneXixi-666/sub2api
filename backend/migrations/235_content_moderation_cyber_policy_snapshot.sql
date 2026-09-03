-- Persist the effective cyber policy used for every upstream cyber_policy hit.
-- Historical rows retain an empty snapshot because their exact policy cannot be
-- reconstructed after configuration changes.
ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS cyber_policy_source VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS cyber_policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_cyber_group_count
    ON content_moderation_logs (user_id, group_id, created_at DESC)
    WHERE action = 'cyber_policy'
      AND flagged = TRUE
      AND cyber_policy_mode = 'enforce';
