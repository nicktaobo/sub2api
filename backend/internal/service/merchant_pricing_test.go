// MERCHANT-SYSTEM v1.0
// MerchantPricingService 单元测试（不依赖 DB）：
//   - feature flag off → empty result
//   - billing type != balance → empty
//   - base cost ≤ 0 → empty
//   - owner 路径返回 MerchantLedger draft
//   - sub_user + suspended → empty
//   - sub_user + markup=1 → empty
//   - sub_user + markup>1 → BalanceCostOverride + MerchantOutbox
//   - 缓存 invalidation

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// fakeMerchantRepo 仅实现 LookupMerchantIDForUser + LoadPricing 用于 pricing 测试。
type fakeMerchantRepo struct {
	lookupResult  map[int64]int64
	pricingByID   map[int64]*CachedMerchantPricing
	lookupCalls   int
	pricingCalls  int
}

func (f *fakeMerchantRepo) Create(context.Context, *Merchant) error           { return nil }
func (f *fakeMerchantRepo) GetByID(context.Context, int64) (*Merchant, error) { return nil, nil }
func (f *fakeMerchantRepo) GetByOwnerUserID(context.Context, int64) (*Merchant, error) {
	return nil, nil
}
func (f *fakeMerchantRepo) GetByDomain(context.Context, string) (*Merchant, error) { return nil, nil }
func (f *fakeMerchantRepo) List(context.Context, string, int, int) ([]*Merchant, int, error) {
	return nil, 0, nil
}
func (f *fakeMerchantRepo) Update(context.Context, *Merchant) error                { return nil }
func (f *fakeMerchantRepo) UpdateStatus(context.Context, int64, string) error      { return nil }
func (f *fakeMerchantRepo) UpdateDiscount(context.Context, int64, float64) error   { return nil }
func (f *fakeMerchantRepo) UpdateMarkupDefault(context.Context, int64, float64) error {
	return nil
}
func (f *fakeMerchantRepo) SoftDelete(context.Context, int64) error { return nil }

func (f *fakeMerchantRepo) LookupMerchantIDForUser(_ context.Context, userID int64) (int64, error) {
	f.lookupCalls++
	return f.lookupResult[userID], nil
}

func (f *fakeMerchantRepo) LoadPricing(_ context.Context, merchantID int64) (*CachedMerchantPricing, error) {
	f.pricingCalls++
	if p, ok := f.pricingByID[merchantID]; ok {
		return p, nil
	}
	return nil, nil
}

func newPricingTestService(enabled bool, lookup map[int64]int64, pricing map[int64]*CachedMerchantPricing) (*MerchantPricingService, *fakeMerchantRepo) {
	cfg := &config.Config{Merchant: config.MerchantConfig{Enabled: enabled}}
	repo := &fakeMerchantRepo{lookupResult: lookup, pricingByID: pricing}
	svc := NewMerchantPricingService(cfg, repo, nil)
	return svc, repo
}

func TestApplyUsageMarkup_FlagOff(t *testing.T) {
	svc, _ := newPricingTestService(false, nil, nil)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: 1, BaseCost: 1.0, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty result with flag off, got %+v", r)
	}
}

