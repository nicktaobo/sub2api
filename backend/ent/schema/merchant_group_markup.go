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

// MerchantGroupMarkup 商户分组级 markup 覆盖（RFC §4.1.5）。
//
// 一个 (merchant_id, group_id) 最多一行。markup ≥ 1（DB CHECK）。
// 商户/分组任一删除时级联删除（CASCADE）。
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
		field.Float("markup").
			SchemaType(map[string]string{dialect.Postgres: "decimal(6,4)"}).
			Comment("该分组特定加价倍率 ≥ 1"),
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
