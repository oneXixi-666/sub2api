-- Split proactive moderation scopes without trusting textual envelopes.
-- Legacy all_groups/group_ids remain the hard-block aliases; user and context
-- observation default to all groups so non-selected groups are not skipped.

UPDATE settings
SET value = jsonb_set(
    jsonb_set(
        value::jsonb,
        '{user_observation_keywords}',
        COALESCE(value::jsonb -> 'user_observation_keywords', value::jsonb -> 'blocked_keywords', '[]'::jsonb),
        true
    ),
    '{blocked_keywords}',
    COALESCE(value::jsonb -> 'user_observation_keywords', value::jsonb -> 'blocked_keywords', '[]'::jsonb),
    true
),
updated_at = NOW()
WHERE key = 'content_moderation_config'
  AND value ~ '^\s*\{';
