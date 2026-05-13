// Package schema - GroupModel: admin 给 group 配置的"展示用"模型列表。
//
// 跟计费完全无关——计费仍走 channel.supported_models / LiteLLM 路径。
// 仅用于「模型列表」展示页，让 admin 能直接在「分组管理」里给某 group
// 配一份对外宣传用的模型清单，不必绕道渠道管理。
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GroupModel admin 配的"展示用"模型列表，每行一个 (group_id, model_name)。
type GroupModel struct {
	ent.Schema
}

func (GroupModel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "group_models"},
	}
}

func (GroupModel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (GroupModel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id").
			Comment("关联的分组 id"),
		field.String("model").
			MaxLen(200).
			NotEmpty().
			Comment("模型名（用户侧可见名）"),
	}
}

func (GroupModel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "model").Unique(),
		index.Fields("group_id"),
	}
}
