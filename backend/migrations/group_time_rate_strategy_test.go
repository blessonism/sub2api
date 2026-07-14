package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTimeRateStrategyMigrationConvertsAndRemovesPeakRate(t *testing.T) {
	content, err := FS.ReadFile("192_group_time_rate_strategy.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS time_rate_priority")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS time_rate_periods JSONB")
	require.Contains(t, sql, "SET time_rate_priority = 'proportional'")
	require.Contains(t, sql, "'rate_multiplier', rate_multiplier * peak_rate_multiplier")
	require.Contains(t, sql, "'visible_rate_multiplier', COALESCE(visible_rate_multiplier, rate_multiplier) * peak_rate_multiplier")
	require.Contains(t, sql, "'start_time', to_char(peak_start::time, 'HH24:MI')")
	require.Contains(t, sql, "'end_time', to_char(peak_end::time, 'HH24:MI')")
	require.Contains(t, sql, "DROP COLUMN IF EXISTS peak_rate_enabled")
	require.Contains(t, sql, "groups_time_rate_priority_check")

	backfill := strings.Index(sql, "UPDATE groups")
	dropLegacy := strings.Index(sql, "DROP COLUMN IF EXISTS peak_rate_enabled")
	require.NotEqual(t, -1, backfill)
	require.NotEqual(t, -1, dropLegacy)
	require.Less(t, backfill, dropLegacy, "旧高峰数据必须先转换，再删除旧字段")
}
