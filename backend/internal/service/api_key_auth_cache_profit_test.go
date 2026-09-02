package service

// 投影漏列回归（service 半程）：认证快照 build → L2 JSON 序列化
// → 反序列化 → 还原 apiKey.Group → 请求 ctx → 利润门解析，全链路保真。
// repository 半程（真实 GetByKeyForAuth 投影）见
// internal/repository/api_key_repo_profit_projection_integration_test.go。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func profitAuthTestAPIKey() *APIKey {
	groupID := int64(50)
	return &APIKey{
		ID:      82,
		UserID:  40,
		GroupID: &groupID,
		Name:    "profit-auth-roundtrip",
		Status:  StatusActive,
		User: &User{
			ID:          40,
			Email:       "profit@test.local",
			Status:      StatusActive,
			Concurrency: 5,
		},
		Group: &Group{
			ID:                   groupID,
			Name:                 "VIP-roundtrip",
			Platform:             PlatformOpenAI,
			Status:               StatusActive,
			Hydrated:             true,
			RateMultiplier:       0.06,
			SubscriptionType:     SubscriptionTypeStandard,
			PeakRateEnabled:      false,
			ProfitControlEnabled: true,
			ProfitMinMargin:      0.2,
			ProfitSafetyBuffer:   0.05,
		},
	}
}

// 快照构建 → L2 JSON 往返 → 还原 → 装门：利润字段必须全程保真，阈值与
// 计费同源（0.06 × (1−0.25) = 0.045）。
func TestAPIKeyAuthSnapshotProfitControlRoundtrip(t *testing.T) {
	svc := &APIKeyService{}
	apiKey := profitAuthTestAPIKey()

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	// 上游此处原本硬编码 19；合并到本 fork 时 v19 已是第五次同号异义（本地 v19 =
	// merchant 字段 + reasoning effort + AllowLive + profit control，上游 v19 =
	// 分组级 search/audio/video_model_prices 计费字段），故常量抬到 20，0.1.176 补
	// LongContextPricingEnabled/ModelPricing 后又抬到 21。本用例只验「快照写的就是当前常量」，
	// 不写死数字；版本下界的守卫在 TestAPIKeyAuthSnapshotVersion_IsPastAllCollidedLineages，
	// 各代旧快照必须被拒的守卫在同文件的 TestAPIKeyService_RejectsV*AuthSnapshot* 系列。
	//
	// 合并 0.1.179 时上游又在这里塞了一句写死的
	// `require.Equal(t, 20, snapshot.Version, "v20 起认证快照携带分组长上下文与模型定价字段")`
	// ——上游独立修了同一个 bug 并 bump 到 20，而本 fork 早已是 21（且字段已在）。
	// 那句写死断言在本 fork 恒失败，且它想守的语义（快照必须带分组长上下文/模型定价字段）
	// 由 api_key_auth_snapshot_group_parity_test.go 的反射 parity 守卫覆盖得更严，
	// 故**丢弃写死数字那句**，只保留跟常量走的断言。下轮合并再冒出来同样处理。
	//
	// 合并 0.1.185 时上游第二次塞了同类写死断言
	// `require.Equal(t, 22, snapshot.Version, "v22 起认证快照携带分组免费 Fast 开关")`
	// ——上游本轮连升两级（v21 = force_openai_fast，v22 = free_openai_fast），
	// 而合并后的快照是两侧字段并集、常量已抬到 23，那句同样恒失败。按既定做法丢弃。
	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)

	// 模拟 L2 缓存的完整 JSON 往返（与 apiKeyCache.SetAuthCache/GetAuthCache 同构）。
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var restored APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &restored))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &restored)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.Hydrated)
	require.True(t, materialized.Group.ProfitControlEnabled)
	require.InDelta(t, 0.2, materialized.Group.ProfitMinMargin, 1e-12)
	require.InDelta(t, 0.05, materialized.Group.ProfitSafetyBuffer, 1e-12)
	require.InDelta(t, 0.06, materialized.Group.RateMultiplier, 1e-12)

	// 中间件语义：materialized.Group 进请求 ctx → 门必须按快照配置装上。
	ctx := context.WithValue(context.Background(), ctxkey.Group, materialized.Group)
	gwSvc := &OpenAIGatewayService{}
	gate := gwSvc.resolveOpenAIProfitControlGate(ctx, materialized.GroupID)
	require.NotNil(t, gate, "还原后的认证分组必须能装门（投影漏列时本断言最先失败）")
	require.InDelta(t, 0.06*(1-0.25), gate.threshold, 1e-12)
}

// 旧版本快照（v16 及更早，无利润字段保真保证）必须被淘汰回源，不得复用。
func TestAPIKeyAuthSnapshotOldVersionEvicted(t *testing.T) {
	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), profitAuthTestAPIKey())
	require.NotNil(t, snapshot)
	snapshot.Version = 16

	materialized, used, err := svc.applyAuthCacheEntry("sk-old", &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.False(t, used, "版本不匹配的缓存条目必须淘汰并回源重建")
	require.Nil(t, materialized)
}
