-- 放宽 user_platform_quotas.platform 的 CHECK：一次性含全部 9 个平台。
-- 平台列表与 backend/internal/service/domain_constants.go 的 AllowedQuotaPlatforms 保持一致。
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'user_platform_quotas_platform_check'
          AND conrelid = 'user_platform_quotas'::regclass
    ) THEN
        ALTER TABLE user_platform_quotas
            ADD CONSTRAINT user_platform_quotas_platform_check
            CHECK (platform IN (
                'anthropic', 'openai', 'gemini', 'antigravity',
                'deepseek', 'moonshot', 'glm', 'qwen', 'seedance'
            ));
    END IF;
END $$;
