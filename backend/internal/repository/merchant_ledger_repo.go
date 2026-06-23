// MERCHANT-SYSTEM v1.0
package repository

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantLedgerRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewMerchantLedgerRepository(client *dbent.Client, db *sql.DB) service.MerchantLedgerRepository {
	return &merchantLedgerRepository{client: client, db: db}
}

// Insert 同步路径写一条 ledger（service 事务内调用）。
// 事务上下文从 ctx 取（dbent.TxFromContext）。
func (r *merchantLedgerRepository) Insert(ctx context.Context, e *service.MerchantLedgerEntry) error {
	if e == nil {
		return errors.New("merchant ledger entry is nil")
	}
	client := clientFromContext(ctx, r.client)
	c := client.MerchantLedger.Create().
		SetMerchantID(e.MerchantID).
		SetOwnerUserID(e.OwnerUserID).
		SetDirection(e.Direction).
		SetAmount(e.Amount).
		SetIsAggregated(e.IsAggregated).
		SetSource(e.Source).
		SetNillableCounterpartyUserID(e.CounterpartyUserID).
		SetNillableBalanceAfter(e.BalanceAfter).
		SetNillableAggregatedCount(e.AggregatedCount).
		SetNillableRefType(e.RefType).
		SetNillableRefID(e.RefID).
		SetNillableIdempotencyKey(e.IdempotencyKey).
		SetNillableNote(e.Note)

	created, err := c.Save(ctx)
	if err != nil {
		return err
	}
	e.ID = created.ID
	e.CreatedAt = created.CreatedAt
	return nil
}

func (r *merchantLedgerRepository) List(ctx context.Context, merchantID int64, offset, limit int) ([]*service.MerchantLedgerEntry, int, error) {
	if r.db == nil {
		return nil, 0, errors.New("merchant ledger repo db is nil")
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM merchant_ledger WHERE merchant_id = $1`, merchantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, merchant_id, owner_user_id, counterparty_user_id, direction, amount,
		       balance_after, is_aggregated, aggregated_count, source, ref_type, ref_id,
		       idempotency_key, note, created_at
		FROM merchant_ledger
		WHERE merchant_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, q, merchantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*service.MerchantLedgerEntry, 0, limit)
	for rows.Next() {
		var (
			e            service.MerchantLedgerEntry
			counterparty sql.NullInt64
			balanceAfter sql.NullFloat64
			aggCount     sql.NullInt64
			refType      sql.NullString
			refID        sql.NullInt64
			idemKey      sql.NullString
			note         sql.NullString
		)
		if err := rows.Scan(
			&e.ID, &e.MerchantID, &e.OwnerUserID, &counterparty, &e.Direction, &e.Amount,
			&balanceAfter, &e.IsAggregated, &aggCount, &e.Source, &refType, &refID,
			&idemKey, &note, &e.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if counterparty.Valid {
			v := counterparty.Int64
			e.CounterpartyUserID = &v
		}
		if balanceAfter.Valid {
			v := balanceAfter.Float64
			e.BalanceAfter = &v
		}
		if aggCount.Valid {
			v := int(aggCount.Int64)
			e.AggregatedCount = &v
		}
		if refType.Valid {
			v := refType.String
			e.RefType = &v
		}
		if refID.Valid {
			v := refID.Int64
			e.RefID = &v
		}
		if idemKey.Valid {
			v := idemKey.String
			e.IdempotencyKey = &v
		}
		if note.Valid {
			v := note.String
			e.Note = &v
		}
		out = append(out, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// 占位防止 dbent unused
var _ = dbent.MerchantLedger{}
