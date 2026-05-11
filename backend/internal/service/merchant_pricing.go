// MERCHANT-SYSTEM v1.0
// MerchantPricingService 共享 hook：
//   - sub_user 消费 markup（异步 outbox）
//   - owner 自用 API（同步 ledger）
//
// RFC §5.2.1 / §5.3.1。
// 双层缓存：userMerchantCache + merchantPricingCache。

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// MerchantOutboxDraft pricing hook 返回给 buildUsageBillingCommand 的 sub_user markup 草稿。
type MerchantOutboxDraft struct {
	MerchantID         int64
	CounterpartyUserID int64   // sub_user.id
	Amount             float64 // base × (markup-1)
	Source             string  // "user_markup_share"
	BaseCost           float64
	Markup             float64 // 实际生效值（group 覆盖或 default）
}

// MerchantLedgerDraft pricing hook 返回给 buildUsageBillingCommand 的 owner 自用同步 ledger 草稿。
// 与 MerchantOutboxDraft 互斥（owner vs sub_user 显式分支）。
type MerchantLedgerDraft struct {
	MerchantID  int64
	OwnerUserID int64
	Source      string  // "owner_usage_debit"
	Direction   string  // "debit"
	Amount      float64 // base cost
	RefType     string  // "usage_billing_dedup"
	// RefID 在 applyUsageBillingEffects 里用 dedupID 填充
}

// MerchantUsagePricingInput pricing hook 入参。
//
// v2.0 改 cost/sell 绝对倍率模型：调用方同时传 RawCost（= bd.TotalCost，base
// 价 × token，不含任何倍率）和 SiteActualCost（= bd.ActualCost，主站普通用户
// 实际付的价，已乘 rateMultiplier）。商户在该 group 有 sell_rate 配置时按
// 绝对倍率算账；未配置则不参与商户分润，sub_user 按主站价付。
type MerchantUsagePricingInput struct {
	UserID         int64
	GroupID        int64 // 来自 apiKey.GroupID（v1 不引入 ResolvedGroupID，详见 RFC §10.2）
	RawCost        float64
	SiteActualCost float64
	BillingType    int8
	APIKeyID       int64
}

// MerchantUsagePricingResult pricing hook 出参。
//   - BalanceCostOverride 仅 sub_user markup>1 时非 nil；钱包扣款用此值，quota/rate_limit 仍用 base
//   - MerchantOutbox 与 MerchantLedger 互斥（owner vs sub_user）
type MerchantUsagePricingResult struct {
	BalanceCostOverride *float64
	MerchantOutbox      *MerchantOutboxDraft
	MerchantLedger      *MerchantLedgerDraft
}

// MerchantPricingService 商户消费 markup / owner 自用 ledger pricing hook 共享 service。
// 同时被 GatewayService.recordUsageCore 和 OpenAIGatewayService.RecordUsage 调用。
type MerchantPricingService struct {
	cfg      *config.Config
	repo     MerchantRepository
	userRepo UserRepository

	userMerchantCache    *ttlCache[int64, int64]                  // user_id -> merchant_id (0=不属于商户)
	merchantPricingCache *ttlCache[int64, *CachedMerchantPricing] // merchant_id -> pricing
}

// NewMerchantPricingService DI 构造函数。
func NewMerchantPricingService(cfg *config.Config, repo MerchantRepository, userRepo UserRepository) *MerchantPricingService {
	const (
		userCacheCap     = 10000
		merchantCacheCap = 1024
		ttl              = 5 * time.Minute
	)
	return &MerchantPricingService{
		cfg:                  cfg,
		repo:                 repo,
		userRepo:             userRepo,
		userMerchantCache:    newTTLCache[int64, int64](userCacheCap, ttl),
		merchantPricingCache: newTTLCache[int64, *CachedMerchantPricing](merchantCacheCap, ttl),
	}
}

// InvalidateUser 解除/绑定 merchant 时主动失效（RFC §5.2.1 Step 2.1）。
// MerchantService 改 user.parent_merchant_id 时必须调一次。
func (s *MerchantPricingService) InvalidateUser(userID int64) {
	if s == nil || s.userMerchantCache == nil {
		return
	}
	s.userMerchantCache.Remove(userID)
}

// InvalidateMerchant admin 改 discount/markup_default/group_markup/status 时主动失效。
// 只删 merchant 维度即可让所有 sub_user 立即生效（无需逐个清 user 维度缓存）。
func (s *MerchantPricingService) InvalidateMerchant(merchantID int64) {
	if s == nil || s.merchantPricingCache == nil {
		return
	}
	s.merchantPricingCache.Remove(merchantID)
}

