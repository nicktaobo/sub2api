package repository

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// merchantAffiliateRepository 代理下级邀请返利（MERCHANT-AFFILIATE v1.0）的 raw-SQL repo。
// 消费侧 outbox 的写入走 usage_billing_repo 的内联 SQL（同事务）；本 repo 负责 hook 查询、
// 商户配置读写、注册绑定、子用户视图统计，以及 worker 起停的积压判断。
type merchantAffiliateRepository struct {
	db *sql.DB
}

// NewMerchantAffiliateRepository 主 repo（hook 查询 + 绑定 + 配置 + 统计）。
func NewMerchantAffiliateRepository(_ *dbent.Client, db *sql.DB) service.MerchantAffiliateRepository {
	return &merchantAffiliateRepository{db: db}
}

// NewMerchantAffiliateOutboxRepository worker 起停判断用（HasPending）。
// 与主 repo 共用实现，无状态，两个实例无副作用。
func NewMerchantAffiliateOutboxRepository(_ *dbent.Client, db *sql.DB) service.MerchantAffiliateOutboxRepository {
	return &merchantAffiliateRepository{db: db}
}

// LookupBindingForInvitee 反查某被邀请人的邀请绑定；无绑定返回 (nil, nil)。
func (r *merchantAffiliateRepository) LookupBindingForInvitee(ctx context.Context, inviteeUserID int64) (*service.MerchantAffiliateBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant affiliate repo db is nil")
	}
	var b service.MerchantAffiliateBinding
	err := r.db.QueryRowContext(ctx, `
		SELECT inviter_user_id, merchant_id
		FROM merchant_affiliate_bindings
		WHERE invitee_user_id = $1
	`, inviteeUserID).Scan(&b.InviterUserID, &b.MerchantID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// LoadRebateConfig 读某商户的返利开关+比例；商户不存在/软删返回 (nil, nil)。
func (r *merchantAffiliateRepository) LoadRebateConfig(ctx context.Context, merchantID int64) (*service.MerchantAffiliateRebateConfig, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant affiliate repo db is nil")
	}
	var cfg service.MerchantAffiliateRebateConfig
	err := r.db.QueryRowContext(ctx, `
		SELECT affiliate_rebate_enabled, affiliate_rebate_rate_percent
		FROM merchants
		WHERE id = $1 AND deleted_at IS NULL
	`, merchantID).Scan(&cfg.Enabled, &cfg.RatePercent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateRebateConfig 写某商户的返利开关+比例。
func (r *merchantAffiliateRepository) UpdateRebateConfig(ctx context.Context, merchantID int64, enabled bool, ratePercent float64) error {
	if r == nil || r.db == nil {
		return errors.New("merchant affiliate repo db is nil")
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE merchants
		SET affiliate_rebate_enabled = $2,
		    affiliate_rebate_rate_percent = $3,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, merchantID, enabled, ratePercent)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrMerchantNotFound
	}
	return nil
}

// ResolveSubUserByAffCode 把邀请码解析成"该商户下的一个活跃子用户"。
// 跨商户 / 平台用户 / 无效码 / 软删 → (0, false, nil)。
func (r *merchantAffiliateRepository) ResolveSubUserByAffCode(ctx context.Context, affCode string, merchantID int64) (int64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, errors.New("merchant affiliate repo db is nil")
	}
	var userID int64
	err := r.db.QueryRowContext(ctx, `
		SELECT ua.user_id
		FROM user_affiliates ua
		JOIN users u ON u.id = ua.user_id
		WHERE ua.aff_code = $1
		  AND u.parent_merchant_id = $2
		  AND u.status = 'active'
		  AND u.deleted_at IS NULL
	`, affCode, merchantID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
}

// CreateBinding 建立单层绑定；invitee 已绑定时 ON CONFLICT DO NOTHING，返回 created=false。
func (r *merchantAffiliateRepository) CreateBinding(ctx context.Context, merchantID, inviterUserID, inviteeUserID int64) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("merchant affiliate repo db is nil")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO merchant_affiliate_bindings (merchant_id, inviter_user_id, invitee_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (invitee_user_id) DO NOTHING
	`, merchantID, inviterUserID, inviteeUserID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DownlineStats 邀请人在其商户下的下线数与累计已入账返利。
func (r *merchantAffiliateRepository) DownlineStats(ctx context.Context, inviterUserID int64) (*service.MerchantAffiliateDownlineStats, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("merchant affiliate repo db is nil")
	}
	var stats service.MerchantAffiliateDownlineStats
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM merchant_affiliate_bindings WHERE inviter_user_id = $1
	`, inviterUserID).Scan(&stats.InviteeCount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM merchant_affiliate_consumption_outbox
		WHERE inviter_user_id = $1 AND processed = TRUE
	`, inviterUserID).Scan(&stats.TotalRebate); err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetAffCode 取某用户的邀请码；无 affiliate 行返回 ("", nil)。
func (r *merchantAffiliateRepository) GetAffCode(ctx context.Context, userID int64) (string, error) {
	if r == nil || r.db == nil {
		return "", errors.New("merchant affiliate repo db is nil")
	}
	var code string
	err := r.db.QueryRowContext(ctx, `
		SELECT aff_code FROM user_affiliates WHERE user_id = $1
	`, userID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return code, nil
}

// HasPending 满足 MerchantAffiliateOutboxRepository（worker 起停判断）。
func (r *merchantAffiliateRepository) HasPending(ctx context.Context) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("merchant affiliate repo db is nil")
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM merchant_affiliate_consumption_outbox WHERE processed = FALSE LIMIT 1)
	`).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
