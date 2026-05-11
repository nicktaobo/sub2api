// MERCHANT-SYSTEM v2.0
// 商户分组拿货价仓储（admin 配置；与 group_markup 配对：sell - cost = 商户利润）。
package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchantgroupcost"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantGroupCostRepository struct {
	client *dbent.Client
}

func NewMerchantGroupCostRepository(client *dbent.Client) service.MerchantGroupCostRepository {
	return &merchantGroupCostRepository{client: client}
}

func (r *merchantGroupCostRepository) Upsert(ctx context.Context, e *service.MerchantGroupCost) error {
	client := clientFromContext(ctx, r.client)
	id, err := client.MerchantGroupCost.Create().
		SetMerchantID(e.MerchantID).
		SetGroupID(e.GroupID).
		SetCostRate(e.CostRate).
		OnConflictColumns(merchantgroupcost.FieldMerchantID, merchantgroupcost.FieldGroupID).
		UpdateCostRate().
		ID(ctx)
	if err != nil {
		return err
	}
	e.ID = id
	return nil
}

func (r *merchantGroupCostRepository) Delete(ctx context.Context, merchantID, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.MerchantGroupCost.Delete().
		Where(
			merchantgroupcost.MerchantIDEQ(merchantID),
			merchantgroupcost.GroupIDEQ(groupID),
		).Exec(ctx)
	return err
}

func (r *merchantGroupCostRepository) ListByMerchant(ctx context.Context, merchantID int64) ([]*service.MerchantGroupCost, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.MerchantGroupCost.Query().
		Where(merchantgroupcost.MerchantIDEQ(merchantID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.MerchantGroupCost, 0, len(rows))
	for _, row := range rows {
		out = append(out, &service.MerchantGroupCost{
			ID:         row.ID,
			MerchantID: row.MerchantID,
			GroupID:    row.GroupID,
			CostRate:   row.CostRate,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return out, nil
}
