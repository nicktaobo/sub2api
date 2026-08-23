package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 任何「收紧到新 8 平台白名单」的 platform CHECK 迁移，都必须排在改名迁移 229 之后。
//
// runner 用 sort.Strings（纯文件名字典序）决定执行顺序
// （internal/repository/migrations_runner.go 的 applyMigrationsFS）。
// 229 是有状态迁移：先 DROP 旧 CHECK → 把 moonshot→kimi / glm→zhipu 改名并删掉
// qwen/seedance 行 → 最后才 ADD 收敛到 8 平台的新 CHECK。任何排在它**之前**、
// 又无条件 ADD CONSTRAINT 8 平台白名单的迁移，都会在「尚未跑过 229、库里还有旧平台行」
// 的部署上当场 SQLSTATE 23514 → 事务型迁移整体回滚 → 容器起不来，
// 且永远轮不到那个能清洗数据的 229。
//
// 本轮合并上游时就踩过一次：上游的 user_platform_quotas_add_cn_providers 原编号 224，
// 字典序排在 229 之前，已 git mv 改号为 230。空库和已跑过 229 的库都测不出来
// （前者无存量行、后者数据已干净），只有这条静态断言能提前挡住。
// 同型事故本仓库另有前科：157（上游）把白名单收回 5 个平台，只能追加 158 恢复。

// knownHistoricalPlatformCheckMigrations 是已发布、受 checksum 保护而无法改号的历史例外。
var knownHistoricalPlatformCheckMigrations = map[string]bool{
	// 157 是上游当年误把白名单收回 5 个平台的那次（紧接着由 158 恢复成 10 个）。
	// 它早已在所有库执行完毕，且后面的 158 立刻放宽回去，
	// 不构成「先收紧后清洗」的危险序列。
	"157_user_platform_quotas_add_grok.sql": true,
}

func TestUserPlatformQuotaCheckMigrationsComeAfterRename(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)
	sort.Strings(names) // 与 runner 的排序方式一致

	renameIdx := -1
	for i, n := range names {
		if n == nationalPlatformRenameMigration {
			renameIdx = i
		}
	}
	require.NotEqual(t, -1, renameIdx, "找不到改名迁移 %s", nationalPlatformRenameMigration)

	checked := 0
	for i, name := range names {
		if name == nationalPlatformRenameMigration {
			continue
		}
		content, err := FS.ReadFile(name)
		require.NoError(t, err)
		sql := string(content)
		if !strings.Contains(sql, "ADD CONSTRAINT user_platform_quotas_platform_check") {
			continue
		}

		// 只看「收敛到新 8 平台」的迁移。142/154/155/157/158 是历史迁移，它们的白名单里
		// 还带着 moonshot/glm/qwen/seedance，排在 229 之前是正确的——229 的职责正是
		// 收敛它们。危险的只有那些白名单已经**不含**旧平台名、却排在 229 之前的迁移：
		// 它们会在数据清洗之前就把约束收紧。
		legacyPlatformNames := false
		for _, legacy := range []string{"'moonshot'", "'glm'", "'qwen'", "'seedance'"} {
			if strings.Contains(sql, legacy) {
				legacyPlatformNames = true
				break
			}
		}
		if legacyPlatformNames || knownHistoricalPlatformCheckMigrations[name] {
			continue
		}

		checked++
		require.Greater(t, i, renameIdx,
			"%s 重建了 user_platform_quotas 的 platform CHECK，但字典序排在改名迁移 %s 之前。\n"+
				"在尚未跑过改名迁移的库上，它会先于数据清洗执行 ADD CONSTRAINT，"+
				"撞上存量的 moonshot/glm/qwen/seedance 行（SQLSTATE 23514），"+
				"整个迁移事务回滚、应用无法启动且重启无限复现。请把它改号到 %s 之后。",
			name, nationalPlatformRenameMigration, nationalPlatformRenameMigration)
	}

	// 至少要检出上游那条被改号的迁移，否则说明匹配条件写错了、这条守卫在空转。
	require.GreaterOrEqual(t, checked, 1,
		"没有检出任何「新 8 平台白名单」迁移，守卫可能已失效")
}

// composite_model_routes 不需要同类改号，这条把「为什么不需要」钉住。
//
// 172_composite_model_routes.sql 建表时 target_platform 的 CHECK 只有
// anthropic/openai/gemini/antigravity/grok 五个具体平台，从未含国产平台，
// 因此库里不可能存在 moonshot/glm/qwen/seedance 的路由行；
// 上游的 227_composite_routes_add_cn_providers.sql 是这五个的纯超集（加 kimi/zhipu/deepseek），
// ADD CONSTRAINT 的即时校验不可能有行违约，排在 229 之前是安全的。
// 若将来有人把国产旧平台名加进 composite 的白名单，这条会红，提醒同时处理改号。
func TestCompositeRoutePlatformCheckNeedsNoRenameOrdering(t *testing.T) {
	origin, err := FS.ReadFile("172_composite_model_routes.sql")
	require.NoError(t, err)
	widened, err := FS.ReadFile("227_composite_routes_add_cn_providers.sql")
	require.NoError(t, err)

	for _, sql := range []string{string(origin), string(widened)} {
		for _, legacy := range []string{"'moonshot'", "'glm'", "'qwen'", "'seedance'"} {
			require.NotContains(t, sql, legacy,
				"composite_model_routes 的 target_platform 白名单一旦含旧国产平台名，"+
					"227 就不再是纯超集，必须像 230 那样改号到 %s 之后",
				nationalPlatformRenameMigration)
		}
	}

	// 227 必须是 172 的超集：逐个平台核对，避免「换名单」被误读成「加平台」。
	for _, p := range []string{"anthropic", "openai", "gemini", "antigravity", "grok"} {
		require.Contains(t, string(widened), "'"+p+"'",
			"227 丢了原白名单里的 %s，存量路由行会当场违约", p)
	}
}
