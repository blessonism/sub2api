package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AnnouncementEmailBroadcast 保存公告邮件群发快照与汇总进度。
type AnnouncementEmailBroadcast struct{ ent.Schema }

func (AnnouncementEmailBroadcast) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "announcement_email_broadcasts"}}
}

func (AnnouncementEmailBroadcast) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("announcement_id").Unique(),
		field.String("subject").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("body_html").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").MaxLen(32).Default("pending"),
		field.Int("total_count").Default(0),
		field.Int("sent_count").Default(0),
		field.Int("failed_count").Default(0),
		field.Int64("created_by").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("started_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AnnouncementEmailBroadcast) Indexes() []ent.Index {
	return []ent.Index{index.Fields("status")}
}
