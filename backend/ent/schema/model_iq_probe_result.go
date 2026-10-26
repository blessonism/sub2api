package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelIQProbeResult holds the schema definition for the ModelIQProbeResult entity.
// 智商探针结果：每轮探测对每个（账号, 模型）目标写一行。糖果问题等固定题目发给
// GPT 模型，按"出现即通过"判分（与渠道监控 challenge 判分语义一致），用于发现
// 模型降智。明细保留 30 天，由 OpsCleanupService 每日物理删（日志类表不用软删）。
type ModelIQProbeResult struct {
	ent.Schema
}

func (ModelIQProbeResult) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_iq_probe_results"},
	}
}

func (ModelIQProbeResult) Fields() []ent.Field {
	return []ent.Field{
		// account_id 为空表示 model_only 模式下未定位到具体账号（保留扩展）。
		field.Int64("account_id").
			Optional().
			Nillable(),
		// account_name 存账号名快照；账号删除后结果行仍可读。仅管理端可见。
		field.String("account_name").
			Optional().
			Default("").
			MaxLen(200),
		field.String("model").
			NotEmpty().
			MaxLen(200),
		// protocol 记录探测时推断的协议（chat_completions / responses），
		// 由账号类型推断，仅供排查参考。
		field.String("protocol").
			Optional().
			Default("").
			MaxLen(40),
		field.Enum("status").
			Values("pass", "wrong_answer", "error"),
		// override_status 非空表示管理员人工改判，作为展示与统计的有效状态。
		field.Enum("override_status").
			Values("pass", "wrong_answer", "error").
			Optional().
			Nillable(),
		field.String("error_category").
			Optional().
			Default("").
			MaxLen(40),
		field.String("error_message").
			Optional().
			Default(""),
		field.String("expected_answer").
			NotEmpty().
			MaxLen(200),
		// actual_answer 存模型回答原文（截断 4KB），供管理端复核判定是否误判。
		field.Text("actual_answer").
			Optional(),
		field.Int64("latency_ms").
			Optional().
			Nillable(),
		// token 三列：当前执行通道（账号连接测试）不返回 usage，恒为 NULL；
		// 保留列以对齐 channel_monitor_histories 约定，未来切换执行通道时启用。
		field.Int("input_tokens").
			Optional().
			Nillable(),
		field.Int("output_tokens").
			Optional().
			Nillable(),
		field.Int("cache_read_tokens").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now),
	}
}

func (ModelIQProbeResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model", "created_at"),
		index.Fields("account_id", "created_at"),
	}
}
