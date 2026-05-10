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

// MerchantAuditLog 商户配置/操作审计日志（RFC §4.1.4），永久保留。
type MerchantAuditLog struct {
	ent.Schema
}

func (MerchantAuditLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_audit_log"},
	}
}

func (MerchantAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.Int64("admin_id").
			Optional().
			Nillable().
			Comment("操作的 admin（NULL=系统）"),
		field.String("field").
			MaxLen(50).
			Comment("discount / user_markup / status / domain_*"),
		field.String("old_value").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),
		field.String("new_value").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),
		field.String("reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MerchantAuditLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("audit_logs").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("merchant_id", "created_at"),
	}
}
