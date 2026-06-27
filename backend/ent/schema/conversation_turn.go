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

// ConversationTurn 保存单次成功 chat/completions 请求解析出的 turn。
type ConversationTurn struct {
	ent.Schema
}

func (ConversationTurn) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "conversation_turns"},
	}
}

func (ConversationTurn) Fields() []ent.Field {
	return []ent.Field{
		field.String("session_id").MaxLen(128).NotEmpty(),
		field.String("request_id").MaxLen(128).NotEmpty(),
		field.String("upstream_request_id").MaxLen(128).Optional().Nillable(),
		field.String("client_request_id").MaxLen(128).Optional().Nillable(),
		field.Int("turn_index"),
		field.String("provider").MaxLen(32).Default("openai"),
		field.String("model").MaxLen(200).NotEmpty(),
		field.String("request_path").MaxLen(200).NotEmpty(),
		field.JSON("request_messages", []json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("response_messages", []json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("tools", []json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("usage", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("meta", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int64("input_tokens").Default(0),
		field.Int64("output_tokens").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Float("actual_cost").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Bool("stream").Default(false),
		field.Bool("client_disconnect").Default(false),
		field.Bool("truncated").Default(false),
		field.String("quality_status").MaxLen(32).Default("unchecked"),
		field.JSON("quality_errors", []json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Bool("exportable").Default(false),
		field.String("parse_status").MaxLen(32).Default("failed"),
		field.String("parse_error").MaxLen(500).Optional().Nillable(),
		field.String("dedupe_hash").MaxLen(128).NotEmpty(),
		field.String("raw_archive_key").MaxLen(512).Optional().Nillable(),
		field.Text("payload_preview").Optional().Nillable(),
		field.Bytes("payload_compressed").Optional().Nillable(),
		field.Time("retention_until"),
		field.Time("created_at").Default(time.Now),
	}
}

func (ConversationTurn) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("request_id"),
		index.Fields("client_request_id"),
		index.Fields("dedupe_hash"),
		index.Fields("created_at"),
		index.Fields("session_id", "turn_index").Unique(),
	}
}
