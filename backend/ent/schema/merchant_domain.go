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

// MerchantDomain 商户域名 + 站点品牌定制（RFC §4.1.2）。
//
// domain 的 UNIQUE 通过 partial unique index 实现（WHERE deleted_at IS NULL），
// 软删除后域名可重新启用，详见 SQL migration。
type MerchantDomain struct {
	ent.Schema
}

func (MerchantDomain) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "merchant_domains"},
	}
}

func (MerchantDomain) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (MerchantDomain) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("merchant_id"),
		field.String("domain").MaxLen(255).NotEmpty(),
		field.String("site_name").MaxLen(100).Default(""),
		field.String("site_logo").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default("").
			Comment("URL 或 base64"),
		field.String("brand_color").MaxLen(20).Default(""),
		field.String("custom_css").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("home_content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default("").
			Comment("自定义 HTML（XSS sanitized）"),
		field.String("seo_title").MaxLen(255).Default(""),
		field.String("seo_description").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("seo_keywords").MaxLen(500).Default(""),
		field.String("verify_token").MaxLen(64).Default(""),
		field.Bool("verified").Default(false),
		field.Time("verified_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MerchantDomain) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("merchant", Merchant.Type).
			Ref("domains").
			Field("merchant_id").
			Unique().
			Required(),
	}
}

func (MerchantDomain) Indexes() []ent.Index {
	return []ent.Index{
		// domain 的 partial unique index 通过 SQL migration 创建（WHERE deleted_at IS NULL）
		index.Fields("merchant_id"),
		index.Fields("verified", "domain"),
		index.Fields("deleted_at"),
	}
}
