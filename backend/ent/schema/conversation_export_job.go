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

// ConversationExportJob 保存后台对话导出任务与导出产物元数据。
type ConversationExportJob struct {
	ent.Schema
}

func (ConversationExportJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "conversation_export_jobs"},
	}
}

func (ConversationExportJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("status").MaxLen(32).Default("pending"),
		field.JSON("filters", map[string]json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("format").MaxLen(32).Default("messages_jsonl"),
		field.String("encoding").MaxLen(32).Default("zstd"),
		field.Int64("session_count").Default(0),
		field.Int64("turn_count").Default(0),
		field.Int64("file_size").Default(0),
		field.String("s3_key").MaxLen(512).Optional().Nillable(),
		field.Time("download_url_expires_at").Optional().Nillable(),
		field.Time("expires_at"),
		field.String("error_message").MaxLen(500).Optional().Nillable(),
		field.Int64("created_by"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
	}
}

func (ConversationExportJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("created_by"),
		index.Fields("created_at"),
		index.Fields("expires_at"),
	}
}
