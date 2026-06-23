// MERCHANT-SYSTEM v1.0
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

// MerchantGroupMarkup 商户分组级对外售价（v2.0 起改成绝对倍率语义）。
//
// 字段 sell_rate：商户对自家 sub_user 报价 = base × sell_rate。商户自助设置。
// 必须 ≥ 对应 MerchantGroupCost.cost_rate（service 层校验），否则商户在亏本卖。
//
// 一个 (merchant_id, group_id) 最多一行。商户/分组任一删除时级联删除（CASCADE）。
//
// 历史命名遗留：表名 / 实体名仍叫 group_markup（v1 时这里存 markup 加价倍率），
// v2.0 重构为 sell_rate 绝对倍率，未来可考虑重命名为 group_sell。
type MerchantGroupMarkup struct {
	ent.Schema
}

func (MerchantGroupMarkup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_group_markups"},
	}
}

func (MerchantGroupMarkup) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (MerchantGroupMarkup) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Int64("group_id"),
		field.Float("sell_rate").
			SchemaType(map[string]string{dialect.Postgres: "decimal(6,4)"}).
			Comment("商户在该分组对外的售价倍率（绝对值），sub_user 付 = base × sell_rate；必须 ≥ 对应 cost_rate"),
	}
}

func (MerchantGroupMarkup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("group_markups").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantGroupMarkup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("merchant_id", "group_id").Unique(),
	}
}
