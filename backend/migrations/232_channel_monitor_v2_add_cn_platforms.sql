-- Add the CN providers introduced after the original V2 factory config.
-- Keep operator-managed platform entries untouched; only append providers that
-- are still missing from the JSON array so existing enabled/model settings win.
UPDATE channel_monitor_v2_config
SET platforms = platforms
    || CASE WHEN NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements(platforms) AS item
        WHERE item->>'platform' = 'kimi'
    ) THEN '[{"platform":"kimi","enabled":true,"models":[]}]'::jsonb ELSE '[]'::jsonb END
    || CASE WHEN NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements(platforms) AS item
        WHERE item->>'platform' = 'zhipu'
    ) THEN '[{"platform":"zhipu","enabled":true,"models":[]}]'::jsonb ELSE '[]'::jsonb END
    || CASE WHEN NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements(platforms) AS item
        WHERE item->>'platform' = 'deepseek'
    ) THEN '[{"platform":"deepseek","enabled":true,"models":[]}]'::jsonb ELSE '[]'::jsonb END,
    version = version + 1,
    updated_at = NOW()
WHERE id = 1
  AND (
      NOT EXISTS (SELECT 1 FROM jsonb_array_elements(platforms) AS item WHERE item->>'platform' = 'kimi')
      OR NOT EXISTS (SELECT 1 FROM jsonb_array_elements(platforms) AS item WHERE item->>'platform' = 'zhipu')
      OR NOT EXISTS (SELECT 1 FROM jsonb_array_elements(platforms) AS item WHERE item->>'platform' = 'deepseek')
  );
