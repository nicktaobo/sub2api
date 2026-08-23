package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUserPlatformQuotasCNProvidersMigration 校验该迁移把 kimi/zhipu/deepseek
// 加入 user_platform_quotas.platform 的 CHECK 约束（对照 157 号 grok 迁移）。
// 约束未放宽时，注册预填充 8 平台默认配额会整条 INSERT 中止 → 新用户零配额行
// （缺失配额行 = 无限额），管理端设置国产平台配额直接 500。
//
// 编号说明：上游原本叫 224，本 fork 在国产平台改名那轮用 git mv 重编号为 230，
// 让它排在 229_rename_national_platforms_to_upstream_naming.sql 之后执行——
// 否则它会在存量 moonshot/glm/qwen/seedance 行被清洗之前就收紧 CHECK，
// 生产库上当场 SQLSTATE 23514、迁移事务回滚、应用起不来。
// 顺序守卫见 platform_check_migration_ordering_test.go。
func TestUserPlatformQuotasCNProvidersMigration(t *testing.T) {
	content, err := FS.ReadFile("230_user_platform_quotas_add_cn_providers.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.Contains(t, sql,
		"CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek'))")
}
