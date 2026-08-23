package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const nationalPlatformRenameMigration = "229_rename_national_platforms_to_upstream_naming.sql"

// 229 把国产平台标识改成上游命名（moonshot→kimi、glm→zhipu，qwen/seedance 下线）。
// 迁移一旦发布就被 checksum 锁死、无法原地修（见 internal/repository/migrations_runner.go），
// 因此这里把最危险的几条性质钉在 CI 里。

// 数据转换必须夹在「DROP 旧 CHECK」和「ADD 新 CHECK」之间——两个方向都会让部署直接躺下，
// 而且**空库一律测不出来**（新库没有旧平台行，所有 UPDATE 影响 0 行、一路全绿）。
func TestMigration229OrdersDataConversionBetweenDropAndAddConstraint(t *testing.T) {
	sql := readRenameMigration(t)

	addConstraint := strings.Index(sql, "ADD CONSTRAINT user_platform_quotas_platform_check")
	require.NotEqual(t, -1, addConstraint, "收紧后的 CHECK 必须存在")

	dropConstraint := strings.Index(sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.NotEqual(t, -1, dropConstraint, "重跑安全要求 DROP ... IF EXISTS")
	require.Less(t, dropConstraint, addConstraint)

	// 每条数据转换语句都必须落在 DROP 之后、ADD 之前。
	conversions := []string{
		"UPDATE accounts SET platform = 'kimi'",
		"UPDATE accounts SET platform = 'zhipu'",
		"UPDATE groups SET platform = 'kimi'",
		"UPDATE groups SET platform = 'zhipu'",
		"UPDATE user_platform_quotas",
		"DELETE FROM user_platform_quotas WHERE platform IN ('qwen', 'seedance')",
	}
	for _, stmt := range conversions {
		idx := strings.Index(sql, stmt)
		require.NotEqual(t, -1, idx, "缺少数据转换语句: %s", stmt)

		// 方向 A：ADD CONSTRAINT 会立即校验存量行。任何一条转换排在它之后，
		// 存量 moonshot/glm/qwen/seedance 行都会让整个迁移事务回滚 → 应用起不来。
		require.Less(t, idx, addConstraint,
			"数据转换必须排在 ADD CONSTRAINT 之前，否则迁移回滚、启动失败: %s", stmt)

		// 方向 B（同样致命）：旧 CHECK 在转换时仍然在位，而 158 留下的白名单里没有
		// kimi / zhipu，第一条 `SET platform = 'kimi'` 就会撞 SQLSTATE 23514。
		require.Greater(t, idx, dropConstraint,
			"旧 CHECK 必须在任何数据转换之前卸掉：它的白名单里没有 kimi/zhipu，"+
				"改名 UPDATE 会当场违约(23514)、整个迁移回滚、应用起不来: %s", stmt)
	}

	// 事务型迁移：不得含 CONCURRENTLY（那需要 _notx 后缀，见 validateMigrationExecutionMode）。
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
	require.False(t, strings.HasSuffix(nationalPlatformRenameMigration, "_notx.sql"))
}

// 终态白名单 = 8 个平台，且收紧后的 CHECK 里不得再出现下线/改名前的标识。
// 与 service.AllowedQuotaPlatforms 的**逐字**比对在
// internal/service/quota_platform_check_parity_test.go（那条才挡得住名单漂移）。
func TestMigration229FinalPlatformWhitelist(t *testing.T) {
	sql := readRenameMigration(t)

	addConstraint := strings.Index(sql, "ADD CONSTRAINT user_platform_quotas_platform_check")
	require.NotEqual(t, -1, addConstraint)
	tail := sql[addConstraint:]

	for _, p := range []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek"} {
		require.Contains(t, tail, "'"+p+"'", "终态白名单缺少平台 %s", p)
	}
	for _, gone := range []string{"'moonshot'", "'glm'", "'qwen'", "'seedance'"} {
		require.NotContains(t, tail, gone, "收紧后的 CHECK 仍含旧平台标识 %s", gone)
	}

	// 预检必须存在：ADD CONSTRAINT 的裸约束冲突信息看不出是哪个平台残留。
	require.Contains(t, sql, "RAISE EXCEPTION",
		"收紧前必须有预检，把残留平台值变成能看懂的报错")
}

