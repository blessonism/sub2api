package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ConversationSession 保存可查询的对话会话索引。
type ConversationSession struct {
	ent.Schema
}

func (ConversationSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "conversation_sessions"},
	}
}

func (ConversationSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("session_id").MaxLen(128).NotEmpty().Unique(),
		field.String("trajectory_id").MaxLen(128).Optional().Nillable(),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("account_id").Optional().Nillable(),
		field.String("provider").MaxLen(32).Default("openai"),
		field.String("model").MaxLen(200).NotEmpty(),
		field.String("request_path").MaxLen(200).NotEmpty(),
		field.String("status").MaxLen(32).Default("active"),
		field.Int("turn_count").Default(0),
		field.Int("source_request_count").Default(0),
		field.Int64("input_tokens").Default(0),
		field.Int64("output_tokens").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Float("actual_cost").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.String("quality_status").MaxLen(32).Default("unchecked"),
		field.JSON("quality_errors", []json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Bool("exportable").Default(false),
		field.String("capture_status").MaxLen(32).Default("captured"),
		field.String("session_source").MaxLen(32).Default("single_turn"),
		field.Time("retention_until"),
		field.Time("started_at"),
		field.Time("ended_at"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ConversationSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("user_id"),
		index.Fields("api_key_id"),
		index.Fields("model"),
		index.Fields("quality_status"),
		index.Fields("exportable"),
		index.Fields("started_at"),
		index.Fields("created_at"),
		index.Fields("user_id", "started_at"),
		index.Fields("api_key_id", "started_at"),
	}
}
