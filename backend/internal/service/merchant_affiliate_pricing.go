// MERCHANT-AFFILIATE v1.0
// MerchantAffiliateRebateService：代理下级邀请返利消费侧 pricing hook。
//
// 与平台级 AffiliateRebatePricingService 对称，但资金来源不同：
//   - 平台级：返利来自 platform 实收（SiteActualCost），进邀请人平台余额；
//   - 本 service：返利来自**商户利润(markup)**——调用方把 merchant markup outbox 的
//     利润额传进来，本 hook 切出一部分给邀请人，调用方据此把商户利润减掉同额并写入
//     merchant_affiliate_consumption_outbox。守恒：商户净利 + 邀请人返利 = 原商户利润。
//
// 单层：一个被邀请人在其商户下只绑定一个邀请人（merchant_affiliate_bindings 唯一约束）。

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// MerchantAffiliateRebateInput hook 入参。
type MerchantAffiliateRebateInput struct {
	MerchantID     int64   // 来自 merchant markup outbox（这次消费归属的商户）
	InviteeUserID  int64   // 产生消费的子用户
	MerchantProfit float64 // = MerchantOutboxDraft.Amount，本次消费的商户利润（返利基数）
}

// MerchantAffiliateRebateOutboxDraft 写入 merchant_affiliate_consumption_outbox 的草稿。
type MerchantAffiliateRebateOutboxDraft struct {
	MerchantID    int64
	InviterUserID int64
	InviteeUserID int64
	Amount        float64 // 已乘过比例、并 clamp 到不超过商户利润
}

// MerchantAffiliateRebateResult hook 出参。Outbox==nil 表示这次消费不产生返利。
type MerchantAffiliateRebateResult struct {
	Outbox *MerchantAffiliateRebateOutboxDraft
}

// MerchantAffiliateRebateConfig 某商户的下级邀请返利配置（缓存对象）。
type MerchantAffiliateRebateConfig struct {
	Enabled     bool
	RatePercent float64 // 0-100
}

// MerchantAffiliateBinding 某被邀请人的绑定（缓存对象）。
type MerchantAffiliateBinding struct {
	InviterUserID int64
	MerchantID    int64
}

// MerchantAffiliateDownlineStats 邀请人在其商户下的下线概况（子用户视图用）。
type MerchantAffiliateDownlineStats struct {
	InviteeCount int64
	TotalRebate  float64 // 累计已入账返利（processed=TRUE 的 outbox 求和）
}

// MerchantAffiliateRepository 下级邀请返利所需的持久化查询。
type MerchantAffiliateRepository interface {
	// LookupBindingForInvitee 反查某被邀请人的邀请绑定；无绑定返回 (nil, nil)。
	LookupBindingForInvitee(ctx context.Context, inviteeUserID int64) (*MerchantAffiliateBinding, error)
	// LoadRebateConfig 读某商户的返利开关+比例。
	LoadRebateConfig(ctx context.Context, merchantID int64) (*MerchantAffiliateRebateConfig, error)
	// UpdateRebateConfig 写某商户的返利开关+比例（商户后台配置）。
	UpdateRebateConfig(ctx context.Context, merchantID int64, enabled bool, ratePercent float64) error
	// ResolveSubUserByAffCode 把邀请码解析成"该商户下的一个子用户"；跨商户/无效码返回 (0,false,nil)。
	ResolveSubUserByAffCode(ctx context.Context, affCode string, merchantID int64) (int64, bool, error)
	// CreateBinding 建立单层绑定；invitee 已绑定时返回 (false, nil)（幂等，不改绑）。
	CreateBinding(ctx context.Context, merchantID, inviterUserID, inviteeUserID int64) (bool, error)
	// DownlineStats 邀请人在其商户下的下线数与累计返利。
	DownlineStats(ctx context.Context, inviterUserID int64) (*MerchantAffiliateDownlineStats, error)
	// GetAffCode 取某用户的邀请码（user_affiliates.aff_code，注册时已生成）。无则返回 ("", nil)。
	GetAffCode(ctx context.Context, userID int64) (string, error)
}

// MerchantAffiliateRebateService 下级邀请返利 pricing hook 服务。
// 双层缓存：invitee→binding（0/nil 表示无邀请人）、merchant→config。
type MerchantAffiliateRebateService struct {
	cfg  *config.Config
	repo MerchantAffiliateRepository

	bindingCache *ttlCache[int64, *MerchantAffiliateBinding]
	configCache  *ttlCache[int64, *MerchantAffiliateRebateConfig]
}

