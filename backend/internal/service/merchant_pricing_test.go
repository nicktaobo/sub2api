// MERCHANT-SYSTEM v2.0
// MerchantPricingService 单元测试（不依赖 DB）：
//   - feature flag off → empty result
//   - billing type != balance → empty
//   - RawCost ≤ 0 → empty
//   - owner 自用 → 返回 MerchantLedger draft（amount = SiteActualCost）
//   - sub_user suspended → empty
//   - sub_user + 商户该 group 未配 sell_rate → empty（按主站价付）
//   - sub_user + 商户配了 sell_rate（cost 未配）→ override = RawCost × sell_rate，无 outbox（无利润）
//   - sub_user + 商户配了 sell_rate + cost_rate → override + outbox（利润 = RawCost × (sell-cost)）
//   - 缓存命中 + InvalidateMerchant 重新加载

package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const floatEps = 1e-9

// fakeMerchantRepo 仅实现 LookupMerchantIDForUser + LoadPricing 用于 pricing 测试。
type fakeMerchantRepo struct {
	lookupResult map[int64]int64
	pricingByID  map[int64]*CachedMerchantPricing
	lookupCalls  int
	pricingCalls int
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
func (f *fakeMerchantRepo) Update(context.Context, *Merchant) error              { return nil }
func (f *fakeMerchantRepo) UpdateStatus(context.Context, int64, string) error    { return nil }
func (f *fakeMerchantRepo) UpdateDiscount(context.Context, int64, float64) error { return nil }
func (f *fakeMerchantRepo) SoftDelete(context.Context, int64) error              { return nil }

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
		UserID: 1, RawCost: 1.0, SiteActualCost: 1.0, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty result with flag off, got %+v", r)
	}
}

func TestApplyUsageMarkup_NonBalanceBilling(t *testing.T) {
	svc, _ := newPricingTestService(true,
		map[int64]int64{1: 100},
		map[int64]*CachedMerchantPricing{100: {MerchantID: 100, OwnerUserID: 999, Status: "active"}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: 1, RawCost: 1.0, SiteActualCost: 1.0, BillingType: BillingTypeSubscription,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty result for subscription billing, got %+v", r)
	}
}

func TestApplyUsageMarkup_FreeRequest(t *testing.T) {
	svc, _ := newPricingTestService(true,
		map[int64]int64{1: 100},
		map[int64]*CachedMerchantPricing{100: {MerchantID: 100, OwnerUserID: 999, Status: "active"}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: 1, RawCost: 0, SiteActualCost: 0, BillingType: BillingTypeBalance,
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
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: ownerID, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	})
	if r.MerchantLedger == nil {
		t.Fatalf("expected MerchantLedger draft for owner, got nil")
	}
	if r.MerchantLedger.Source != MerchantSourceOwnerUsageDebit {
		t.Errorf("expected source=owner_usage_debit, got %s", r.MerchantLedger.Source)
	}
	if r.MerchantLedger.Amount != 10 {
		t.Errorf("expected amount=10 (site actual), got %v", r.MerchantLedger.Amount)
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
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "suspended",
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: ownerID, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
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
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "suspended",
			GroupSellRates: map[int64]float64{7: 1.5},
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: 7, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty for suspended sub_user, got %+v", r)
	}
}

func TestApplyUsageMarkup_SubUser_NoSellRateConfigured(t *testing.T) {
	// 商户没在该 group 配 sell_rate → 不参与分润，sub_user 按主站价付（pricing hook 返回空）
	const subID, ownerID, merchantID = int64(1), int64(999), int64(100)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: 7, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride != nil || r.MerchantOutbox != nil || r.MerchantLedger != nil {
		t.Fatalf("expected empty when no sell_rate configured, got %+v", r)
	}
}

func TestApplyUsageMarkup_SubUser_SellRateOnly_NoProfit(t *testing.T) {
	// sell_rate=1.0 = 主站价（rateMultiplier=1.0），cost 未配 → 回退到 site rate → 利润=0，
	// 仍按 sell_rate 扣 sub_user，但不写 outbox。
	const subID, ownerID, merchantID, groupID = int64(1), int64(999), int64(100), int64(7)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
			GroupSellRates: map[int64]float64{groupID: 1.0},
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: groupID, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride == nil || *r.BalanceCostOverride != 10 {
		t.Errorf("expected override=10 (10*1.0), got %+v", r.BalanceCostOverride)
	}
	if r.MerchantOutbox != nil {
		t.Errorf("expected no outbox when profit=0, got %+v", r.MerchantOutbox)
	}
}

func TestApplyUsageMarkup_SubUser_SellAndCost_Profit(t *testing.T) {
	// 商户配 sell_rate=1.8 cost_rate=1.2 → sub_user 扣 RawCost×1.8=18，
	// 商户利润 RawCost×(1.8-1.2)=6 进 outbox。
	const subID, ownerID, merchantID, groupID = int64(1), int64(999), int64(100), int64(7)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
			GroupSellRates: map[int64]float64{groupID: 1.8},
			GroupCosts:     map[int64]float64{groupID: 1.2},
		}},
	)
	r := svc.ApplyUsageMarkup(context.Background(), MerchantUsagePricingInput{
		UserID: subID, GroupID: groupID, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	})
	if r.BalanceCostOverride == nil || *r.BalanceCostOverride != 18 {
		t.Fatalf("expected override=18 (10*1.8), got %+v", r.BalanceCostOverride)
	}
	if r.MerchantOutbox == nil {
		t.Fatalf("expected outbox draft, got nil")
	}
	if math.Abs(r.MerchantOutbox.Amount-6) > floatEps {
		t.Errorf("expected outbox amount≈6 (10*0.6), got %v", r.MerchantOutbox.Amount)
	}
	if r.MerchantOutbox.Source != MerchantSourceUserMarkupShare {
		t.Errorf("expected source=user_markup_share, got %s", r.MerchantOutbox.Source)
	}
	if r.MerchantOutbox.CounterpartyUserID != subID {
		t.Errorf("expected counterparty=sub_user, got %d", r.MerchantOutbox.CounterpartyUserID)
	}
}

func TestApplyUsageMarkup_CacheUsesPerMerchantInvalidation(t *testing.T) {
	const subID, ownerID, merchantID, groupID = int64(1), int64(999), int64(100), int64(7)
	pricing := &CachedMerchantPricing{
		MerchantID: merchantID, OwnerUserID: ownerID, Status: "active",
		GroupSellRates: map[int64]float64{groupID: 1.5},
	}
	svc, repo := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: pricing},
	)
	in := MerchantUsagePricingInput{
		UserID: subID, GroupID: groupID, RawCost: 10, SiteActualCost: 10, BillingType: BillingTypeBalance,
	}

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
	pricing.GroupSellRates[groupID] = 1.8 // mock fake repo data change
	_ = svc.ApplyUsageMarkup(context.Background(), in)
	if repo.lookupCalls != 1 || repo.pricingCalls != 2 {
		t.Fatalf("expected (1,2) after merchant invalidation, got (%d,%d)", repo.lookupCalls, repo.pricingCalls)
	}
}