func TestApplyUsageMarkup_NonBalanceBilling(t *testing.T) {
	svc, _ := newPricingTestService(true,
		map[int64]int64{1: 100},
		map[int64]*CachedMerchantPricing{100: {MerchantID: 100, OwnerUserID: 999, Status: "active", UserMarkupDefault: 1.5}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: 1, BaseCost: 1.0, BillingType: BillingTypeSubscription,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty result for subscription billing, got %+v", r)
	}
}

func TestApplyUsageMarkup_FreeRequest(t *testing.T) {
	svc, _ := newPricingTestService(true,
		map[int64]int64{1: 100},
		map[int64]*CachedMerchantPricing{100: {MerchantID: 100, OwnerUserID: 999, Status: "active", UserMarkupDefault: 1.5}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: 1, BaseCost: 0, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty result for free request, got %+v", r)
	}
}

func TestApplyUsageMarkup_OwnerSelfUsage(t *testing.T) {
	const ownerID, merchantID = int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{ownerID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active", UserMarkupDefault: 1.5,
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: ownerID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r.MerchantLedger == nil {
		t.Fatalf("expected MerchantLedger draft for owner, got nil")
	}
	if r.MerchantLedger.Source != MerchantSourceOwnerUsageDebit {
		t.Errorf("expected source=owner_usage_debit, got %s", r.MerchantLedger.Source)
	}
	if r.MerchantLedger.Amount != 10 {
		t.Errorf("expected amount=10 (base), got %v", r.MerchantLedger.Amount)
	}
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil {
		t.Errorf("owner self usage should not produce override or outbox draft")
	}
}

func TestApplyUsageMarkup_OwnerSelfUsage_SuspendedStillWrites(t *testing.T) {
	// RFC §5.3.1：suspended owner 仍写 owner_usage_debit ledger（保对账等式）
	const ownerID, merchantID = int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{ownerID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "suspended", UserMarkupDefault: 1.5,
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: ownerID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r.MerchantLedger == nil {
		t.Fatalf("expected MerchantLedger even for suspended owner, got nil")
	}
}

func TestApplyUsageMarkup_SubUser_Suspended(t *testing.T) {
	const subID, ownerID, merchantID = int64(1), int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "suspended", UserMarkupDefault: 1.5,
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty for suspended sub_user, got %+v", r)
	}
}

func TestApplyUsageMarkup_SubUser_NoMarkup(t *testing.T) {
	const subID, ownerID, merchantID = int64(1), int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active", UserMarkupDefault: 1.0,
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty for markup=1.0, got %+v", r)
	}
}

func TestApplyUsageMarkup_SubUser_DefaultMarkup(t *testing.T) {
	const subID, ownerID, merchantID = int64(1), int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active", UserMarkupDefault: 1.5,
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride == nil || r.MerchantOutbox == nil {
		t.Fatalf("expected override+outbox for sub_user markup>1, got %+v", r)
	}
	if *r.BalanceCostOverride != 15 {
		t.Errorf("expected override=15 (10*1.5), got %v", *r.BalanceCostOverride)
	}
	if r.MerchantOutbox.Amount != 5 {
		t.Errorf("expected outbox amount=5 (10*0.5), got %v", r.MerchantOutbox.Amount)
	}
	if r.MerchantOutbox.Source != MerchantSourceUserMarkupShare {
		t.Errorf("expected source=user_markup_share, got %s", r.MerchantOutbox.Source)
	}
	if r.MerchantOutbox.CounterpartyUserID != subID {
		t.Errorf("expected counterparty=sub_user, got %d", r.MerchantOutbox.CounterpartyUserID)
	}
}

func TestApplyUsageMarkup_SubUser_GroupOverride(t *testing.T) {
	const subID, ownerID, merchantID, groupID = int64(1), int64(999), int64(100), int64(7)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
			UserMarkupDefault: 1.2,
			GroupMarkups:      map[int64]float64{groupID: 1.8},
		}},
	)
	// 命中 group
	r1 := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: groupID, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r1.BalanceCostOverride == nil || *r1.BalanceCostOverride != 18 {
		t.Errorf("expected override=18 (group markup 1.8), got %+v", r1.BalanceCostOverride)
	}
	// 不在 group 配置 → fallback default
	r2 := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: 99, BaseCost: 10, BillingType: BillingTypeBalance,
	})
	if r2.BalanceCostOverride == nil || *r2.BalanceCostOverride != 12 {
		t.Errorf("expected override=12 (default 1.2), got %+v", r2.BalanceCostOverride)
	}
}

func TestApplyUsageMarkup_CacheUsesPerMerchantInvalidation(t *testing.T) {
	const subID, ownerID, merchantID = int64(1), int64(999), int64(100)
	pricing := &CachedMerchantPricing{
		MerchantID: merchantID, OwnerUserID: ownerID, Status: "active", UserMarkupDefault: 1.5,
	}
	svc, repo := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: pricing},
	)
	in := MerchantUsagePricingInput{UserID: subID, BaseCost: 10, BillingType: BillingTypeBalance}

	// 第一次调用：触发 lookup + load pricing
	_ = svc.ApplyUsageMarkup(context.Background(), in)
	if repo.lookupCalls != 1 || repo.pricingCalls != 1 {
		t.Fatalf("expected (1,1) calls, got (%d,%d)", repo.lookupCalls, repo.pricingCalls)
	}
	// 第二次：完全命中缓存
	_ = svc.ApplyUsageMarkup(context.Background(), in)
	if repo.lookupCalls != 1 || repo.pricingCalls != 1 {
		t.Fatalf("expected cached (1,1), got (%d,%d)", repo.lookupCalls, repo.pricingCalls)
	}
	// admin 改价 → 只 invalidate merchant 维度
	svc.InvalidateMerchant(merchantID)
	pricing.UserMarkupDefault = 1.8 // mock fake repo data change
	_ = svc.ApplyUsageMarkup(context.Background(), in)
	if repo.lookupCalls != 1 || repo.pricingCalls != 2 {
		t.Fatalf("expected (1,2) after merchant invalidation, got (%d,%d)", repo.lookupCalls, repo.pricingCalls)
	}
}
