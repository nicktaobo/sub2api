package service

import "testing"

func TestAPIKeyService_RejectsV10AuthSnapshotWithoutModelsListConfig(t *testing.T) {
	groupID := int64(9)
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-models-list", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  10,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:               groupID,
				Name:             "openai",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1,
			},
		},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatalf("expected v10 auth snapshot to be rejected after models_list_config was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

// v14 快照缺 user.parent_merchant_id：若不整体拒绝，L2 Redis 中最长存活一个 TTL 窗口的
// 旧快照反序列化后 ParentMerchantID 恒为 nil，merchant suspended 拦截与 batch_image
// merchant 子用户守卫在灰度窗口内会被静默绕过（回归场景）。
func TestAPIKeyService_RejectsV14AuthSnapshotWithoutParentMerchantID(t *testing.T) {
	groupID := int64(9)
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-parent-merchant", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  14,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:               groupID,
				Name:             "openai",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1,
			},
		},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatalf("expected v14 auth snapshot to be rejected after parent_merchant_id was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_RejectsV15AuthSnapshotWithoutReasoningEffortPolicy(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-reasoning-mappings", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 15},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatal("expected v15 auth snapshot to be rejected after reasoning effort policy was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

// v16 曾被两条独立血脉同时使用、语义不同：本地 v16 = user.parent_merchant_id +
// group.affiliate_rebate_excluded，上游 v16 = group reasoning effort ceiling/mappings。
// 合并后快照同时含两套字段，必须整体拒绝任何 v16 条目，否则灰度窗口内命中另一血脉写的
// v16 缓存会通过版本校验、缺失字段读零值（merchant 停用守卫失效 / reasoning 策略读空）。
func TestAPIKeyService_RejectsV16AuthSnapshotFromEitherPreMergeLineage(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-v16", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 16},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatal("expected v16 auth snapshot to be rejected: local and upstream v16 have different meanings, merged snapshot is past v16")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

// v17 是第三次同号异义：本地 v17（合并两条 v16 血脉的产物）与上游 v17（group Live gate）
// 含义不同，合并后升到 18，故 v17 条目同样必须整体拒绝。
func TestAPIKeyService_RejectsV17AuthSnapshotFromEitherLineage(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-v17", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 17},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatal("expected v17 auth snapshot to be rejected: local v17 (merged v16 lineages) and upstream v17 (group Live gate) have different meanings")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

// 版本号必须严格大于所有已知的撞号版本（v16 两条血脉、v17 两条血脉），否则旧缓存会被误判有效。
// 上游每给 group 加一个进快照的字段就 bump 一次，本 fork 也在加，撞号已连续三轮；下轮合并
// 若上游再 bump 到 18，这里要继续抬高，不能沿用。
func TestAPIKeyAuthSnapshotVersion_IsPastAllCollidedLineages(t *testing.T) {
	const lastCollidedVersion = 17
	if apiKeyAuthSnapshotVersion <= lastCollidedVersion {
		t.Fatalf("apiKeyAuthSnapshotVersion must be > %d after merging the conflicting v16/v17 lineages, got %d", lastCollidedVersion, apiKeyAuthSnapshotVersion)
	}
}
