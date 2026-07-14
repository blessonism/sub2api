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

// AnnouncementEmailDelivery 保存每位收件人的投递状态。
type AnnouncementEmailDelivery struct{ ent.Schema }

func (AnnouncementEmailDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "announcement_email_deliveries"}}
}

func (AnnouncementEmailDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("broadcast_id"),
		field.Int64("user_id").Optional().Nillable(),
		field.String("email").MaxLen(320),
		field.String("status").MaxLen(32).Default("pending"),
		field.Int("attempt_count").Default(0),
		field.Time("lease_expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("error_message").MaxLen(500).Optional().Nillable(),
		field.Time("last_attempt_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("sent_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AnnouncementEmailDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("broadcast_id", "user_id").Unique(),
		index.Fields("status", "lease_expires_at", "id"),
		index.Fields("broadcast_id", "status", "id"),
		index.Fields("broadcast_id", "email"),
	}
}
