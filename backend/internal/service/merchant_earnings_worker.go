// MERCHANT-SYSTEM v1.0
// MerchantEarningsWorker：异步聚合 outbox → ledger（RFC §5.2.3）。
//
// 单事务内：claim → process → mark，多副本通过 FOR UPDATE SKIP LOCKED 安全。
// source 行为分流（v1.10 P1-A）：
//   - user_markup_share / user_recharge_share：聚合 + 加 owner 余额
//   - self_recharge：逐笔写 ledger，**不**加余额（已由 redeem 加）

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// MerchantEarningsWorker 异步分润处理 worker。
type MerchantEarningsWorker struct {
	cfg        *config.Config
	db         *sql.DB
	outboxRepo MerchantOutboxRepository
	tickEvery  time.Duration
	batchLimit int

	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

// NewMerchantEarningsWorker DI 构造函数。
func NewMerchantEarningsWorker(cfg *config.Config, db *sql.DB, outboxRepo MerchantOutboxRepository) *MerchantEarningsWorker {
	return &MerchantEarningsWorker{
		cfg:        cfg,
		db:         db,
		outboxRepo: outboxRepo,
		tickEvery:  1 * time.Second,
		batchLimit: 1000,
	}
}

// Start 启动后台 goroutine（在 wire ProvideMerchantEarningsWorker 调用一次）。
func (w *MerchantEarningsWorker) Start() {
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

// Stop 优雅关闭：cancel ctx，等 Run 返回（包括处理完最后一批）。
func (w *MerchantEarningsWorker) Stop() {
	if w == nil || !w.started || w.cancel == nil {
		return
	}
	w.cancel()
	if w.done != nil {
		<-w.done
	}
}

// Run 启动 worker 主循环。SIGTERM 时优雅完成最后一批再退出（RFC 附录 A.2）。
func (w *MerchantEarningsWorker) Run(ctx context.Context) {
	if w == nil || w.db == nil {
		slog.Warn("merchant earnings worker not initialized; skipping")
		return
	}
	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()
	slog.Info("merchant earnings worker started",
		"enabled", w.cfg != nil && w.cfg.Merchant.Enabled,
		"tick", w.tickEvery)
	for {
		select {
		case <-ctx.Done():
			// 完成最后一批再退出（避免数据残留）
			_ = w.processBatch(context.Background())
			slog.Info("merchant earnings worker stopped")
			return
		case <-ticker.C:
			// 例外白名单（RFC §1.0.1 v1.12 P2-3）：flag 关闭后仍处理积压
			if w.cfg == nil || (!w.cfg.Merchant.Enabled && !w.hasPending(ctx)) {
				continue
			}
			if err := w.processBatch(ctx); err != nil {
				slog.Error("merchant earnings worker batch failed", "error", err)
			}
		}
	}
}

func (w *MerchantEarningsWorker) hasPending(ctx context.Context) bool {
	if w.outboxRepo == nil {
		return false
	}
	got, err := w.outboxRepo.HasPending(ctx)
	if err != nil {
		slog.Warn("merchant earnings worker: hasPending failed", "error", err)
		return false
	}
	return got
}

// processBatch 一轮处理：单事务 claim → 分流 → 写 ledger → mark processed → commit。
func (w *MerchantEarningsWorker) processBatch(ctx context.Context) (err error) {
	startedAt := time.Now()
	batchSize := 0
	defer func() {
		// Phase 7 metrics：每轮采样一次（即使 batchSize=0 也输出，便于看 worker 心跳）
		emitMerchantMetricsSample(ctx, w.db, batchSize, time.Since(startedAt))
	}()
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
	}()

	rows, err := selectPendingOutboxForUpdate(ctx, tx, w.batchLimit)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if len(rows) == 0 {
		_ = tx.Rollback()
		return nil
	}
	batchSize = len(rows)

	aggregatedGroups, perRowEntries := splitOutboxBySource(rows)

	// 2a. 聚合组（user_markup_share / user_recharge_share）
	for _, g := range aggregatedGroups {
		if err = processAggregatedGroup(ctx, tx, g); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	// 2b. 逐笔组（self_recharge）
	for _, e := range perRowEntries {
		if err = processSelfRechargeRow(ctx, tx, e); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	// 3. mark processed
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err = markOutboxProcessed(ctx, tx, ids); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	slog.Info("merchant earnings worker batch processed",
		"rows", len(rows),
		"agg_groups", len(aggregatedGroups),
		"per_row", len(perRowEntries))
	return nil
}

// ----------------------------------------------------------------------------
// SQL helpers
// ----------------------------------------------------------------------------

type outboxRow struct {
	ID                 int64
	MerchantID         int64
	CounterpartyUserID *int64
	Amount             float64
	Source             string
	RefType            string
	RefID              int64
}

func selectPendingOutboxForUpdate(ctx context.Context, tx *sql.Tx, limit int) ([]outboxRow, error) {
	const q = `
		SELECT id, merchant_id, counterparty_user_id, amount, source, ref_type, ref_id
		FROM merchant_earnings_outbox
		WHERE processed = FALSE
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]outboxRow, 0, limit)
	for rows.Next() {
		var (
			r            outboxRow
			counterparty sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &r.MerchantID, &counterparty, &r.Amount, &r.Source, &r.RefType, &r.RefID); err != nil {
			return nil, err
		}
		if counterparty.Valid {
			v := counterparty.Int64
			r.CounterpartyUserID = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// outboxGroup 聚合组（同 merchant + 同 source 的多条 outbox 合并写一条 ledger）。
type outboxGroup struct {
	MerchantID int64
	Source     string
	Sum        float64
	Rows       []outboxRow
}

// splitOutboxBySource 拆分聚合组 vs 逐笔组（RFC §5.2.3 v1.10 P1-A）。
func splitOutboxBySource(rows []outboxRow) (aggregated []outboxGroup, perRow []outboxRow) {
	aggMap := make(map[string]*outboxGroup, 8)
	for _, r := range rows {
		if r.Source == MerchantSourceSelfRecharge {
			perRow = append(perRow, r)
			continue
		}
		key := fmt.Sprintf("%d:%s", r.MerchantID, r.Source)
		g, ok := aggMap[key]
		if !ok {
			g = &outboxGroup{MerchantID: r.MerchantID, Source: r.Source}
			aggMap[key] = g
		}
		g.Sum += r.Amount
		g.Rows = append(g.Rows, r)
	}
	keys := make([]string, 0, len(aggMap))
	for k := range aggMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		aggregated = append(aggregated, *aggMap[k])
	}
	return
}

// processAggregatedGroup 处理聚合组：占位 ledger → 加余额 → 回填 balance_after。
// UNIQUE 冲突回滚整个事务（RFC §5.2.3 Finding 5）。
func processAggregatedGroup(ctx context.Context, tx *sql.Tx, g outboxGroup) error {
	ownerID, err := selectOwnerForUpdate(ctx, tx, g.MerchantID)
	if err != nil {
		return err
	}
	key := deterministicOutboxBatchKey(g)

	// 占位 ledger（UNIQUE 命中 → 整体回滚）
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO merchant_ledger
			(merchant_id, owner_user_id, counterparty_user_id, direction, amount, balance_after,
			 is_aggregated, aggregated_count, source, ref_type, ref_id, idempotency_key, note)
		VALUES ($1, $2, NULL, 'credit', $3, NULL, TRUE, $4, $5, $6, NULL, $7, $8)
	`, g.MerchantID, ownerID, g.Sum, len(g.Rows), g.Source, MerchantRefTypeOutboxBatch, key,
		fmt.Sprintf("aggregated %d outbox rows", len(g.Rows))); err != nil {
		return fmt.Errorf("insert aggregated ledger: %w", err)
	}

	// 加余额并取新值
	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`, g.Sum, ownerID).Scan(&balanceAfter); err != nil {
		return fmt.Errorf("increment owner balance: %w", err)
	}

	// 回填 balance_after
	if _, err := tx.ExecContext(ctx, `
		UPDATE merchant_ledger SET balance_after = $1
		WHERE idempotency_key = $2
	`, balanceAfter, key); err != nil {
		return fmt.Errorf("update ledger balance_after: %w", err)
	}
	return nil
}

// processSelfRechargeRow 逐笔写一条 ledger，**不**加余额（已由 redeem 加）。
// ref_type=payment_order, ref_id=payment_order.id。
func processSelfRechargeRow(ctx context.Context, tx *sql.Tx, r outboxRow) error {
	ownerID, err := selectOwnerForUpdate(ctx, tx, r.MerchantID)
	if err != nil {
		return err
	}
	currentBalance, err := readOwnerBalance(ctx, tx, ownerID)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("ledger:self_recharge:%d", r.RefID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO merchant_ledger
			(merchant_id, owner_user_id, counterparty_user_id, direction, amount, balance_after,
			 is_aggregated, source, ref_type, ref_id, idempotency_key, note)
		VALUES ($1, $2, NULL, 'credit', $3, $4, FALSE, $5, $6, $7, $8, $9)
	`, r.MerchantID, ownerID, r.Amount, currentBalance,
		MerchantSourceSelfRecharge, MerchantRefTypePaymentOrder, r.RefID, key,
		"owner self recharge (per-row)"); err != nil {
		return fmt.Errorf("insert self_recharge ledger: %w", err)
	}
	return nil
}

func selectOwnerForUpdate(ctx context.Context, tx *sql.Tx, merchantID int64) (int64, error) {
	var ownerID int64
	err := tx.QueryRowContext(ctx, `
		SELECT u.id FROM users u
		JOIN merchants m ON m.owner_user_id = u.id
		WHERE m.id = $1 AND u.deleted_at IS NULL AND m.deleted_at IS NULL
		FOR UPDATE OF u
	`, merchantID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("owner not found for merchant %d", merchantID)
	}
	if err != nil {
		return 0, err
	}
	return ownerID, nil
}

func readOwnerBalance(ctx context.Context, tx *sql.Tx, ownerID int64) (float64, error) {
	var bal float64
	err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, ownerID).Scan(&bal)
	return bal, err
}

