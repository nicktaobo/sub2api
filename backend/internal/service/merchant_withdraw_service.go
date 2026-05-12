// MERCHANT-SYSTEM v1.0
// 商户提现 + 分成统计。

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchantwithdrawrequest"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// MerchantStats 商户分成统计。
type MerchantStats struct {
	TotalRecharge    float64 `json:"total_recharge"`    // sub_user 累计充值
	TotalShare       float64 `json:"total_share"`       // 累计分成
	WithdrawnAmount  float64 `json:"withdrawn_amount"`  // 已提现 paid
	PendingWithdraw  float64 `json:"pending_withdraw"`  // 审核中 (pending+approved)
	AvailableBalance float64 `json:"available_balance"` // 可提现
}

func (s *MerchantService) sqlDB() (*sql.DB, error) {
	if s.entClient == nil {
		return nil, errors.New("ent client nil")
	}
	drv, ok := s.entClient.Driver().(*entsql.Driver)
	if !ok {
		return nil, errors.New("ent driver is not *entsql.Driver")
	}
	return drv.DB(), nil
}

// GetMerchantStats 计算商户分成统计。
func (s *MerchantService) GetMerchantStats(ctx context.Context, merchantID int64) (*MerchantStats, error) {
	if !s.enabled() {
		return nil, ErrMerchantInvalidParam
	}
	db, err := s.sqlDB()
	if err != nil {
		return nil, err
	}
	stats := &MerchantStats{}

	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(po.amount), 0)::float8
		FROM payment_orders po
		JOIN users u ON u.id = po.user_id
		WHERE u.parent_merchant_id = $1
		  AND po.status IN ('completed', 'paid')
		  AND po.order_type = 'balance'
	`, merchantID).Scan(&stats.TotalRecharge); err != nil {
		return nil, err
	}

	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)::float8
		FROM merchant_ledger
		WHERE merchant_id = $1
		  AND direction = 'credit'
		  AND source IN ('user_recharge_share', 'user_markup_share', 'self_recharge')
	`, merchantID).Scan(&stats.TotalShare); err != nil {
		return nil, err
	}

	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)::float8
		FROM merchant_withdraw_requests
		WHERE merchant_id = $1 AND status = 'paid'
	`, merchantID).Scan(&stats.WithdrawnAmount); err != nil {
		return nil, err
	}

	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)::float8
		FROM merchant_withdraw_requests
		WHERE merchant_id = $1 AND status IN ('pending', 'approved')
	`, merchantID).Scan(&stats.PendingWithdraw); err != nil {
		return nil, err
	}

	stats.AvailableBalance = stats.TotalShare - stats.WithdrawnAmount - stats.PendingWithdraw
	if stats.AvailableBalance < 0 {
		stats.AvailableBalance = 0
	}
	return stats, nil
}

// CreateWithdrawInput owner 申请提现。
type CreateWithdrawInput struct {
	MerchantID     int64
	Amount         float64
	PaymentMethod  string
	PaymentAccount string
	PaymentName    string
	Note           string
}

