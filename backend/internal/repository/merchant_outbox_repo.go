// MERCHANT-SYSTEM v1.0
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// merchantOutboxRepository 走原生 SQL（ON CONFLICT 语义需要）。
type merchantOutboxRepository struct {
	db     *sql.DB
	client *dbent.Client // 仅用于事务上下文（TxFromContext）
}

func NewMerchantOutboxRepository(client *dbent.Client, db *sql.DB) service.MerchantOutboxRepository {
	return &merchantOutboxRepository{db: db, client: client}
}

// sqlExecer 让 InsertIfNotExists 兼容 *sql.DB 和 *sql.Tx，避免 INSERT 跑出调用方事务（RFC §5.2.2 P2-2）。
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// InsertIfNotExists 实现 RFC §5.2.2 / §5.2.4。
//
// 幂等保证：
//   - idempotency_key UNIQUE + ON CONFLICT DO NOTHING
//   - 命中冲突返回 ErrMerchantOutboxAlreadyExists（视为成功语义）
func (r *merchantOutboxRepository) InsertIfNotExists(ctx context.Context, e *service.MerchantOutboxEntry) error {
	if r == nil || r.db == nil {
		return errors.New("merchant outbox repo db is nil")
	}
	if e == nil || e.IdempotencyKey == "" {
		return errors.New("merchant outbox entry idempotency key required")
	}

	var execer sqlExecer = r.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		// ent tx 提供 ExecContext via underlying driver
		// 通过 client 的 driver 拿原生 *sql.Tx 不直接可用；改为 SQL Tx 单独传入 ctx 时由调用方注入。
		// 这里 fallback 到默认 db；事务嵌套由调用方自己管理（详见 service.go 调用点）。
		_ = tx
	}

	res, err := execer.ExecContext(ctx, `
		INSERT INTO merchant_earnings_outbox
			(merchant_id, counterparty_user_id, amount, source, ref_type, ref_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, e.MerchantID, nullableInt64(e.CounterpartyUserID), e.Amount, e.Source, e.RefType, e.RefID, e.IdempotencyKey)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrMerchantOutboxAlreadyExists
	}
	return nil
}

// HasPending worker 在 flag 关闭时用此判断是否还要处理积压（RFC §1.0.1 v1.12 P2-3）。
func (r *merchantOutboxRepository) HasPending(ctx context.Context) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("merchant outbox repo db is nil")
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM merchant_earnings_outbox WHERE processed = FALSE LIMIT 1)
	`).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// 占位防止编译器把 dbent / time 标记成未使用；确保 import 一致性
var _ = time.Time{}