func markOutboxProcessed(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE merchant_earnings_outbox
		SET processed = TRUE, processed_at = NOW()
		WHERE id = ANY($1::bigint[])
	`, int64ArrayParam(ids))
	return err
}

// deterministicOutboxBatchKey 生成确定性 batch idempotency_key（RFC §5.2.3 v1.3 R4）。
// 重启重试时相同的 outbox 行集合会得到相同的 key，UNIQUE 约束保证只插入一次。
func deterministicOutboxBatchKey(g outboxGroup) string {
	if len(g.Rows) == 0 {
		return fmt.Sprintf("outbox_batch:%d:%s:empty", g.MerchantID, g.Source)
	}
	minID, maxID := g.Rows[0].ID, g.Rows[0].ID
	for _, r := range g.Rows {
		if r.ID < minID {
			minID = r.ID
		}
		if r.ID > maxID {
			maxID = r.ID
		}
	}
	return fmt.Sprintf("outbox_batch:%d:%s:%d:%d:%d",
		g.MerchantID, g.Source, minID, maxID, len(g.Rows))
}

// int64ArrayParam 把 []int64 编码为 PostgreSQL bigint[]（用 lib/pq 兼容写法）。
func int64ArrayParam(ids []int64) any {
	// PostgreSQL Postgres array literal: "{1,2,3}"
	if len(ids) == 0 {
		return "{}"
	}
	b := make([]byte, 0, len(ids)*8)
	b = append(b, '{')
	for i, id := range ids {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendInt64(b, id)
	}
	b = append(b, '}')
	return string(b)
}

func appendInt64(b []byte, v int64) []byte {
	if v == 0 {
		return append(b, '0')
	}
	neg := v < 0
	if neg {
		v = -v
		b = append(b, '-')
	}
	var tmp [20]byte
	pos := len(tmp)
	for v > 0 {
		pos--
		tmp[pos] = byte('0' + v%10)
		v /= 10
	}
	return append(b, tmp[pos:]...)
}
