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
		t.Fatal("expected v16 auth snapshot to be rejected: local and upstream v16 have different meanings, merged snapshot is v17")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

// 版本号必须严格大于两条血脉各自的 v16，否则旧缓存会被误判有效。
func TestAPIKeyAuthSnapshotVersion_IsPastBothV16Lineages(t *testing.T) {
	if apiKeyAuthSnapshotVersion <= 16 {
		t.Fatalf("apiKeyAuthSnapshotVersion must be > 16 after merging the two conflicting v16 lineages, got %d", apiKeyAuthSnapshotVersion)
	}
}
