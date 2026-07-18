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
func (f *fakeMerchantRepo) Update(context.Context, *Merchant) error           { return nil }
func (f *fakeMerchantRepo) UpdateStatus(context.Context, int64, string) error { return nil }
func (f *fakeMerchantRepo) SoftDelete(context.Context, int64) error           { return nil }

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

// ----------------------------------------------------------------------------
// 展示层查询：LookupSellRateForUser / LookupOwnerSellRate / LookupSellRateByMerchant
// 保证"模型列表"三条展示路径与计费口径一致：sub_user 看售价、owner 看主站价（但可预览）、
// 白标广场按域名商户看售价。
// ----------------------------------------------------------------------------

func TestLookupSellRateForUser_SubUserSeesSellRate(t *testing.T) {
	const merchantID, ownerID, subID, groupID = int64(100), int64(999), int64(7), int64(3)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	rate, ok := svc.LookupSellRateForUser(context.Background(), subID, groupID)
	if !ok || math.Abs(rate-1.5) > floatEps {
		t.Fatalf("sub_user should see sell_rate 1.5, got (%v,%v)", rate, ok)
	}
}

func TestLookupSellRateForUser_OwnerFallsBackToSiteRate(t *testing.T) {
	const merchantID, ownerID, groupID = int64(100), int64(999), int64(3)
	svc, _ := newPricingTestService(true,
		map[int64]int64{ownerID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	if _, ok := svc.LookupSellRateForUser(context.Background(), ownerID, groupID); ok {
		t.Fatal("owner must fall back to site rate (ok=false), got ok=true")
	}
}

func TestLookupSellRateForUser_FlagOff(t *testing.T) {
	const merchantID, subID, groupID = int64(100), int64(7), int64(3)
	svc, _ := newPricingTestService(false,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	if _, ok := svc.LookupSellRateForUser(context.Background(), subID, groupID); ok {
		t.Fatal("flag off must return ok=false")
	}
}

func TestLookupOwnerSellRate_OwnerGetsPreview(t *testing.T) {
	const merchantID, ownerID, groupID = int64(100), int64(999), int64(3)
	svc, _ := newPricingTestService(true,
		map[int64]int64{ownerID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	rate, ok := svc.LookupOwnerSellRate(context.Background(), ownerID, groupID)
	if !ok || math.Abs(rate-1.5) > floatEps {
		t.Fatalf("owner preview should return sell_rate 1.5, got (%v,%v)", rate, ok)
	}
}

func TestLookupOwnerSellRate_SubUserNoPreview(t *testing.T) {
	const merchantID, ownerID, subID, groupID = int64(100), int64(999), int64(7), int64(3)
	svc, _ := newPricingTestService(true,
		map[int64]int64{subID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	// sub_user 不走预览通道（他们走 LookupSellRateForUser 拿真实价）
	if _, ok := svc.LookupOwnerSellRate(context.Background(), subID, groupID); ok {
		t.Fatal("sub_user must not get owner preview (ok=false)")
	}
}

func TestLookupOwnerSellRate_SuspendedNoPreview(t *testing.T) {
	const merchantID, ownerID, groupID = int64(100), int64(999), int64(3)
	svc, _ := newPricingTestService(true,
		map[int64]int64{ownerID: merchantID},
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: ownerID, Status: MerchantStatusSuspended,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	if _, ok := svc.LookupOwnerSellRate(context.Background(), ownerID, groupID); ok {
		t.Fatal("suspended merchant owner preview must be ok=false")
	}
}

func TestLookupSellRateByMerchant_ActiveSeesSellRate(t *testing.T) {
	const merchantID, groupID = int64(100), int64(3)
	svc, _ := newPricingTestService(true,
		nil,
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, OwnerUserID: 999, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	rate, ok := svc.LookupSellRateByMerchant(context.Background(), merchantID, groupID)
	if !ok || math.Abs(rate-1.5) > floatEps {
		t.Fatalf("public plaza should show merchant sell_rate 1.5, got (%v,%v)", rate, ok)
	}
}

func TestLookupSellRateByMerchant_SuspendedFallsBack(t *testing.T) {
	const merchantID, groupID = int64(100), int64(3)
	svc, _ := newPricingTestService(true,
		nil,
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, Status: MerchantStatusSuspended,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	if _, ok := svc.LookupSellRateByMerchant(context.Background(), merchantID, groupID); ok {
		t.Fatal("suspended merchant must fall back to site rate (ok=false)")
	}
}

func TestLookupSellRateByMerchant_UnconfiguredGroupFallsBack(t *testing.T) {
	const merchantID, groupID, otherGroup = int64(100), int64(3), int64(9)
	svc, _ := newPricingTestService(true,
		nil,
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{otherGroup: 1.5},
		}},
	)
	if _, ok := svc.LookupSellRateByMerchant(context.Background(), merchantID, groupID); ok {
		t.Fatal("group without sell_rate must fall back to site rate (ok=false)")
	}
}

func TestLookupSellRateByMerchant_FlagOff(t *testing.T) {
	const merchantID, groupID = int64(100), int64(3)
	svc, _ := newPricingTestService(false,
		nil,
		map[int64]*CachedMerchantPricing{merchantID: {
			MerchantID: merchantID, Status: MerchantStatusActive,
			GroupSellRates: map[int64]float64{groupID: 1.5},
		}},
	)
	if _, ok := svc.LookupSellRateByMerchant(context.Background(), merchantID, groupID); ok {
		t.Fatal("flag off must return ok=false")
	}
}
