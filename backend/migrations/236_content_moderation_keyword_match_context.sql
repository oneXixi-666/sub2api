ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS matched_role VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS matched_source VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS matched_start INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS matched_end INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_keyword_observe
    ON content_moderation_logs (created_at DESC, id DESC)
    WHERE action = 'keyword_observe';
