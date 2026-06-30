-- 157_user_platform_quotas_add_grok.sql（来自上游 main）把 user_platform_quotas.platform
-- 的 CHECK 重置为仅 5 个平台（anthropic/openai/gemini/antigravity/grok）——因为上游 main
-- 没有本 fork 的国产平台。但 local_dev 的 service.AllowedQuotaPlatforms 含国产 5 平台
-- (deepseek/moonshot/glm/qwen/seedance)，被 157 覆盖后国产平台配额写入会被 CHECK 拒绝
-- （等于回退了本仓库 155_user_platform_quota_add_grok_check.sql 的修复）。
-- 迁移按文件名排序执行，157 在 155 之后跑、把约束改回 5 平台；此处 158 在 157 之后再恢复
-- 为完整 10 平台清单，与 AllowedQuotaPlatforms 对齐。
-- DROP IF EXISTS 保证可重入；新约束是 157（5 平台）的超集，存量行瞬时校验通过。
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN (
        'anthropic', 'openai', 'gemini', 'antigravity', 'grok',
        'deepseek', 'moonshot', 'glm', 'qwen', 'seedance'
    ));
