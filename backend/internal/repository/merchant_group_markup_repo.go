// MERCHANT-SYSTEM v2.0
// 商户分组对外售价仓储（字段从 markup 改为 sell_rate）。
package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchantgroupmarkup"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantGroupMarkupRepository struct {
	client *dbent.Client
}

func NewMerchantGroupMarkupRepository(client *dbent.Client) service.MerchantGroupMarkupRepository {
	return &merchantGroupMarkupRepository{client: client}
}

// Upsert 插入或更新 (merchant_id, group_id) 的 sell_rate（UNIQUE 约束）。
func (r *merchantGroupMarkupRepository) Upsert(ctx context.Context, e *service.MerchantGroupMarkup) error {
	client := clientFromContext(ctx, r.client)
	id, err := client.MerchantGroupMarkup.Create().
		SetMerchantID(e.MerchantID).
		SetGroupID(e.GroupID).
		SetSellRate(e.SellRate).
		OnConflictColumns(merchantgroupmarkup.FieldMerchantID, merchantgroupmarkup.FieldGroupID).
		UpdateSellRate().
		ID(ctx)
	if err != nil {
		return err
	}
	e.ID = id
	return nil
}

func (r *merchantGroupMarkupRepository) Delete(ctx context.Context, merchantID, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.MerchantGroupMarkup.Delete().
		Where(
			merchantgroupmarkup.MerchantIDEQ(merchantID),
			merchantgroupmarkup.GroupIDEQ(groupID),
		).Exec(ctx)
	return err
}

func (r *merchantGroupMarkupRepository) ListByMerchant(ctx context.Context, merchantID int64) ([]*service.MerchantGroupMarkup, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.MerchantGroupMarkup.Query().
		Where(merchantgroupmarkup.MerchantIDEQ(merchantID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.MerchantGroupMarkup, 0, len(rows))
	for _, row := range rows {
		out = append(out, &service.MerchantGroupMarkup{
			ID:         row.ID,
			MerchantID: row.MerchantID,
			GroupID:    row.GroupID,
			SellRate:   row.SellRate,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return out, nil
}
