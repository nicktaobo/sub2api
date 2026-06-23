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

// Merchant 商户主表（RFC §4.1.1）。
//
// 关键约束：
//   - owner_user_id UNIQUE：一个 user 最多是一个商户的 owner
//
// 利润模型：v3.0 起仅由消费侧 per-group sell_rate / cost_rate 倍率差产生
// （见 merchant_group_costs / merchant_group_markups）。充值环节不再有折扣或分成。
type Merchant struct {
	ent.Schema
}

func (Merchant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchants"},
	}
}

func (Merchant) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (Merchant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id").
			Unique().
			Comment("商户拥有者 user.id；UNIQUE 保证一个 user 最多是一个商户的 owner"),
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("status").
			MaxLen(20).
			Default("active").
			Comment("active / suspended"),
		field.Float("owner_balance_baseline").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("开通商户时的 owner.balance 快照，对账等式基线"),
		field.Float("low_balance_threshold").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.JSON("notify_emails", []string{}).
			Default([]string{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (Merchant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("domains", MerchantDomain.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("ledger_entries", MerchantLedger.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("outbox_entries", MerchantEarningsOutbox.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("audit_logs", MerchantAuditLog.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("group_markups", MerchantGroupMarkup.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("group_costs", MerchantGroupCost.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("withdraw_requests", MerchantWithdrawRequest.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("sub_users", User.Type),
	}
}

func (Merchant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("deleted_at"),
	}
}
