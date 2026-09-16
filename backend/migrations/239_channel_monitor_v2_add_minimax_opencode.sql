-- Append MiniMax / OpenCode to channel_monitor_v2_config.platforms.
-- Keep operator-managed platform entries untouched; only append providers that
-- are still missing from the JSON array so existing enabled/model settings win.
UPDATE channel_monitor_v2_config
SET platforms = platforms
    || CASE WHEN NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements(platforms) AS item
        WHERE item->>'platform' = 'minimax'
    ) THEN '[{"platform":"minimax","enabled":true,"models":[]}]'::jsonb ELSE '[]'::jsonb END
    || CASE WHEN NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements(platforms) AS item
        WHERE item->>'platform' = 'opencode_go'
    ) THEN '[{"platform":"opencode_go","enabled":true,"models":[]}]'::jsonb ELSE '[]'::jsonb END,
    version = version + 1,
    updated_at = NOW()
WHERE id = 1
  AND (
      NOT EXISTS (SELECT 1 FROM jsonb_array_elements(platforms) AS item WHERE item->>'platform' = 'minimax')
      OR NOT EXISTS (SELECT 1 FROM jsonb_array_elements(platforms) AS item WHERE item->>'platform' = 'opencode_go')
  );
