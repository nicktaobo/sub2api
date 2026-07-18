// MERCHANT-AFFILIATE v1.0
// MerchantAffiliateRebateWorker：代理下级邀请返利异步入账 worker（仿 affiliate_rebate_worker）。
//
// 工作流（每 5s 一轮）：
//  1. BEGIN; SELECT outbox rows FOR UPDATE SKIP LOCKED LIMIT batch
//  2. 按 inviter_user_id 聚合，给每个邀请人的 users.balance 加返利额
//  3. mark processed（不留尾巴）
//  4. COMMIT
//
// 与平台返利 worker 的差异：返利资金来自商户利润（已在消费 hook 里从 merchant markup
// 利润扣出），这里只负责把待入账额加到邀请人余额，不再动商户/平台任何账。
// 不做 duration / per-invitee cap（v1 单层，比例封顶已在 hook clamp）。

package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// MerchantAffiliateOutboxRepository worker 起停判断用（是否还有积压）。
type MerchantAffiliateOutboxRepository interface {
	HasPending(ctx context.Context) (bool, error)
}

// MerchantAffiliateRebateWorker 下级邀请返利后台入账 worker。
type MerchantAffiliateRebateWorker struct {
	cfg        *config.Config
	db         *sql.DB
	outboxRepo MerchantAffiliateOutboxRepository
	tickEvery  time.Duration
	batchLimit int

	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

// NewMerchantAffiliateRebateWorker DI 构造函数。
func NewMerchantAffiliateRebateWorker(cfg *config.Config, db *sql.DB, outboxRepo MerchantAffiliateOutboxRepository) *MerchantAffiliateRebateWorker {
	return &MerchantAffiliateRebateWorker{
		cfg:        cfg,
		db:         db,
		outboxRepo: outboxRepo,
		tickEvery:  5 * time.Second,
		batchLimit: 1000,
	}
}

// Start 启动后台 goroutine（DI 装配后调用一次）。
func (w *MerchantAffiliateRebateWorker) Start() {
	if w == nil || w.started {
		return
	}
	w.started = true
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		w.Run(ctx)
	}()
}

// Stop 优雅关停：cancel + 等最后一批跑完。
func (w *MerchantAffiliateRebateWorker) Stop() {
	if w == nil || !w.started || w.cancel == nil {
		return
	}
	w.cancel()
	if w.done != nil {
		<-w.done
	}
}

// Run 主循环。
func (w *MerchantAffiliateRebateWorker) Run(ctx context.Context) {
	if w == nil || w.db == nil {
		slog.Warn("merchant affiliate rebate worker not initialized; skipping")
		return
	}
	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()
	slog.Info("merchant affiliate rebate worker started", "tick", w.tickEvery)
	for {
		select {
		case <-ctx.Done():
			// 完成最后一批再退出，避免 outbox 残留
			_ = w.processBatch(context.Background())
			slog.Info("merchant affiliate rebate worker stopped")
			return
		case <-ticker.C:
			if !w.shouldRun(ctx) {
				continue
			}
			if err := w.processBatch(ctx); err != nil {
				slog.Error("merchant affiliate rebate worker batch failed", "error", err)
			}
		}
	}
}

// shouldRun 功能开着就跑；关了但仍有积压也跑（把队列排清），否则跳过。
func (w *MerchantAffiliateRebateWorker) shouldRun(ctx context.Context) bool {
	if w.cfg != nil && w.cfg.Merchant.Enabled && w.cfg.Merchant.AffiliateRebateEnabled {
		return true
	}
	if w.outboxRepo == nil {
		return false
	}
	pending, err := w.outboxRepo.HasPending(ctx)
	if err != nil {
		slog.Warn("merchant affiliate rebate worker: hasPending failed", "error", err)
		return false
	}
	return pending
}

