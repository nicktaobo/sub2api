// MERCHANT-SYSTEM v1.0
// PaymentService 的两个充值 hook：
//   - applyMerchantRechargeShareForOrder：sub_user 充值，给 owner 分成（异步 outbox）
//   - applyMerchantSelfRechargeForOrder：owner 自充池子（异步 outbox）
//
// 关键设计（RFC §5.2.2 / §5.2.4）：
//   - INTENT audit 写失败 → 阻塞 markCompleted（返回 *MerchantBlockingError{Stage:"intent_write"}）
//   - outbox 写失败 → 非阻塞，订单仍 markCompleted，reconcile job 兜底
//   - INTENT audit 用 writePaymentAuditLogStrict（强持久化，不是 best-effort）
//   - outbox 用 InsertIfNotExists（idempotency_key UNIQUE，幂等）

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// applyMerchantRechargeShareForOrder sub_user 充值，给 owner 分成（异步 outbox）（RFC §5.2.2）。
//
// 触发：order.user 是某商户的 sub_user 且 merchant.status=active 且 discount<1.0。
// 短路：feature flag off / order 非 balance 类型 / amount<=0 / merchant 不存在/suspended/discount=1.0。
//
// 阻塞性：
//   - INTENT 写失败 → 阻塞（包成 MerchantBlockingError）
//   - outbox 写失败 → 非阻塞（普通 error，caller slog.Warn）
func (s *PaymentService) applyMerchantRechargeShareForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if o == nil || o.OrderType != payment.OrderTypeBalance || o.Amount <= 0 {
		return nil
	}
	// v1.13 P1-#1：PaymentService 上下文用 s.merchantCfg.Enabled
	if !s.merchantCfg.Enabled {
		return nil
	}
	if s.merchantRepo == nil || s.merchantOutboxRepo == nil {
		return nil
	}

	user, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		return nil // 不阻塞：用户查不到也无法分成
	}
	if user.ParentMerchantID == nil {
		return nil // 不是子用户
	}

	m, err := s.merchantRepo.GetByID(ctx, *user.ParentMerchantID)
	if err != nil || m == nil {
		return nil // merchant 不存在
	}
	if m.Status != MerchantStatusActive {
		return nil // suspended → 不分成（RFC §5.2.2 边界表）
	}
	if m.Discount >= 1.0 {
		return nil // discount=1 表示不分成（DB CHECK 保证 ≤1）
	}

	shareAmount := o.Amount * (1.0 - m.Discount)
	if shareAmount <= 0 {
		return nil
	}

	// 步骤 1：独立事务先提交 INTENT audit（即使后续 outbox insert 失败 INTENT 也存活）
	// v1.10 P1-B：必须用 writePaymentAuditLogStrict（返回 error）
	// v1.12 P1-#1：失败必须包成 MerchantBlockingError
	if err := s.writePaymentAuditLogStrict(ctx, o.ID, "MERCHANT_RECHARGE_SHARE_INTENT", "system", map[string]any{
		"merchant_id":  m.ID,
		"sub_user_id":  o.UserID,
		"share_amount": shareAmount,
		"discount":     m.Discount,
		"order_amount": o.Amount,
	}); err != nil {
		return &MerchantBlockingError{Stage: "intent_write", Err: err}
	}

	// 步骤 2：独立 INSERT outbox。靠 idempotency_key UNIQUE（v1.10 P1-C）
	subID := o.UserID
	err = s.merchantOutboxRepo.InsertIfNotExists(ctx, &MerchantOutboxEntry{
		MerchantID:         m.ID,
		CounterpartyUserID: &subID,
		Amount:             shareAmount,
		Source:             MerchantSourceUserRechargeShare,
		RefType:            MerchantRefTypePaymentOrder,
		RefID:              o.ID,
		IdempotencyKey:     fmt.Sprintf("recharge_share:%d", o.ID),
	})
	if errors.Is(err, ErrMerchantOutboxAlreadyExists) {
		return nil // 已经写过（重试），幂等成功
	}
	if err != nil {
		return err // outbox 失败：INTENT 已落库，reconcile 兜底
	}
	return nil
}

// applyMerchantSelfRechargeForOrder owner 自充池子（异步 outbox）（RFC §5.2.4）。
//
// 触发：order.user 是某商户的 owner 且 amount>0。
// 余额已由 redeem 加（worker 不重复加，详见 §5.2.3 P1-A）。
// 阻塞性同 applyMerchantRechargeShareForOrder。
func (s *PaymentService) applyMerchantSelfRechargeForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if o == nil || o.OrderType != payment.OrderTypeBalance || o.Amount <= 0 {
		return nil
	}
	if !s.merchantCfg.Enabled {
		return nil
	}
	if s.merchantRepo == nil || s.merchantOutboxRepo == nil {
		return nil
	}

	m, err := s.merchantRepo.GetByOwnerUserID(ctx, o.UserID)
	if err != nil || m == nil {
		return nil // 不是 owner / merchant 不存在或已删除
	}
	if m.DeletedAt != nil {
		return nil
	}

	// 不论 status 都写——RFC §5.3.1：suspended owner 自充仍写 ledger 保对账
	if err := s.writePaymentAuditLogStrict(ctx, o.ID, "MERCHANT_SELF_RECHARGE_INTENT", "system", map[string]any{
		"merchant_id":     m.ID,
		"owner_user_id":   m.OwnerUserID,
		"credited_amount": o.Amount,
		"pay_amount":      o.PayAmount,
		"discount":        m.Discount,
	}); err != nil {
		return &MerchantBlockingError{Stage: "intent_write", Err: err}
	}

	err = s.merchantOutboxRepo.InsertIfNotExists(ctx, &MerchantOutboxEntry{
		MerchantID:     m.ID,
		Amount:         o.Amount,
		Source:         MerchantSourceSelfRecharge,
		RefType:        MerchantRefTypePaymentOrder,
		RefID:          o.ID,
		IdempotencyKey: fmt.Sprintf("self_recharge:%d", o.ID),
	})
	if errors.Is(err, ErrMerchantOutboxAlreadyExists) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

// applyMerchantHookForOrder 通用入口：根据 order.user 角色派发到对应的 hook。
//
// 调用方应处理：
//   - MerchantBlockingError → 阻塞 markCompleted（return 让 executeFulfillment markFailed）
//   - 普通 error → 非阻塞，slog.Warn，订单仍 markCompleted（reconcile 兜底）
//   - nil → 已处理或不适用，继续
func (s *PaymentService) applyMerchantHookForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if o == nil || !s.merchantCfg.Enabled {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		// 用户查不到不阻塞，避免拖累订单完成
		slog.Warn("merchant hook: user lookup failed (non-blocking)", "order_id", o.ID, "user_id", o.UserID, "error", err)
		return nil
	}
	if user.ParentMerchantID != nil {
		return s.applyMerchantRechargeShareForOrder(ctx, o)
	}
	return s.applyMerchantSelfRechargeForOrder(ctx, o)
}
