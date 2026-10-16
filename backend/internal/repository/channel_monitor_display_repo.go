package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type channelMonitorDisplayRepository struct{ db *sql.DB }

func NewChannelMonitorDisplayRepository(db *sql.DB) service.ChannelMonitorDisplayRepository {
	return &channelMonitorDisplayRepository{db: db}
}

func (r *channelMonitorDisplayRepository) BatchMetrics(ctx context.Context, keys []service.ChannelMonitorMetricKey, now time.Time) (map[service.ChannelMonitorMetricKey]service.ChannelMonitorDisplayMetrics, error) {
	out := make(map[service.ChannelMonitorMetricKey]service.ChannelMonitorDisplayMetrics, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	ids, models := make([]int64, 0, len(keys)), make([]string, 0, len(keys))
	for _, key := range keys {
		ids, models = append(ids, key.GroupID), append(models, key.Model)
	}
	// 先限定分组和时间，复用 group_id/created_at 索引；缺少样本保留 SQL NULL。
	query := `WITH targets AS (
		SELECT * FROM unnest($1::bigint[], $2::text[]) AS t(group_id, model)
	)
	SELECT t.group_id, t.model,
		SUM(ul.cache_read_tokens)::double precision /
			NULLIF(SUM(ul.input_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0),
		AVG(ul.output_tokens * 1000.0 / NULLIF(ul.duration_ms, 0))
			FILTER (WHERE ul.created_at >= $4 ` + usageOutputTPSValidSQL + `)
	FROM targets t
	LEFT JOIN usage_logs ul ON ul.group_id = t.group_id
		AND ul.created_at >= $3 AND ul.created_at < $5
		AND ` + channelMonitorV2ModelSQL + ` = t.model
	GROUP BY t.group_id, t.model`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids), pq.Array(models), now.Add(-24*time.Hour), now.Add(-5*time.Minute), now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var key service.ChannelMonitorMetricKey
		var cache, tps sql.NullFloat64
		if err := rows.Scan(&key.GroupID, &key.Model, &cache, &tps); err != nil {
			return nil, err
		}
		metric := service.ChannelMonitorDisplayMetrics{}
		if cache.Valid {
			metric.CacheRate24h = &cache.Float64
		}
		if tps.Valid {
			metric.OutputTPS5m = &tps.Float64
		}
		out[key] = metric
	}
	return out, rows.Err()
}