// processBatch 单事务处理一批。
func (w *MerchantAffiliateRebateWorker) processBatch(ctx context.Context) (err error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := selectPendingMerchantAffOutboxForUpdate(ctx, tx, w.batchLimit)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		_ = tx.Rollback()
		return nil
	}

	// 按 inviter 分组（保留原始行，便于邀请人失踪时按行退款给 owner）。
	rowsByInviter := make(map[int64][]merchantAffOutboxRow, len(rows))
	for _, r := range rows {
		rowsByInviter[r.InviterUserID] = append(rowsByInviter[r.InviterUserID], r)
	}
	// 确定性加锁顺序（inviter id 升序），避免与其它事务/多实例形成 AB-BA 死锁。
	inviterIDs := make([]int64, 0, len(rowsByInviter))
	for id := range rowsByInviter {
		inviterIDs = append(inviterIDs, id)
	}
	sort.Slice(inviterIDs, func(i, j int) bool { return inviterIDs[i] < inviterIDs[j] })

	for _, inviterID := range inviterIDs {
		grp := rowsByInviter[inviterID]
		var sum float64
		for _, r := range grp {
			sum += r.Amount
		}
		amount := roundTo(sum, 8)
		if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			continue
		}
		res, e := tx.ExecContext(ctx, `
			UPDATE users SET balance = balance + $1 WHERE id = $2 AND deleted_at IS NULL
		`, amount, inviterID)
		if e != nil {
			return fmt.Errorf("credit inviter balance: %w", e)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// 邀请人已被删/软删：把每笔返利退回对应商户 owner（走 merchant_earnings_outbox，
			// 由 earnings worker 入账 + 记 ledger，保持对账等式 owner_net + inviter_rebate == profit）。
			for _, r := range grp {
				if e2 := insertOwnerRefundOutbox(ctx, tx, r); e2 != nil {
					return fmt.Errorf("refund rebate to owner: %w", e2)
				}
			}
			slog.Warn("merchant affiliate rebate worker: inviter missing, refunded to owner",
				"inviter_user_id", inviterID, "amount", amount)
		}
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err = markMerchantAffOutboxProcessed(ctx, tx, ids); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	slog.Info("merchant affiliate rebate worker batch processed",
		"rows", len(rows), "inviters", len(inviterIDs))
	return nil
}

// insertOwnerRefundOutbox 邀请人失踪时把这笔返利退回商户 owner：写一条 merchant_earnings_outbox
// （source=user_markup_share，语义即"未能路由给邀请人的那份 markup 利润回到 owner"），
// 由 MerchantEarningsWorker 入账并记 ledger，保持商户对账等式不破。
// 幂等键 maff_refund:{affRowID} 与原返利行一一对应；counterparty 记失踪的邀请人（软删时仍在，
// 硬删时其 outbox 行已被 CASCADE 清掉、不会走到这里）。
func insertOwnerRefundOutbox(ctx context.Context, tx *sql.Tx, r merchantAffOutboxRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO merchant_earnings_outbox
			(merchant_id, counterparty_user_id, amount, source, ref_type, ref_id, idempotency_key)
		VALUES ($1, $2, $3, 'user_markup_share', 'usage_billing_dedup', $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, r.MerchantID, r.InviterUserID, r.Amount, r.RefID, fmt.Sprintf("maff_refund:%d", r.ID))
	return err
}

type merchantAffOutboxRow struct {
	ID            int64
	MerchantID    int64
	InviterUserID int64
	Amount        float64
	RefID         int64
}

func selectPendingMerchantAffOutboxForUpdate(ctx context.Context, tx *sql.Tx, limit int) ([]merchantAffOutboxRow, error) {
	const q = `
		SELECT id, merchant_id, inviter_user_id, amount, ref_id
		FROM merchant_affiliate_consumption_outbox
		WHERE processed = FALSE
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rs, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	out := make([]merchantAffOutboxRow, 0, limit)
	for rs.Next() {
		var r merchantAffOutboxRow
		if err := rs.Scan(&r.ID, &r.MerchantID, &r.InviterUserID, &r.Amount, &r.RefID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func markMerchantAffOutboxProcessed(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE merchant_affiliate_consumption_outbox
		SET processed = TRUE, processed_at = NOW()
		WHERE id = ANY($1)
	`, int64ArrayParam(ids))
	if err != nil {
		return fmt.Errorf("mark merchant affiliate outbox processed: %w", err)
	}
	return nil
}