// ApplyUsageMarkup 主入口（v2.0 cost/sell 绝对值模型）。
//
// 路径：
//   - 早返回：flag 关闭 / 非 balance 计费 / RawCost ≤ 0 / user 不属于商户 / merchant 不存在
//   - owner 自用：返回 MerchantLedgerDraft（同步 ledger 入账主站价，与 v1 行为一致）
//   - sub_user suspended：返回 empty（防御性，正常已被 API key auth 拦截）
//   - sub_user active：
//   - 商户该 group 没配 sell_rate → 返回 empty，sub_user 按主站价付（不分润）
//   - 商户该 group 配了 sell_rate：
//   - BalanceCostOverride = RawCost × sell_rate（sub_user 实际扣这么多）
//   - 商户赚 = RawCost × (sell_rate − cost_rate)，写 outbox
//   - 平台收 = RawCost × cost_rate（隐式：扣的钱里减去商户的部分）
//   - cost_rate 未配置时回退 SiteActualCost / RawCost（即跟主站 rateMultiplier 等价，商户不享折扣）
func (s *MerchantPricingService) ApplyUsageMarkup(ctx context.Context, in MerchantUsagePricingInput) MerchantUsagePricingResult {
	if s == nil || s.cfg == nil {
		return MerchantUsagePricingResult{}
	}
	if !s.cfg.Merchant.Enabled {
		return MerchantUsagePricingResult{}
	}
	// v1 仅余额计费参与 merchant（订阅计费 RFC §3.3.0 跳过）
	if in.BillingType != BillingTypeBalance {
		return MerchantUsagePricingResult{}
	}
	// 免费请求短路（RFC v1.7 P2-4）：避免 amount > 0 CHECK 让计费事务失败
	if in.RawCost <= 0 {
		return MerchantUsagePricingResult{}
	}

	// 两层缓存
	merchantID, ok := s.userMerchantCache.Get(in.UserID)
	if !ok {
		mid, err := s.repo.LookupMerchantIDForUser(ctx, in.UserID)
		if err != nil {
			slog.Warn("merchant pricing: lookup merchant id failed",
				"user_id", in.UserID, "error", err)
			return MerchantUsagePricingResult{}
		}
		merchantID = mid
		s.userMerchantCache.Set(in.UserID, merchantID)
	}
	if merchantID == 0 {
		return MerchantUsagePricingResult{} // 普通主站用户
	}

	pricing, ok := s.merchantPricingCache.Get(merchantID)
	if !ok {
		p, err := s.repo.LoadPricing(ctx, merchantID)
		if err != nil {
			slog.Warn("merchant pricing: load pricing failed",
				"merchant_id", merchantID, "error", err)
			return MerchantUsagePricingResult{}
		}
		pricing = p
		if pricing != nil {
			s.merchantPricingCache.Set(merchantID, pricing)
		}
	}
	if pricing == nil {
		return MerchantUsagePricingResult{} // merchant 不存在或已 soft-deleted
	}

	// owner 自用（不论 status，保对账等式 §4.2.2.4）。
	// 入账金额沿用主站价（SiteActualCost），与 v1 行为一致：owner 自用不享 cost_rate 折扣。
	if pricing.OwnerUserID == in.UserID {
		return MerchantUsagePricingResult{
			MerchantLedger: &MerchantLedgerDraft{
				MerchantID:  pricing.MerchantID,
				OwnerUserID: pricing.OwnerUserID,
				Source:      MerchantSourceOwnerUsageDebit,
				Direction:   MerchantLedgerDirectionDebit,
				Amount:      in.SiteActualCost,
				RefType:     MerchantRefTypeUsageBillingDedup,
			},
		}
	}

	// sub_user：suspended 短路（API key auth 应该已经拦截，此处是防御性兜底）
	if pricing.Status != MerchantStatusActive {
		return MerchantUsagePricingResult{}
	}

	// 解析 cost/sell rate
	sellRate, hasSell := s.lookupRate(pricing.GroupSellRates, in.GroupID)
	if !hasSell {
		// 商户没在该 group 配 sell_rate → 不参与商户分润，sub_user 按主站价
		return MerchantUsagePricingResult{}
	}
	// cost_rate 未配 → 回退到 site rate（即 SiteActualCost / RawCost，确保 cost == 主站价）
	costRate, hasCost := s.lookupRate(pricing.GroupCosts, in.GroupID)
	if !hasCost {
		if in.RawCost > 0 {
			costRate = in.SiteActualCost / in.RawCost
		} else {
			costRate = sellRate // RawCost=0 已经在前面 early-return，这里只为防御性
		}
	}
	// 防御性：service 层会校验 sell ≥ cost，但缓存可能滞后；运行时取较大者避免负利润
	if sellRate < costRate {
		sellRate = costRate
	}

	subUserCharge := in.RawCost * sellRate
	merchantProfit := in.RawCost * (sellRate - costRate)
	if merchantProfit <= 0 {
		// sub_user 按主站价或更低，商户不赚 → 仍然按 sell_rate 扣 sub_user，但不写 outbox
		return MerchantUsagePricingResult{
			BalanceCostOverride: &subUserCharge,
		}
	}
	return MerchantUsagePricingResult{
		BalanceCostOverride: &subUserCharge,
		MerchantOutbox: &MerchantOutboxDraft{
			MerchantID:         pricing.MerchantID,
			CounterpartyUserID: in.UserID,
			Amount:             merchantProfit,
			Source:             MerchantSourceUserMarkupShare,
			BaseCost:           in.RawCost,
			Markup:             sellRate, // 字段语义保留，记录的是实际生效的 sell_rate（用于审计）
		},
	}
}

// lookupRate 从缓存里取某 group 的倍率配置；未命中返回 (0, false)。
func (s *MerchantPricingService) lookupRate(m map[int64]float64, groupID int64) (float64, bool) {
	if groupID <= 0 || m == nil {
		return 0, false
	}
	v, ok := m[groupID]
	return v, ok
}
