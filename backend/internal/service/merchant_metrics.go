// MERCHANT-SYSTEM v1.0
// Phase 7 监控指标采样：worker 每秒上报 outbox.pending_count / batch_size / process_latency。
//
// 复用 modelboxs 现有的 slog 通道（结构化日志），不依赖独立的 metrics 库。
// 运维侧通过抓取 slog 输出的 module=merchant.metrics 行来构建监控仪表盘。

package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

// emitMerchantMetricsSample 每轮 worker 调用一次，输出三类指标。
//
// 输出字段：
//   - merchant.outbox.pending_count: 当前未处理 outbox 行数（积压告警阈值 >1000）
//   - merchant.outbox.batch_size: 本轮处理条数
//   - merchant.outbox.process_latency_ms: 本轮事务耗时 ms（P99 告警 >5s）
func emitMerchantMetricsSample(ctx context.Context, db *sql.DB, batchSize int, latency time.Duration) {
	pendingCount := -1
	if db != nil {
		var n int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM merchant_earnings_outbox WHERE processed = FALSE`).Scan(&n)
		if err == nil {
			pendingCount = n
		} else if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("merchant.metrics: pending_count query failed", "error", err)
		}
	}
	slog.Info("merchant.metrics",
		"module", "merchant.metrics",
		"merchant.outbox.pending_count", pendingCount,
		"merchant.outbox.batch_size", batchSize,
		"merchant.outbox.process_latency_ms", latency.Milliseconds(),
	)
	// 积压告警（运维通过 log 阈值采集器触发）
	if pendingCount > 1000 {
		slog.Warn("merchant.metrics.outbox_backlog_high",
			"pending_count", pendingCount,
			"alert_threshold", 1000,
			"hint", "check worker health / DB connectivity")
	}
}
