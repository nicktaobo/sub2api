// AffiliateRebateWorker 邀请返利消费侧异步入账 worker（仿 merchant_earnings_worker）。
//
// 工作流（每 5s 一轮）：
//  1. BEGIN; SELECT outbox rows FOR UPDATE SKIP LOCKED LIMIT batch
//  2. 在事务内按 (inviter_id, invitee_user_id) 聚合，对每组：
//     - 查 invitee.created_at 用于 duration 校验
//     - 查 inviter←invitee 已累计返利用于 cap 截断
//     - UPDATE user_affiliates / INSERT user_affiliate_ledger
//  3. 不论结果如何，把这批 outbox 行 mark processed（不留尾巴）
//  4. COMMIT
//
// Tick 设为 5s 而非 merchant 的 1s：消费返利对时效不敏感，且批越大聚合越省。
// 关停时机仿 merchant：cancel ctx 后跑完最后一批再 return。

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// AffiliateRebateWorker 消费返利后台 worker。
type AffiliateRebateWorker struct {
	cfg            *config.Config
	db             *sql.DB
	outboxRepo     AffiliateConsumeOutboxRepository
	settingService *SettingService
	tickEvery      time.Duration
	batchLimit     int

	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

// NewAffiliateRebateWorker DI 构造函数。
func NewAffiliateRebateWorker(cfg *config.Config, db *sql.DB, outboxRepo AffiliateConsumeOutboxRepository, settingService *SettingService) *AffiliateRebateWorker {
	return &AffiliateRebateWorker{
		cfg:            cfg,
		db:             db,
		outboxRepo:     outboxRepo,
		settingService: settingService,
		tickEvery:      5 * time.Second,
		batchLimit:     1000,
	}
}

// Start 启动后台 goroutine（DI 装配后调用一次）。
func (w *AffiliateRebateWorker) Start() {
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
func (w *AffiliateRebateWorker) Stop() {
	if w == nil || !w.started || w.cancel == nil {
		return
	}
	w.cancel()
	if w.done != nil {
		<-w.done
	}
}

// Run 主循环。
func (w *AffiliateRebateWorker) Run(ctx context.Context) {
	if w == nil || w.db == nil {
		slog.Warn("affiliate rebate worker not initialized; skipping")
		return
	}
	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()
	slog.Info("affiliate rebate worker started", "tick", w.tickEvery)
	for {
		select {
		case <-ctx.Done():
			// 完成最后一批再退出，避免 outbox 残留
			_ = w.processBatch(context.Background())
			slog.Info("affiliate rebate worker stopped")
			return
		case <-ticker.C:
			// 总开关 + 子开关都关，且没有积压才跳过
			if !w.shouldRun(ctx) {
				continue
			}
			if err := w.processBatch(ctx); err != nil {
				slog.Error("affiliate rebate worker batch failed", "error", err)
			}
		}
	}
}

// shouldRun 决定本轮要不要处理。开关全关时若没有积压则跳过，留个出口让运维 flag 关闭后排清。
func (w *AffiliateRebateWorker) shouldRun(ctx context.Context) bool {
	if w.settingService == nil {
		return false
	}
	enabled := w.settingService.IsAffiliateEnabled(ctx) && w.settingService.IsAffiliateConsumeRebateEnabled(ctx)
	if enabled {
		return true
	}
	if w.outboxRepo == nil {
		return false
	}
	pending, err := w.outboxRepo.HasPending(ctx)
	if err != nil {
		slog.Warn("affiliate rebate worker: hasPending failed", "error", err)
		return false
	}
	return pending
}

// processBatch 单事务处理一批。
func (w *AffiliateRebateWorker) processBatch(ctx context.Context) (err error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := selectPendingAffOutboxForUpdate(ctx, tx, w.batchLimit)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		_ = tx.Rollback()
		return nil
	}

	// 一批用同一份 settings，跨 group 一致
	durationDays := 0
	perInviteeCap := 0.0
	if w.settingService != nil {
		durationDays = w.settingService.GetAffiliateRebateDurationDays(ctx)
		perInviteeCap = w.settingService.GetAffiliateRebatePerInviteeCap(ctx)
	}

	groups := aggregateAffOutbox(rows)
	for _, g := range groups {
		if err = processAffOutboxGroup(ctx, tx, g, durationDays, perInviteeCap); err != nil {
			return err
		}
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err = markAffOutboxProcessed(ctx, tx, ids); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	slog.Info("affiliate rebate worker batch processed",
		"rows", len(rows), "groups", len(groups))
	return nil
}

// ----------------------------------------------------------------------------
// SQL helpers
// ----------------------------------------------------------------------------

type affOutboxRow struct {
	ID            int64
	InviterID     int64
	InviteeUserID int64
	Amount        float64
}

type affOutboxGroup struct {
	InviterID     int64
	InviteeUserID int64
	Amount        float64
}

func selectPendingAffOutboxForUpdate(ctx context.Context, tx *sql.Tx, limit int) ([]affOutboxRow, error) {
	const q = `
		SELECT id, inviter_id, invitee_user_id, amount
		FROM user_affiliate_consumption_outbox
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
	out := make([]affOutboxRow, 0, limit)
	for rs.Next() {
		var r affOutboxRow
		if err := rs.Scan(&r.ID, &r.InviterID, &r.InviteeUserID, &r.Amount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// aggregateAffOutbox 按 (inviter_id, invitee_user_id) 聚合，便于一次 AccrueQuota 调用处理一对人。
func aggregateAffOutbox(rows []affOutboxRow) []affOutboxGroup {
	type key struct {
		inviter, invitee int64
	}
	bucket := make(map[key]float64, len(rows))
	for _, r := range rows {
		k := key{r.InviterID, r.InviteeUserID}
		bucket[k] += r.Amount
	}
	out := make([]affOutboxGroup, 0, len(bucket))
	for k, amt := range bucket {
		out = append(out, affOutboxGroup{InviterID: k.inviter, InviteeUserID: k.invitee, Amount: amt})
	}
	return out
}

// processAffOutboxGroup 处理一组 (inviter, invitee) 聚合：
//   - duration 窗口过期 → 跳过（amount=0）
//   - cap 截断 → 用 remaining 入账
//   - 否则按聚合 amount 入账
//
// 所有 SQL 走调用方的 *sql.Tx，与 outbox claim 在同一事务。
func processAffOutboxGroup(ctx context.Context, tx *sql.Tx, g affOutboxGroup, durationDays int, perInviteeCap float64) error {
	if g.Amount <= 0 || math.IsNaN(g.Amount) || math.IsInf(g.Amount, 0) {
		return nil
	}

	// 1) duration 检查：读 invitee 注册时间，超期跳过
	if durationDays > 0 {
		var createdAt time.Time
		err := tx.QueryRowContext(ctx,
			`SELECT created_at FROM user_affiliates WHERE user_id = $1`,
			g.InviteeUserID,
		).Scan(&createdAt)
		if errors.Is(err, sql.ErrNoRows) {
			// invitee 的 affiliate row 不存在，理论上不会发生（消费前一定 ensure 过）
			return nil
		}
		if err != nil {
			return fmt.Errorf("query invitee created_at: %w", err)
		}
		if time.Now().After(createdAt.AddDate(0, 0, durationDays)) {
			return nil
		}
	}

	amount := g.Amount

	// 2) per-invitee cap 截断（与充值返利共用同一条 cap）
	if perInviteeCap > 0 {
		var existing float64
		err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0)::double precision
			 FROM user_affiliate_ledger
			 WHERE user_id = $1 AND source_user_id = $2 AND action = 'accrue'`,
			g.InviterID, g.InviteeUserID,
		).Scan(&existing)
		if err != nil {
			return fmt.Errorf("query existing accrued: %w", err)
		}
		if existing >= perInviteeCap {
			return nil
		}
		if remaining := perInviteeCap - existing; amount > remaining {
			amount = remaining
		}
	}

	amount = roundTo(amount, 8)
	if amount <= 0 {
		return nil
	}

	// 3) 入账：UPDATE user_affiliates + INSERT ledger，与 AccrueConsumptionQuota 同语义
	res, err := tx.ExecContext(ctx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_history_quota = aff_history_quota + $1,
    updated_at = NOW()
WHERE user_id = $2`, amount, g.InviterID)
	if err != nil {
		return fmt.Errorf("update inviter quota: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// inviter 的 affiliate row 缺失，跳过这一组
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger
    (user_id, action, amount, source_user_id, source_order_id, source_type, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, NULL, 'consume', NOW(), NOW())`,
		g.InviterID, amount, g.InviteeUserID); err != nil {
		return fmt.Errorf("insert consume accrue ledger: %w", err)
	}
	return nil
}

func markAffOutboxProcessed(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
UPDATE user_affiliate_consumption_outbox
SET processed = TRUE, processed_at = NOW()
WHERE id = ANY($1)`, int64ArrayParam(ids))
	if err != nil {
		return fmt.Errorf("mark affiliate outbox processed: %w", err)
	}
	return nil
}
