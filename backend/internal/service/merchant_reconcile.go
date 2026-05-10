// MERCHANT-SYSTEM v1.0
// MerchantReconcileJob 兜底任务：扫已落库 INTENT audit 但 outbox 漏写的订单，
// 用快照补 outbox 并写 RECONCILED audit 防重扫（RFC §5.2.2 reconcile pseudocode）。
//
// 关键原则：reconcile **不重新调用** hook，从 INTENT audit detail 直接读取金额快照
// 补 outbox（避免读取当前 discount 违反 P3 历史不可变）。

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// MerchantReconcileJob 兜底定时任务（每小时跑一次，单次最多 500 条）。
type MerchantReconcileJob struct {
	cfg            *config.Config
	db             *sql.DB
	outboxRepo     MerchantOutboxRepository
	paymentService *PaymentService

	tickEvery  time.Duration
	batchLimit int

	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

// NewMerchantReconcileJob DI 构造函数。
func NewMerchantReconcileJob(
	cfg *config.Config,
	db *sql.DB,
	outboxRepo MerchantOutboxRepository,
	paymentService *PaymentService,
) *MerchantReconcileJob {
	return &MerchantReconcileJob{
		cfg:            cfg,
		db:             db,
		outboxRepo:     outboxRepo,
		paymentService: paymentService,
		tickEvery:      time.Hour,
		batchLimit:     500,
	}
}

// Start 启动后台 goroutine。
func (j *MerchantReconcileJob) Start() {
	if j == nil || j.started {
		return
	}
	j.started = true
	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel
	j.done = make(chan struct{})
	go func() {
		defer close(j.done)
		j.Run(ctx)
	}()
}

// Stop 优雅关闭。
func (j *MerchantReconcileJob) Stop() {
	if j == nil || !j.started || j.cancel == nil {
		return
	}
	j.cancel()
	if j.done != nil {
		<-j.done
	}
}

// Run 启动 reconcile 主循环。
func (j *MerchantReconcileJob) Run(ctx context.Context) {
	if j == nil || j.db == nil {
		slog.Warn("merchant reconcile job not initialized; skipping")
		return
	}
	ticker := time.NewTicker(j.tickEvery)
	defer ticker.Stop()
	slog.Info("merchant reconcile job started", "tick", j.tickEvery)
	// 启动时先扫一轮
	if err := j.runOnce(ctx); err != nil {
		slog.Error("merchant reconcile initial run failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 例外白名单（RFC §1.0.1 v1.12 P2-3）：flag 关闭也跑（兜底已落库的 INTENT）
			if err := j.runOnce(ctx); err != nil {
				slog.Error("merchant reconcile run failed", "error", err)
			}
		}
	}
}

// runOnce 跑一轮 reconcile。
func (j *MerchantReconcileJob) runOnce(ctx context.Context) error {
	intents, err := j.scanPendingIntents(ctx)
	if err != nil {
		return err
	}
	if len(intents) == 0 {
		return nil
	}
	slog.Info("merchant reconcile: found intents to补", "count", len(intents))
	for _, in := range intents {
		if err := j.reconcileOne(ctx, in); err != nil {
			slog.Error("merchant reconcile one failed",
				"audit_id", in.AuditID, "order_id", in.OrderID, "error", err)
			continue
		}
	}
	if len(intents) == j.batchLimit {
		slog.Warn("merchant reconcile batch fully consumed; outbox writes may be persistently failing",
			"batch_limit", j.batchLimit)
	}
	return nil
}

type pendingIntent struct {
	AuditID int64
	OrderID int64
	Action  string
	Detail  string
}

func (j *MerchantReconcileJob) scanPendingIntents(ctx context.Context) ([]pendingIntent, error) {
	const q = `
		SELECT pal.id, pal.order_id, pal.action, pal.detail
		FROM payment_audit_logs pal
		WHERE pal.action IN ('MERCHANT_RECHARGE_SHARE_INTENT', 'MERCHANT_SELF_RECHARGE_INTENT')
		  AND NOT EXISTS (
			SELECT 1 FROM merchant_earnings_outbox o
			WHERE o.idempotency_key IN (
			  'recharge_share:' || pal.order_id,
			  'self_recharge:'  || pal.order_id
			)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM payment_audit_logs done
			WHERE done.order_id = pal.order_id
			  AND done.action IN ('MERCHANT_RECHARGE_SHARE_RECONCILED', 'MERCHANT_SELF_RECHARGE_RECONCILED')
		  )
		ORDER BY pal.created_at ASC
		LIMIT $1
	`
	rows, err := j.db.QueryContext(ctx, q, j.batchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pendingIntent, 0, j.batchLimit)
	for rows.Next() {
		var (
			in       pendingIntent
			orderStr string
		)
		if err := rows.Scan(&in.AuditID, &orderStr, &in.Action, &in.Detail); err != nil {
			return nil, err
		}
		oid, err := strconv.ParseInt(orderStr, 10, 64)
		if err != nil {
			slog.Warn("merchant reconcile: invalid order_id", "audit_id", in.AuditID, "order_id", orderStr)
			continue
		}
		in.OrderID = oid
		out = append(out, in)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type intentSnapshot struct {
	MerchantID     int64   `json:"merchant_id"`
	SubUserID      int64   `json:"sub_user_id,omitempty"`
	ShareAmount    float64 `json:"share_amount,omitempty"`
	OwnerUserID    int64   `json:"owner_user_id,omitempty"`
	CreditedAmount float64 `json:"credited_amount,omitempty"`
}

func (j *MerchantReconcileJob) reconcileOne(ctx context.Context, in pendingIntent) error {
	var s intentSnapshot
	if err := json.Unmarshal([]byte(in.Detail), &s); err != nil {
		return fmt.Errorf("decode intent detail: %w", err)
	}
	var entry *MerchantOutboxEntry
	var reconciledAction string
	switch in.Action {
	case "MERCHANT_RECHARGE_SHARE_INTENT":
		if s.MerchantID == 0 || s.ShareAmount <= 0 {
			return errors.New("invalid recharge_share INTENT snapshot")
		}
		sub := s.SubUserID
		entry = &MerchantOutboxEntry{
			MerchantID:         s.MerchantID,
			CounterpartyUserID: &sub,
			Amount:             s.ShareAmount,
			Source:             MerchantSourceUserRechargeShare,
			RefType:            MerchantRefTypePaymentOrder,
			RefID:              in.OrderID,
			IdempotencyKey:     fmt.Sprintf("recharge_share:%d", in.OrderID),
		}
		reconciledAction = "MERCHANT_RECHARGE_SHARE_RECONCILED"
	case "MERCHANT_SELF_RECHARGE_INTENT":
		if s.MerchantID == 0 || s.CreditedAmount <= 0 {
			return errors.New("invalid self_recharge INTENT snapshot")
		}
		entry = &MerchantOutboxEntry{
			MerchantID:     s.MerchantID,
			Amount:         s.CreditedAmount,
			Source:         MerchantSourceSelfRecharge,
			RefType:        MerchantRefTypePaymentOrder,
			RefID:          in.OrderID,
			IdempotencyKey: fmt.Sprintf("self_recharge:%d", in.OrderID),
		}
		reconciledAction = "MERCHANT_SELF_RECHARGE_RECONCILED"
	default:
		return fmt.Errorf("unexpected intent action: %s", in.Action)
	}

	outboxAlreadyExisted := false
	if err := j.outboxRepo.InsertIfNotExists(ctx, entry); err != nil {
		if errors.Is(err, ErrMerchantOutboxAlreadyExists) {
			outboxAlreadyExisted = true
		} else {
			return fmt.Errorf("insert outbox: %w", err)
		}
	}
	// 写 RECONCILED audit 防重扫（必须 strict）
	if err := j.paymentService.writePaymentAuditLogStrict(ctx, in.OrderID, reconciledAction, "reconcile",
		map[string]any{
			"intent_audit_id":        in.AuditID,
			"outbox_already_existed": outboxAlreadyExisted,
		}); err != nil {
		// 不返回 error：outbox 已写/已存在；audit 失败下轮会重试（幂等仍正确）
		slog.Error("merchant reconcile audit RECONCILED failed",
			"order_id", in.OrderID, "error", err)
	}
	return nil
}
