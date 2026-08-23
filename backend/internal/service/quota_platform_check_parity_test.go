package service

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/userplatformquota"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"

	// ent 的字段校验器（含 platform 的 Validate 闭包）由 ent/runtime 的 init 注入，
	// 不 import 它 userplatformquota.PlatformValidator 会是 nil。
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
)

// 平台白名单在本仓库有三个源，任意两处漂移都会变成线上静默故障：
//   1. service.AllowedQuotaPlatforms —— 运行时唯一权威源（后台校验、注册预填都读它）；
//   2. user_platform_quotas.platform 的 DB CHECK —— 终态由迁移
//      229_rename_national_platforms_to_upstream_naming.sql 收敛（230 重建同一份名单）；
//   3. ent/schema/user_platform_quota.go 的 Validate —— 构建期约束，生成到
//      ent/userplatformquota.PlatformValidator。
//
// 漂移的典型后果：后台校验通过 → INSERT 撞 DB CHECK → 注册路径 fail-open 吞错 →
// 新用户拿到**零条**配额行 = 全平台无限额。这类事故本仓库已经发生过四次
// （150 / 155 / 157 / 224 各修一次，157 是上游误把白名单收回 5 个平台、158 才恢复）。
//
// migrations 包里的 TestMigration229FinalPlatformWhitelist 只能断言 SQL 里出现了某几个
// 平台名——它把同一份名单硬编码了第二遍，因此挡不住漂移：谁往 AllowedQuotaPlatforms
// 加第 9 个平台，那边照样绿。这里从三侧的**真源**取值比对。

const nationalPlatformRenameMigration = "229_rename_national_platforms_to_upstream_naming.sql"

func TestQuotaPlatformCheckMatchesAllowedQuotaPlatforms(t *testing.T) {
	sqlPlatforms := parsePlatformCheckWhitelist(t, nationalPlatformRenameMigration)

	goPlatforms := append([]string(nil), AllowedQuotaPlatforms...)
	sort.Strings(goPlatforms)

	require.Equal(t, goPlatforms, sqlPlatforms,
		"AllowedQuotaPlatforms 与迁移 %s 的 CHECK 名单不一致。\n"+
			"新增/删除平台时必须同时追加一个收紧 CHECK 的新迁移——"+
			"已发布迁移受 checksum 保护，不能原地修改。", nationalPlatformRenameMigration)
}

// 230（上游原 224，本 fork 改号排到 229 之后）重建的是同一份名单，
// 两份 CHECK 必须逐字一致，否则后跑的那个会把前一个的收敛结果改掉。
func TestQuotaPlatformCheckIsConsistentAcrossMigrations(t *testing.T) {
	require.Equal(t,
		parsePlatformCheckWhitelist(t, nationalPlatformRenameMigration),
		parsePlatformCheckWhitelist(t, "230_user_platform_quotas_add_cn_providers.sql"),
		"229 与 230 的 platform CHECK 名单必须一致：230 排在后面，它才是最终生效的那份")
}

// ent 的构建期校验器必须与 AllowedQuotaPlatforms 完全同集：
// 松了就写得进 DB CHECK 拒绝的值，紧了就在 ent 层先炸、错误信息还指不到平台白名单。
func TestEntPlatformValidatorMatchesAllowedQuotaPlatforms(t *testing.T) {
	require.NotNil(t, userplatformquota.PlatformValidator,
		"PlatformValidator 为 nil：ent/runtime 未被 import，校验器没注入")

	for _, p := range AllowedQuotaPlatforms {
		require.NoError(t, userplatformquota.PlatformValidator(p),
			"ent schema 的 Validate 拒绝了 AllowedQuotaPlatforms 里的 %s", p)
	}

	// 已改名 / 已下线的旧标识必须被拒绝，否则它们还能被写进库、
	// 下次收紧 CHECK 的迁移就会在存量行上失败。
	for _, gone := range []string{"moonshot", "glm", "qwen", "seedance"} {
		require.Error(t, userplatformquota.PlatformValidator(gone),
			"ent schema 的 Validate 仍接受已下线平台 %s", gone)
	}
	require.Error(t, userplatformquota.PlatformValidator("not-a-platform"))
}

// AllowedQuotaPlatforms 本身不得有重复项。
// 改名那轮 domain 常量里 PlatformDeepSeek 与 PlatformDeepseek 同为 "deepseek"，
// 两个都留在切片里就会出现重复——注册预填会为 deepseek 写两行，
// 撞 userplatformquota_user_id_platform_uq 部分唯一索引，整条 BulkInsertInitial 中止，
// fail-open 之后新用户一条配额行都没有 = 全平台无限额。
func TestAllowedQuotaPlatformsHasNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, len(AllowedQuotaPlatforms))
	for _, p := range AllowedQuotaPlatforms {
		_, dup := seen[p]
		require.False(t, dup, "AllowedQuotaPlatforms 含重复平台 %q", p)
		seen[p] = struct{}{}
	}
}

// parsePlatformCheckWhitelist 从迁移 SQL 里解析
// `ADD CONSTRAINT user_platform_quotas_platform_check ... CHECK (platform IN (...))`
// 的平台列表，返回排序后的切片。
func parsePlatformCheckWhitelist(t *testing.T, migrationName string) []string {
	t.Helper()

	content, err := migrations.FS.ReadFile(migrationName)
	require.NoError(t, err, "读取迁移 %s 失败", migrationName)
	sql := string(content)

	idx := strings.Index(sql, "ADD CONSTRAINT user_platform_quotas_platform_check")
	require.NotEqual(t, -1, idx, "%s 里找不到收紧后的 CHECK", migrationName)

	tail := sql[idx:]
	open := strings.Index(tail, "CHECK (platform IN (")
	require.NotEqual(t, -1, open, "%s 的 CHECK 子句形态与预期不符", migrationName)
	rest := tail[open+len("CHECK (platform IN ("):]
	end := strings.Index(rest, ")")
	require.NotEqual(t, -1, end, "%s 的 CHECK 平台列表没有闭合括号", migrationName)

	matches := regexp.MustCompile(`'([a-z0-9_]+)'`).FindAllStringSubmatch(rest[:end], -1)
	require.NotEmpty(t, matches, "%s 的 CHECK 里没解析出任何平台", migrationName)

	platforms := make([]string, 0, len(matches))
	for _, m := range matches {
		platforms = append(platforms, m[1])
	}
	sort.Strings(platforms)
	return platforms
}
