// MERCHANT-AFFILIATE v1.0
// MerchantAffiliateRebateService.ApplyRebate 单元测试（不依赖 DB）。

package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type fakeMerchantAffiliateRepo struct {
	binding *MerchantAffiliateBinding
	cfg     *MerchantAffiliateRebateConfig
}

func (f *fakeMerchantAffiliateRepo) LookupBindingForInvitee(_ context.Context, _ int64) (*MerchantAffiliateBinding, error) {
	return f.binding, nil
}
func (f *fakeMerchantAffiliateRepo) LoadRebateConfig(_ context.Context, _ int64) (*MerchantAffiliateRebateConfig, error) {
	return f.cfg, nil
}
func (f *fakeMerchantAffiliateRepo) UpdateRebateConfig(context.Context, int64, bool, float64) error {
	return nil
}
func (f *fakeMerchantAffiliateRepo) ResolveSubUserByAffCode(context.Context, string, int64) (int64, bool, error) {
	return 0, false, nil
}
func (f *fakeMerchantAffiliateRepo) CreateBinding(context.Context, int64, int64, int64) (bool, error) {
	return true, nil
}
func (f *fakeMerchantAffiliateRepo) DownlineStats(context.Context, int64) (*MerchantAffiliateDownlineStats, error) {
	return &MerchantAffiliateDownlineStats{}, nil
}
func (f *fakeMerchantAffiliateRepo) GetAffCode(context.Context, int64) (string, error) {
	return "TESTCODE", nil
}

func newMerchantAffiliateTestService(enabled bool, binding *MerchantAffiliateBinding, cfg *MerchantAffiliateRebateConfig) *MerchantAffiliateRebateService {
	c := &config.Config{Merchant: config.MerchantConfig{Enabled: enabled, AffiliateRebateEnabled: enabled}}
	repo := &fakeMerchantAffiliateRepo{binding: binding, cfg: cfg}
	return NewMerchantAffiliateRebateService(c, repo)
}

const (
	maMerchantID = int64(100)
	maInviter    = int64(7)
	maInvitee    = int64(9)
)

func activeBinding() *MerchantAffiliateBinding {
	return &MerchantAffiliateBinding{InviterUserID: maInviter, MerchantID: maMerchantID}
}
func enabledCfg(rate float64) *MerchantAffiliateRebateConfig {
	return &MerchantAffiliateRebateConfig{Enabled: true, RatePercent: rate}
}

func TestMerchantAffiliate_HappyPath(t *testing.T) {
	svc := newMerchantAffiliateTestService(true, activeBinding(), enabledCfg(20))
	res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	})
	if res.Outbox == nil {
		t.Fatal("expected rebate outbox")
	}
	if math.Abs(res.Outbox.Amount-2.0) > 1e-9 {
		t.Fatalf("expected rebate 2.0 (20%% of 10), got %v", res.Outbox.Amount)
	}
	if res.Outbox.InviterUserID != maInviter || res.Outbox.InviteeUserID != maInvitee || res.Outbox.MerchantID != maMerchantID {
		t.Fatalf("unexpected outbox identity: %+v", res.Outbox)
	}
}

func TestMerchantAffiliate_FlagOff(t *testing.T) {
	svc := newMerchantAffiliateTestService(false, activeBinding(), enabledCfg(20))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("flag off must not produce rebate")
	}
}

func TestMerchantAffiliate_NoProfit(t *testing.T) {
	svc := newMerchantAffiliateTestService(true, activeBinding(), enabledCfg(20))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 0,
	}); res.Outbox != nil {
		t.Fatal("no merchant profit must not produce rebate")
	}
}

func TestMerchantAffiliate_MerchantDisabled(t *testing.T) {
	svc := newMerchantAffiliateTestService(true, activeBinding(), &MerchantAffiliateRebateConfig{Enabled: false, RatePercent: 20})
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("merchant with rebate disabled must not produce rebate")
	}
}

func TestMerchantAffiliate_ZeroRate(t *testing.T) {
	svc := newMerchantAffiliateTestService(true, activeBinding(), enabledCfg(0))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("zero rate must not produce rebate")
	}
}

func TestMerchantAffiliate_NoBinding(t *testing.T) {
	svc := newMerchantAffiliateTestService(true, nil, enabledCfg(20))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("no inviter binding must not produce rebate")
	}
}

func TestMerchantAffiliate_BindingWrongMerchant(t *testing.T) {
	// 绑定属于另一个商户：防御性拒绝（子用户换商户后绑定应已清理）。
	binding := &MerchantAffiliateBinding{InviterUserID: maInviter, MerchantID: 999}
	svc := newMerchantAffiliateTestService(true, binding, enabledCfg(20))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("binding from a different merchant must not produce rebate")
	}
}

func TestMerchantAffiliate_SelfInviteGuard(t *testing.T) {
	binding := &MerchantAffiliateBinding{InviterUserID: maInvitee, MerchantID: maMerchantID} // inviter == invitee
	svc := newMerchantAffiliateTestService(true, binding, enabledCfg(20))
	if res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	}); res.Outbox != nil {
		t.Fatal("self-invite must not produce rebate")
	}
}

func TestMerchantAffiliate_RateClampedToProfit(t *testing.T) {
	// 比例 100% → 返利 = 全部利润（billing 路径据此把 merchant outbox 置 nil）。
	svc := newMerchantAffiliateTestService(true, activeBinding(), enabledCfg(100))
	res := svc.ApplyRebate(context.Background(), MerchantAffiliateRebateInput{
		MerchantID: maMerchantID, InviteeUserID: maInvitee, MerchantProfit: 10,
	})
	if res.Outbox == nil || math.Abs(res.Outbox.Amount-10.0) > 1e-9 {
		t.Fatalf("100%% rate should rebate full profit 10, got %+v", res.Outbox)
	}
}
