// MERCHANT-SYSTEM v2.0
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MerchantGroupCost 商户在某分组上的拿货价倍率（admin 配置）。
//
// 语义：商户的 sub_user 在该 group 消费时，平台从中扣 = base × cost_rate
// （base = 上游 API 原始 token 成本）。未配置时回退到 group.rate_multiplier，
// 即商户跟主站普通用户同价拿货，不享受任何"分销折扣"。
//
// 与 MerchantGroupMarkup 配对：sell_rate ≥ cost_rate，差值即商户毛利。
//
// 一个 (merchant_id, group_id) 最多一行。cost_rate > 0（DB CHECK）。
// 商户/分组任一删除时级联删除（CASCADE）。
type MerchantGroupCost struct {
	ent.Schema
}

func (MerchantGroupCost) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_group_costs"},
	}
}

func (MerchantGroupCost) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (MerchantGroupCost) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Int64("group_id"),
		field.Float("cost_rate").
			SchemaType(map[string]string{dialect.Postgres: "decimal(6,4)"}).
			Comment("商户在该分组上的拿货价倍率 > 0；终端用户付的钱里平台扣 base × cost_rate"),
	}
}

func (MerchantGroupCost) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("group_costs").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantGroupCost) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("merchant_id", "group_id").Unique(),
	}
}