// 覆盖面：所有存平台标识的表都要改到，漏一张就是「旧名残留 + 新代码读不到」。
// 表名与列名均已对照本仓库 migrations 核实（channels 无 platform 列，见下一条测试）。
func TestMigration229CoversEveryPlatformBearingTable(t *testing.T) {
	sql := readRenameMigration(t)

	// 业务/配置表：直接改 platform 列或 JSON 里的平台键。
	for _, needle := range []string{
		"UPDATE accounts SET platform",
		"UPDATE groups SET platform",
		"UPDATE channel_model_pricing SET platform",
		"UPDATE channel_account_stats_model_pricing SET platform",
		"UPDATE channels\nSET model_mapping", // 只改外层平台键
		"UPDATE user_platform_quotas",        // 改名 + 删 qwen/seedance
		"FROM settings",                      // default_platform_quotas / auth_source_default_%_platform_quotas
		"UPDATE error_passthrough_rules",     // platforms JSONB 数组
		"UPDATE channel_monitor_v2_config c", // platforms JSONB 对象数组
		"UPDATE ops_alert_rules",             // filters->>'platform'
	} {
		require.Contains(t, sql, needle, "迁移漏改了: %s", needle)
	}

	// settings 的两类 key 都要覆盖：只改系统层会让各登录来源的默认配额继续写入旧平台。
	require.Contains(t, sql, "'default_platform_quotas'")
	require.Contains(t, sql, `'auth\_source\_default\_%\_platform\_quotas'`)

	// 观测 / 历史表：只改名不删行，逐表在 DO 块里处理。
	for _, tbl := range []string{
		"ops_metrics_hourly",
		"ops_metrics_daily",
		"ops_error_logs",
		"ops_system_metrics",
		"ops_system_logs",
		"ops_alert_silences",
		"channel_monitor_v2_metrics_1m",
		"channel_monitor_v2_user_metrics_1m",
		"channel_monitor_v2_error_metrics_1m",
		"channel_monitor_v2_latency_histograms_1m",
		"channel_monitor_v2_metrics_rollup",
		"channel_monitor_v2_user_metrics_rollup",
		"channel_monitor_v2_error_metrics_rollup",
		"channel_monitor_v2_latency_histograms_rollup",
	} {
		require.Contains(t, sql, "'"+tbl+"'", "观测表改名列表漏了 %s", tbl)
	}
	// 这些表带含 platform 的主键/唯一索引，混合数据上重跑可能撞键；
	// 撞键时跳过该表而不是让启动失败。
	require.Contains(t, sql, "EXCEPTION WHEN unique_violation")
}

// 本仓库特有：channels 表**没有** platform 列（081_create_channels.sql 建表 + 后续 ALTER 均无），
// 平台维度只落在 channel_model_pricing.platform（086 新增）。
// 参照工程（/Project/sub2api dev 的 226）有 `UPDATE channels SET platform = ...`，
// 照抄过来会在运行时报 "column platform of relation channels does not exist"，
// 事务型迁移整体回滚 → 应用起不来。这条专门挡住「下次同步时把它抄回来」。
func TestMigration229DoesNotTouchNonexistentChannelsPlatformColumn(t *testing.T) {
	executable := stripSQLLineComments(readRenameMigration(t))
	require.NotContains(t, executable, "UPDATE channels SET platform",
		"本仓库 channels 表没有 platform 列，这条语句会让迁移整体回滚")
}

