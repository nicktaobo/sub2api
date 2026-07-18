// MERCHANT-AFFILIATE v1.0
// MerchantAffiliateRebateService 的非热路径方法：注册绑定、商户后台配置、子用户视图。
// 与 pricing hook（merchant_affiliate_pricing.go）同一个 struct，共享 repo 与缓存失效。

package service

import (
	"context"
	"log/slog"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// BindInviterByCode 注册时给商户子用户建立单层邀请绑定。
//
// 静默语义：注册流程不应因邀请绑定失败而失败，任何"不该绑"的情况都返回 nil 跳过：
//   - 功能总开关关
//   - 无邀请码
//   - 码解析不到"同商户的另一个子用户"（跨商户 / 平台码 / 无效码）
//   - 自己邀请自己
//   - invitee 已绑定（单层不可改绑）
//
// 仅在确实新建了一条绑定时失效缓存。
func (s *MerchantAffiliateRebateService) BindInviterByCode(ctx context.Context, inviteeUserID, merchantID int64, affCode string) error {
	if !s.enabled() || s.repo == nil {
		return nil
	}
	code := strings.TrimSpace(affCode)
	if code == "" || inviteeUserID <= 0 || merchantID <= 0 {
		return nil
	}
	inviterUserID, ok, err := s.repo.ResolveSubUserByAffCode(ctx, code, merchantID)
	if err != nil {
		slog.Warn("merchant affiliate: resolve inviter by code failed",
			"merchant_id", merchantID, "invitee_user_id", inviteeUserID, "error", err)
		return nil // 静默：不阻断注册
	}
	if !ok || inviterUserID <= 0 || inviterUserID == inviteeUserID {
		return nil
	}
	created, err := s.repo.CreateBinding(ctx, merchantID, inviterUserID, inviteeUserID)
	if err != nil {
		slog.Warn("merchant affiliate: create binding failed",
			"merchant_id", merchantID, "inviter_user_id", inviterUserID,
			"invitee_user_id", inviteeUserID, "error", err)
		return nil
	}
	if created {
		s.InvalidateInvitee(inviteeUserID)
	}
	return nil
}

// GetRebateConfig 商户后台读本商户的下级邀请返利配置。功能总开关关时返回禁用态。
func (s *MerchantAffiliateRebateService) GetRebateConfig(ctx context.Context, merchantID int64) (*MerchantAffiliateRebateConfig, error) {
	if s == nil || s.repo == nil {
		return &MerchantAffiliateRebateConfig{}, nil
	}
	cfg, err := s.repo.LoadRebateConfig(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return &MerchantAffiliateRebateConfig{}, nil
	}
	return cfg, nil
}

// SetRebateConfig 商户后台写本商户的返利开关+比例。校验比例 [0,100] 并失效缓存。
func (s *MerchantAffiliateRebateService) SetRebateConfig(ctx context.Context, merchantID int64, enabled bool, ratePercent float64) error {
	if s == nil || s.repo == nil {
		return ErrMerchantInvalidParam
	}
	if !s.enabled() {
		return infraerrors.Forbidden("MERCHANT_AFFILIATE_DISABLED", "merchant affiliate rebate is disabled on this platform")
	}
	if ratePercent < 0 || ratePercent > 100 {
		return infraerrors.BadRequest("MERCHANT_AFFILIATE_RATE_OUT_OF_RANGE", "rate_percent must be within [0,100]")
	}
	if err := s.repo.UpdateRebateConfig(ctx, merchantID, enabled, ratePercent); err != nil {
		return err
	}
	s.InvalidateMerchant(merchantID)
	return nil
}

// GetDownlineStats 子用户视图：该用户在其商户下的下线数与累计返利。
func (s *MerchantAffiliateRebateService) GetDownlineStats(ctx context.Context, inviterUserID int64) (*MerchantAffiliateDownlineStats, error) {
	if s == nil || s.repo == nil {
		return &MerchantAffiliateDownlineStats{}, nil
	}
	stats, err := s.repo.DownlineStats(ctx, inviterUserID)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		return &MerchantAffiliateDownlineStats{}, nil
	}
	return stats, nil
}

// SubUserInviteOverview 子用户邀请页所需的聚合信息。
type SubUserInviteOverview struct {
	AffCode      string  `json:"aff_code"`
	InviteeCount int64   `json:"invitee_count"`
	TotalRebate  float64 `json:"total_rebate"`
	Enabled      bool    `json:"enabled"`      // 全局功能 + 该商户都开了才 true
	RatePercent  float64 `json:"rate_percent"` // 该商户返利比例
}

// GetSubUserInviteOverview 组装商户子用户邀请页：邀请码、下线数、累计返利、是否开启+比例。
func (s *MerchantAffiliateRebateService) GetSubUserInviteOverview(ctx context.Context, userID, merchantID int64) (*SubUserInviteOverview, error) {
	if s == nil || s.repo == nil {
		return &SubUserInviteOverview{}, nil
	}
	ov := &SubUserInviteOverview{}
	code, err := s.repo.GetAffCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	ov.AffCode = code

	if stats, err := s.repo.DownlineStats(ctx, userID); err != nil {
		return nil, err
	} else if stats != nil {
		ov.InviteeCount = stats.InviteeCount
		ov.TotalRebate = stats.TotalRebate
	}

	if cfg, err := s.repo.LoadRebateConfig(ctx, merchantID); err != nil {
		return nil, err
	} else if cfg != nil {
		ov.RatePercent = cfg.RatePercent
		ov.Enabled = s.enabled() && cfg.Enabled
	}
	return ov, nil
}
