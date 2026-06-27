-- 154 放宽 user_platform_quotas.platform 的 CHECK 时只含 9 个平台、漏了 grok。
-- 合并 main 后 service.AllowedQuotaPlatforms 已含 grok（共 10 个平台），否则 grok 配额
-- 写入会被 Postgres CHECK 约束拒绝。此处补全到 10 个平台，与 AllowedQuotaPlatforms 一致。
-- 先无条件 DROP 再无条件 ADD：避免 154 的 IF NOT EXISTS 守卫在约束已存在时跳过、沿用旧的 9 平台清单。
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN (
        'anthropic', 'openai', 'gemini', 'antigravity', 'grok',
        'deepseek', 'moonshot', 'glm', 'qwen', 'seedance'
    ));
