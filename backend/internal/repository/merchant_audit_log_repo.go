// MERCHANT-SYSTEM v1.0
package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchantauditlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantAuditLogRepository struct {
	client *dbent.Client
}

func NewMerchantAuditLogRepository(client *dbent.Client) service.MerchantAuditLogRepository {
	return &merchantAuditLogRepository{client: client}
}

func (r *merchantAuditLogRepository) Insert(ctx context.Context, e *service.MerchantAuditLogEntry) error {
	client := clientFromContext(ctx, r.client)
	c := client.MerchantAuditLog.Create().
		SetMerchantID(e.MerchantID).
		SetField(e.Field).
		SetReason(e.Reason).
		SetNillableAdminID(e.AdminID).
		SetNillableOldValue(e.OldValue).
		SetNillableNewValue(e.NewValue)
	created, err := c.Save(ctx)
	if err != nil {
		return err
	}
	e.ID = created.ID
	e.CreatedAt = created.CreatedAt
	return nil
}

func (r *merchantAuditLogRepository) ListByMerchant(ctx context.Context, merchantID int64, offset, limit int) ([]*service.MerchantAuditLogEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	client := clientFromContext(ctx, r.client)
	q := client.MerchantAuditLog.Query().Where(merchantauditlog.MerchantIDEQ(merchantID))
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Order(dbent.Desc(merchantauditlog.FieldCreatedAt)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*service.MerchantAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		entry := &service.MerchantAuditLogEntry{
			ID:         row.ID,
			MerchantID: row.MerchantID,
			Field:      row.Field,
			Reason:     row.Reason,
			CreatedAt:  row.CreatedAt,
		}
		if row.AdminID != nil {
			v := *row.AdminID
			entry.AdminID = &v
		}
		if row.OldValue != nil {
			v := *row.OldValue
			entry.OldValue = &v
		}
		if row.NewValue != nil {
			v := *row.NewValue
			entry.NewValue = &v
		}
		out = append(out, entry)
	}
	return out, total, nil
}