func (s *MerchantService) CreateWithdrawRequest(ctx context.Context, in CreateWithdrawInput) (*dbent.MerchantWithdrawRequest, error) {
	if !s.enabled() {
		return nil, ErrMerchantInvalidParam
	}
	if in.Amount <= 0 {
		return nil, infraerrors.BadRequest("MERCHANT_WITHDRAW_INVALID_AMOUNT", "amount must be > 0")
	}
	method := strings.ToLower(strings.TrimSpace(in.PaymentMethod))
	switch method {
	case "alipay", "wechat", "bank", "usdt", "other":
	default:
		return nil, infraerrors.BadRequest("MERCHANT_WITHDRAW_BAD_METHOD", "invalid payment_method")
	}
	if strings.TrimSpace(in.PaymentAccount) == "" || strings.TrimSpace(in.PaymentName) == "" {
		return nil, infraerrors.BadRequest("MERCHANT_WITHDRAW_REQUIRED", "payment_account and payment_name required")
	}
	stats, err := s.GetMerchantStats(ctx, in.MerchantID)
	if err != nil {
		return nil, err
	}
	if in.Amount > stats.AvailableBalance {
		return nil, infraerrors.BadRequest("MERCHANT_WITHDRAW_EXCEEDS_AVAILABLE",
			fmt.Sprintf("amount %.2f exceeds available balance %.2f", in.Amount, stats.AvailableBalance))
	}

	// 同时校验 owner.balance —— available_balance 是"累计分成 - 已提现 - 审核中"，
	// 不反映商户当前可动用的真实余额（商户可能把分成都给子用户充值掉了）。
	// 这里再卡一刀：申请提现金额必须 ≤ owner 用户当前余额。
	// 申请阶段不扣余额，所以这只是 UX 兜底（避免挂着永远批不下来的请求）；
	// 真正的扣款原子校验在 AdminApproveWithdrawal 走 DeductBalanceStrict。
	m, err := s.repo.GetByID(ctx, in.MerchantID)
	if err != nil {
		return nil, err
	}
	owner, err := s.userRepo.GetByID(ctx, m.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if in.Amount > owner.Balance {
		return nil, infraerrors.BadRequest("MERCHANT_WITHDRAW_EXCEEDS_BALANCE",
			fmt.Sprintf("amount %.2f exceeds current balance %.2f", in.Amount, owner.Balance))
	}

	w, err := s.entClient.MerchantWithdrawRequest.Create().
		SetMerchantID(in.MerchantID).
		SetAmount(in.Amount).
		SetStatus("pending").
		SetPaymentMethod(method).
		SetPaymentAccount(strings.TrimSpace(in.PaymentAccount)).
		SetPaymentName(strings.TrimSpace(in.PaymentName)).
		SetNote(in.Note).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.writeAudit(ctx, in.MerchantID, 0, "withdraw_request", "",
		fmt.Sprintf("amount=%g method=%s", in.Amount, method), in.Note)
	return w, nil
}

func (s *MerchantService) ListWithdrawRequests(ctx context.Context, merchantID int64, status string, offset, limit int) ([]*dbent.MerchantWithdrawRequest, int, error) {
	q := s.entClient.MerchantWithdrawRequest.Query().Where(merchantwithdrawrequest.MerchantIDEQ(merchantID))
	if status != "" {
		q = q.Where(merchantwithdrawrequest.StatusEQ(status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.Order(dbent.Desc(merchantwithdrawrequest.FieldCreatedAt)).
		Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *MerchantService) AdminListWithdrawals(ctx context.Context, status string, merchantID int64, offset, limit int) ([]*dbent.MerchantWithdrawRequest, int, error) {
	q := s.entClient.MerchantWithdrawRequest.Query()
	if status != "" {
		q = q.Where(merchantwithdrawrequest.StatusEQ(status))
	}
	if merchantID > 0 {
		q = q.Where(merchantwithdrawrequest.MerchantIDEQ(merchantID))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.Order(dbent.Desc(merchantwithdrawrequest.FieldCreatedAt)).
		Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// AdminApproveWithdrawal admin 批准 + 打款：扣 owner.balance + 写 ledger debit + 状态 paid。
func (s *MerchantService) AdminApproveWithdrawal(ctx context.Context, withdrawID, adminID int64) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	w, err := s.entClient.MerchantWithdrawRequest.Get(ctx, withdrawID)
	if err != nil {
		return err
	}
	if w.Status != "pending" && w.Status != "approved" {
		return infraerrors.BadRequest("MERCHANT_WITHDRAW_INVALID_STATUS",
			"withdrawal cannot be approved in status "+w.Status)
	}
	m, err := s.repo.GetByID(ctx, w.MerchantID)
	if err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	// 提现：商户余额不足时 ErrInsufficientBalance 中断（避免审批通过后扣成负数）
	if err := s.userRepo.DeductBalanceStrict(txCtx, m.OwnerUserID, w.Amount); err != nil {
		return err
	}
	bal, err := s.readOwnerBalanceInTx(txCtx, m.OwnerUserID)
	if err != nil {
		return err
	}
	idem := fmt.Sprintf("withdraw:%d", w.ID)
	ledgerEntry := &MerchantLedgerEntry{
		MerchantID:     w.MerchantID,
		OwnerUserID:    m.OwnerUserID,
		Direction:      MerchantLedgerDirectionDebit,
		Amount:         w.Amount,
		BalanceAfter:   &bal,
		Source:         "withdraw",
		IdempotencyKey: &idem,
	}
	if err := s.ledgerRepo.Insert(txCtx, ledgerEntry); err != nil {
		return err
	}

	now := time.Now()
	upd := tx.Client().MerchantWithdrawRequest.UpdateOneID(w.ID).
		SetStatus("paid").
		SetLedgerID(ledgerEntry.ID).
		SetProcessedAt(now)
	if adminID > 0 {
		upd = upd.SetAdminID(adminID)
	}
	if _, err := upd.Save(txCtx); err != nil {
		return err
	}

	if err := s.writeAudit(txCtx, w.MerchantID, adminID, "withdraw_approved", "",
		fmt.Sprintf("withdraw_id=%d amount=%g", w.ID, w.Amount), ""); err != nil {
		return err
	}
	return tx.Commit()
}

// AdminRejectWithdrawal admin 拒绝提现。
func (s *MerchantService) AdminRejectWithdrawal(ctx context.Context, withdrawID, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	w, err := s.entClient.MerchantWithdrawRequest.Get(ctx, withdrawID)
	if err != nil {
		return err
	}
	if w.Status != "pending" && w.Status != "approved" {
		return infraerrors.BadRequest("MERCHANT_WITHDRAW_INVALID_STATUS",
			"withdrawal cannot be rejected in status "+w.Status)
	}
	now := time.Now()
	upd := s.entClient.MerchantWithdrawRequest.UpdateOneID(w.ID).
		SetStatus("rejected").
		SetRejectReason(reason).
		SetProcessedAt(now)
	if adminID > 0 {
		upd = upd.SetAdminID(adminID)
	}
	if _, err := upd.Save(ctx); err != nil {
		return err
	}
	_ = s.writeAudit(ctx, w.MerchantID, adminID, "withdraw_rejected",
		fmt.Sprintf("withdraw_id=%d amount=%g", w.ID, w.Amount), "", reason)
	return nil
}
