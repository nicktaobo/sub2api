// AffiliateRebatePricingService 邀请返利消费侧 pricing hook（与 merchant_pricing 对称）。
//
// 在 gateway 的 recordUsageCore / OpenAIGatewayService.RecordUsage 中调用，根据被邀请人
// 是否绑定了 inviter、所在分组是否被排除、消费金额是否达到阈值等条件，决定是否要写一条
// outbox 草稿到 user_affiliate_consumption_outbox。
//
// 与 merchant 的差异：
//   - merchant 解决"商户从 sub_user 消费里抽成"；affiliate 解决"邀请人从被邀请人消费里拿返利"
//   - 两者完全独立，hook 顺序无所谓，互不影响
//   - 本 hook 仅按全局消费返利比例计算 amount，inviter 的专属比例 (aff_rebate_rate_percent)
//     在 v1 不影响消费侧（仅作用于充值返利），简化热路径

package service

import (
	"context"
	"log/slog"
	"time"
)

// AffiliateRebateInput pricing hook 入参。
type AffiliateRebateInput struct {
	UserID         int64
	GroupID        int64
	GroupExcluded  bool // groups.affiliate_rebate_excluded
	BillingType    int8
	SiteActualCost float64 // 平台实收，作为返利基数
}

// AffiliateRebateOutboxDraft 写入 user_affiliate_consumption_outbox 的草稿。
type AffiliateRebateOutboxDraft struct {
	InviterID     int64
	InviteeUserID int64
	Amount        float64 // 已经乘过比例的待入账金额
}

// AffiliateRebateResult hook 出参。Outbox==nil 表示这次消费不产生返利。
type AffiliateRebateResult struct {
	Outbox *AffiliateRebateOutboxDraft
}

// AffiliateRebatePricingService 消费返利 pricing hook 服务。
// 内部缓存 invitee userID → inviterID 映射（0 = 无邀请人），避免每次请求查 DB。
type AffiliateRebatePricingService struct {
	settingService *SettingService
	repo           AffiliateRepository
	inviterCache   *ttlCache[int64, int64]
}

// NewAffiliateRebatePricingService DI 构造函数。
func NewAffiliateRebatePricingService(settingService *SettingService, repo AffiliateRepository) *AffiliateRebatePricingService {
	const (
		cacheCap = 10000
		ttl      = 5 * time.Minute
	)
	return &AffiliateRebatePricingService{
		settingService: settingService,
		repo:           repo,
		inviterCache:   newTTLCache[int64, int64](cacheCap, ttl),
	}
}

// InvalidateUser 绑定/解绑邀请人时主动失效。AffiliateService.BindInviterByCode 成功后应当调用。
func (s *AffiliateRebatePricingService) InvalidateUser(userID int64) {
	if s == nil || s.inviterCache == nil {
		return
	}
	s.inviterCache.Remove(userID)
}

// ApplyConsumptionRebate 主入口，返回是否要写 outbox。
// 早返回顺序（成本递增）：
//  1. service 未初始化
//  2. 总开关 / 消费子开关 / 计费类型 / 分组排除 / 最小金额
//  3. inviter 查找（带缓存）
//  4. 比例计算
func (s *AffiliateRebatePricingService) ApplyConsumptionRebate(ctx context.Context, in AffiliateRebateInput) AffiliateRebateResult {
	if s == nil || s.settingService == nil || s.repo == nil {
		return AffiliateRebateResult{}
	}
	// 总开关 + 子开关都开才走（任一关都早返）
	if !s.settingService.IsAffiliateEnabled(ctx) {
		return AffiliateRebateResult{}
	}
	if !s.settingService.IsAffiliateConsumeRebateEnabled(ctx) {
		return AffiliateRebateResult{}
	}
	// 仅余额计费参与，与 merchant 一致：订阅扣的是 daily/weekly/monthly_usage_usd，
	// 不实际从 users.balance 走钱，不应该再分润给 inviter。
	if in.BillingType != BillingTypeBalance {
		return AffiliateRebateResult{}
	}
	if in.GroupExcluded {
		return AffiliateRebateResult{}
	}
	if in.SiteActualCost <= 0 {
		return AffiliateRebateResult{}
	}
	minAmount := s.settingService.GetAffiliateConsumeRebateMinAmount(ctx)
	if in.SiteActualCost < minAmount {
		return AffiliateRebateResult{}
	}

	// 查 inviter（cache 0 表示"无邀请人"，避免重复查 DB）
	inviterID, ok := s.inviterCache.Get(in.UserID)
	if !ok {
		summary, err := s.repo.EnsureUserAffiliate(ctx, in.UserID)
		if err != nil {
			slog.Warn("affiliate consume rebate: lookup inviter failed",
				"user_id", in.UserID, "error", err)
			return AffiliateRebateResult{}
		}
		if summary != nil && summary.InviterID != nil && *summary.InviterID > 0 {
			inviterID = *summary.InviterID
		}
		s.inviterCache.Set(in.UserID, inviterID)
	}
	if inviterID == 0 {
		return AffiliateRebateResult{}
	}

	ratePercent := s.settingService.GetAffiliateConsumeRebateRatePercent(ctx)
	amount := roundTo(in.SiteActualCost*(ratePercent/100), 8)
	if amount <= 0 {
		return AffiliateRebateResult{}
	}

	return AffiliateRebateResult{
		Outbox: &AffiliateRebateOutboxDraft{
			InviterID:     inviterID,
			InviteeUserID: in.UserID,
			Amount:        amount,
		},
	}
}
