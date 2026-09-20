-- 通用设置：前台语言包与展示货币。
-- 缺省值与历史硬编码行为一致（en/zh/fr/ru、en、CNY、¥），已有 key 不覆盖。
INSERT INTO settings (key, value, updated_at)
VALUES
    ('display_locales', '["en","zh","fr","ru"]', NOW()),
    ('default_locale', 'en', NOW()),
    ('display_currency', 'CNY', NOW()),
    ('display_currency_symbol', '¥', NOW())
ON CONFLICT (key) DO NOTHING;
