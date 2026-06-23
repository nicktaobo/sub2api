// MERCHANT-SYSTEM v3.0
// PaymentService 充值 hook：仅保留 owner 自充 outbox 写入。
//
// v3.0 起子用户充值不再给 owner 分成，sub_user 充值订单走完支付流程后直接 return nil。
// owner 自充仍要写 outbox 让 worker 落 ledger（self_recharge），保对账等式。
//
// 关键设计（RFC §5.2.4）：
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

// applyMerchantSelfRechargeForOrder owner 自充池子（异步 outbox）（RFC §5.2.4）。
//
// 触发：order.user 是某商户的 owner 且 amount>0。
// 余额已由 redeem 加（worker 不重复加，详见 §5.2.3 P1-A）。
//
// 阻塞性：
//   - INTENT 写失败 → 阻塞（包成 MerchantBlockingError）
//   - outbox 写失败 → 非阻塞（普通 error，caller slog.Warn）
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

// applyMerchantHookForOrder 通用入口：owner 自充走 self_recharge outbox；
// sub_user 充值不再产生 owner 分成（v3.0），直接 return nil。
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
		// sub_user 充值：v3.0 不再分成
		return nil
	}
	return s.applyMerchantSelfRechargeForOrder(ctx, o)
}
