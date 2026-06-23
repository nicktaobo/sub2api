// MERCHANT-SYSTEM v1.0
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MerchantEarningsOutbox 商户分润短期缓冲队列（RFC §4.1.3）。
//
// 这张表**不是流水表**——它是网关→worker 的临时队列。
// processed=true 后保留 30 天用于排查（后台 cron 归档/删除）。
type MerchantEarningsOutbox struct {
	ent.Schema
}

func (MerchantEarningsOutbox) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_earnings_outbox"},
	}
}

func (MerchantEarningsOutbox) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Int64("counterparty_user_id").
			Optional().
			Nillable().
			Comment("触发该笔分润的 sub_user"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("source").
			MaxLen(40).
			Comment("user_markup_share / self_recharge"),
		field.String("ref_type").
			MaxLen(40).
			Comment("usage_billing_dedup / payment_order"),
		field.Int64("ref_id"),
		field.String("idempotency_key").
			MaxLen(100).
			Unique(),
		field.Bool("processed").Default(false),
		field.Time("processed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MerchantEarningsOutbox) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("outbox_entries").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantEarningsOutbox) Indexes() []ent.Index {
	return []ent.Index{
		// idx_outbox_pending 通过 SQL migration 加 partial index (WHERE processed=false)
		index.Fields("merchant_id", "created_at"),
	}
}