// 边界：只改平台标识，绝不碰模型名。
func TestMigration229DoesNotRewriteModelNames(t *testing.T) {
	// 只看可执行语句：注释里为了说明「平台标识 vs 模型名」的区别会举模型名的例子。
	executable := stripSQLLineComments(readRenameMigration(t))

	// 模型名 / 探测器名字面量不得出现在任何转换语句里。
	// qwen3guard-openai 是 181_prompt_audit.sql 的 scanner_backend 默认值——
	// 名字里带 qwen，但它是模型不是平台，绝不能被平台改名波及。
	for _, modelish := range []string{
		"moonshot-v1", "kimi-k2", "kimi-k3", "glm-4", "glm-5",
		"qwen-max", "qwen-plus", "qwen3guard", "seedance-1",
	} {
		require.NotContains(t, executable, modelish,
			"迁移不得引用模型名 %s——本迁移只改平台标识", modelish)
	}

	// channels.model_mapping 是 {平台: {源模型: 目标模型}}：只允许整体搬迁外层键，
	// 内层 map 必须原样取值（-> 而不是任何逐键改写）。
	require.Contains(t, executable, "jsonb_build_object('kimi', model_mapping -> 'moonshot')")
	require.Contains(t, executable, "jsonb_build_object('zhipu', model_mapping -> 'glm')")

	// 计价表只改 platform 列，models 数组（模型名）不能出现在 SET 子句里。
	require.NotContains(t, executable, "channel_model_pricing SET models")
	require.NotContains(t, executable, "channel_account_stats_model_pricing SET models")
	// channel_monitor_v2_config 只改每项的 "platform" 字段，"models" 不动。
	require.NotContains(t, executable, `'{models}'`)
}

// qwen/seedance 的残留账号与分组必须**停用**而非删除。
//
// 摘掉 OpenAI 兼容分支后它们会 fall through 到 Anthropic 网关
// （matchingPlatforms('qwen') 原样返回 ['qwen']，没有平台白名单兜底），
// Account.GetBaseURL() 在 credentials.base_url 为空时默认回落 https://api.anthropic.com
// → 百炼/火山的 Key 会被当 x-api-key 发出去 = 凭证外泄。
// 停用可逆、删除不可逆，账号/分组还挂着用量与外键。
func TestMigration229DisablesRatherThanDeletesRetiredPlatformRows(t *testing.T) {
	executable := stripSQLLineComments(readRenameMigration(t))

	require.Contains(t, executable, "UPDATE accounts\nSET status = 'disabled'")
	require.Contains(t, executable, "UPDATE groups\nSET status = 'disabled'")
	require.Contains(t, executable, "WHERE platform IN ('qwen', 'seedance')")

	require.NotContains(t, executable, "DELETE FROM accounts",
		"qwen/seedance 账号必须停用而非删除：删除不可逆且会撞外键")
	require.NotContains(t, executable, "DELETE FROM groups",
		"qwen/seedance 分组必须停用而非删除：删除不可逆且会撞外键")

	// 只动 active 行，重跑为空操作。
	require.Contains(t, executable, "AND status = 'active'")
}

// error_passthrough_rules.platforms 的空数组陷阱：
// platformMatchesCached() 里 len(lowerPlatforms) == 0 表示**匹配所有平台**，不是「不匹配」。
// 剔除 qwen/seedance 后若聚合结果为空，必须回落到原值而不是 '[]'——
// 否则一条只作用于已下线平台的规则会静默升级成全平台生效，
// 改变 Anthropic/OpenAI 等主力平台的错误透传与故障转移行为。
func TestMigration229KeepsEmptyPlatformArrayFallbackSafe(t *testing.T) {
	sql := readRenameMigration(t)

	epr := strings.Index(sql, "UPDATE error_passthrough_rules")
	require.NotEqual(t, -1, epr)
	next := strings.Index(sql[epr:], "-- 2.3")
	require.NotEqual(t, -1, next, "找不到 2.2 段落的结尾")
	block := sql[epr : epr+next]

	require.NotContains(t, block, "'[]'::jsonb",
		"platforms 聚合为空时不得回落 '[]'：空数组在服务端语义是「匹配所有平台」")
	require.Contains(t, block, "        platforms\n    ),",
		"COALESCE 的兜底必须是原值 platforms")
}

func readRenameMigration(t *testing.T) string {
	t.Helper()
	content, err := FS.ReadFile(nationalPlatformRenameMigration)
	require.NoError(t, err, "读取迁移 %s 失败", nationalPlatformRenameMigration)
	return string(content)
}

// stripSQLLineComments 去掉整行 `--` 注释，只留可执行 SQL。
// 本迁移不含行内 `--`（字符串字面量里也没有），因此逐行前缀判断即可。
func stripSQLLineComments(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