// NewMerchantAffiliateRebateService DI 构造函数。
func NewMerchantAffiliateRebateService(cfg *config.Config, repo MerchantAffiliateRepository) *MerchantAffiliateRebateService {
	const (
		bindingCacheCap = 10000
		configCacheCap  = 1024
		ttl             = 5 * time.Minute
	)
	return &MerchantAffiliateRebateService{
		cfg:          cfg,
		repo:         repo,
		bindingCache: newTTLCache[int64, *MerchantAffiliateBinding](bindingCacheCap, ttl),
		configCache:  newTTLCache[int64, *MerchantAffiliateRebateConfig](configCacheCap, ttl),
	}
}

// InvalidateInvitee 绑定/解绑邀请人时主动失效。
func (s *MerchantAffiliateRebateService) InvalidateInvitee(inviteeUserID int64) {
	if s == nil || s.bindingCache == nil {
		return
	}
	s.bindingCache.Remove(inviteeUserID)
}

// InvalidateMerchant 商户改返利开关/比例时主动失效。
func (s *MerchantAffiliateRebateService) InvalidateMerchant(merchantID int64) {
	if s == nil || s.configCache == nil {
		return
	}
	s.configCache.Remove(merchantID)
}

// enabled 总开关：整个商户系统开 + 下级邀请返利子开关开。
func (s *MerchantAffiliateRebateService) enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Merchant.Enabled && s.cfg.Merchant.AffiliateRebateEnabled
}

// ApplyRebate 主入口：仅在被邀请人产生商户利润（MerchantProfit>0）且其商户开了返利、
// 被邀请人绑定了邀请人时，返回一条返利 outbox 草稿。
//
// 早返回顺序（成本递增）：
//  1. service/flag 未启用
//  2. 入参非法 / 无商户利润
//  3. 商户返利配置（带缓存）
//  4. 邀请绑定查找（带缓存）
//  5. 比例计算 + clamp
func (s *MerchantAffiliateRebateService) ApplyRebate(ctx context.Context, in MerchantAffiliateRebateInput) MerchantAffiliateRebateResult {
	if !s.enabled() || s.repo == nil {
		return MerchantAffiliateRebateResult{}
	}
	if in.MerchantID <= 0 || in.InviteeUserID <= 0 || in.MerchantProfit <= 0 {
		return MerchantAffiliateRebateResult{}
	}

	// 商户返利配置
	cfg, ok := s.configCache.Get(in.MerchantID)
	if !ok {
		loaded, err := s.repo.LoadRebateConfig(ctx, in.MerchantID)
		if err != nil {
			slog.Warn("merchant affiliate: load rebate config failed",
				"merchant_id", in.MerchantID, "error", err)
			return MerchantAffiliateRebateResult{}
		}
		cfg = loaded
		if cfg != nil {
			s.configCache.Set(in.MerchantID, cfg)
		}
	}
	if cfg == nil || !cfg.Enabled || cfg.RatePercent <= 0 {
		return MerchantAffiliateRebateResult{}
	}

	// 邀请绑定
	binding, ok := s.bindingCache.Get(in.InviteeUserID)
	if !ok {
		loaded, err := s.repo.LookupBindingForInvitee(ctx, in.InviteeUserID)
		if err != nil {
			slog.Warn("merchant affiliate: lookup binding failed",
				"invitee_user_id", in.InviteeUserID, "error", err)
			return MerchantAffiliateRebateResult{}
		}
		binding = loaded // nil 表示无邀请人，同样缓存避免重复查
		s.bindingCache.Set(in.InviteeUserID, binding)
	}
	if binding == nil || binding.InviterUserID <= 0 {
		return MerchantAffiliateRebateResult{}
	}
	// 防御：绑定的商户必须与本次消费归属的商户一致（子用户换商户后绑定应已失效/清理）。
	if binding.MerchantID != in.MerchantID {
		return MerchantAffiliateRebateResult{}
	}
	// 防御：不给自己返利（DB 已有 CHECK，双保险）。
	if binding.InviterUserID == in.InviteeUserID {
		return MerchantAffiliateRebateResult{}
	}

	rebate := roundTo(in.MerchantProfit*(cfg.RatePercent/100), 8)
	if rebate <= 0 {
		return MerchantAffiliateRebateResult{}
	}
	// 返利不能超过商户利润（比例理论上 ≤100，clamp 防御浮点/配置异常）。
	if rebate > in.MerchantProfit {
		rebate = in.MerchantProfit
	}

	return MerchantAffiliateRebateResult{
		Outbox: &MerchantAffiliateRebateOutboxDraft{
			MerchantID:    in.MerchantID,
			InviterUserID: binding.InviterUserID,
			InviteeUserID: in.InviteeUserID,
			Amount:        rebate,
		},
	}
}
