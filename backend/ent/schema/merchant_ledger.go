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

// MerchantLedger 商户 owner 钱包资金流水（RFC §4.2.2）。
//
// 关键语义：
//   - owner-only：sub_user 余额变动**不**在此记账，仅通过 counterparty_user_id 引用
//   - 对账等式：owner.balance = baseline + Σ(credit) - Σ(debit) WHERE owner_user_id=X
//   - 同步路径行 balance_after 有值；异步聚合行 NULL（is_aggregated=true）
//   - 永久保留（审计权威源）；硬删除（无 deleted_at）
type MerchantLedger struct {
	ent.Schema
}

func (MerchantLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_ledger"},
	}
}

func (MerchantLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Int64("owner_user_id").Comment("余额变动主体（恒为 merchant.owner_user_id）"),
		field.Int64("counterparty_user_id").
			Optional().
			Nillable().
			Comment("交易对手（如 sub_user.id 或 NULL）"),
		field.String("direction").
			MaxLen(10).
			Comment("credit / debit"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("正数"),
		field.Float("balance_after").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("写入后 owner.balance 快照（聚合路径 NULL）"),
		field.Bool("is_aggregated").Default(false),
		field.Int("aggregated_count").
			Optional().
			Nillable(),
		field.String("source").MaxLen(40),
		field.String("ref_type").
			MaxLen(40).
			Optional().
			Nillable().
			Comment("payment_order / usage_billing_dedup / redeem_code / outbox_batch"),
		field.Int64("ref_id").
			Optional().
			Nillable(),
		field.String("idempotency_key").
			MaxLen(120).
			Optional().
			Nillable().
			Unique(),
		field.String("note").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MerchantLedger) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("ledger_entries").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("merchant_id", "created_at"),
		index.Fields("owner_user_id", "created_at"),
		index.Fields("counterparty_user_id", "created_at"),
		index.Fields("source", "ref_type", "ref_id"),
	}
}
