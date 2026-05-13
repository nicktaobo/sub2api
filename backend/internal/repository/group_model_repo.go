// Package repository - GroupModel repository.
//
// 「模型列表」展示页用的 admin-managed 配置，跟计费无关。
package repository

import (
	"context"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/groupmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type groupModelRepository struct {
	client *dbent.Client
}

func NewGroupModelRepository(client *dbent.Client) service.GroupModelRepository {
	return &groupModelRepository{client: client}
}

func (r *groupModelRepository) ListByGroup(ctx context.Context, groupID int64) ([]string, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.GroupModel.Query().
		Where(groupmodel.GroupIDEQ(groupID)).
		Order(dbent.Asc(groupmodel.FieldModel)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Model)
	}
	return out, nil
}

// SetForGroup 全量替换：先删该 group 所有模型，再批量插入新列表（去重 + 去空）。
// 在一个事务内完成。
func (r *groupModelRepository) SetForGroup(ctx context.Context, groupID int64, models []string) error {
	client := clientFromContext(ctx, r.client)
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	txClient := tx.Client()

	// 1. 清空旧数据
	if _, err := txClient.GroupModel.Delete().
		Where(groupmodel.GroupIDEQ(groupID)).Exec(txCtx); err != nil {
		return err
	}

	// 2. 去重 + 清洗
	seen := make(map[string]struct{}, len(models))
	clean := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		clean = append(clean, m)
	}

	// 3. 批量插入
	if len(clean) > 0 {
		builders := make([]*dbent.GroupModelCreate, 0, len(clean))
		for _, m := range clean {
			builders = append(builders, txClient.GroupModel.Create().SetGroupID(groupID).SetModel(m))
		}
		if _, err := txClient.GroupModel.CreateBulk(builders...).Save(txCtx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ListAll 返回所有 (group_id → models) 映射，用于「模型列表」展示页一次拉全。
func (r *groupModelRepository) ListAll(ctx context.Context) (map[int64][]string, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.GroupModel.Query().
		Order(dbent.Asc(groupmodel.FieldGroupID), dbent.Asc(groupmodel.FieldModel)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][]string, 16)
	for _, r := range rows {
		out[r.GroupID] = append(out[r.GroupID], r.Model)
	}
	return out, nil
}
