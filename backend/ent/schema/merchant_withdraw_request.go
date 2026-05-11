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

// MerchantWithdrawRequest 商户提现申请。
//
//   - status: pending（待审核）/ approved（已批准但未打款）/ paid（已打款，已扣 owner.balance）/ rejected（已拒绝）
//   - 批准时写一条 merchant_ledger debit（source=withdraw），扣 owner.balance
//   - pending 状态金额计入"审核中"，paid 计入"已提现"
type MerchantWithdrawRequest struct {
	ent.Schema
}

func (MerchantWithdrawRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_withdraw_requests"},
	}
}

func (MerchantWithdrawRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("status").
			MaxLen(20).
			Default("pending").
			Comment("pending / approved / paid / rejected"),
		field.String("payment_method").
			MaxLen(20).
			Comment("alipay / wechat / bank / usdt / other"),
		field.String("payment_account").MaxLen(255),
		field.String("payment_name").MaxLen(100),
		field.String("note").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Int64("admin_id").
			Optional().
			Nillable(),
		field.String("reject_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Int64("ledger_id").
			Optional().
			Nillable().
			Comment("批准后写入 merchant_ledger 的 id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("processed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MerchantWithdrawRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("withdraw_requests").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantWithdrawRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("merchant_id", "created_at"),
		index.Fields("status", "created_at"),
	}
}
