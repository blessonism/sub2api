package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorGatewayGroupMigration(t *testing.T) {
	content, err := FS.ReadFile("234_channel_monitor_gateway_group.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS error_category VARCHAR(40) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS target_kind VARCHAR(20) NOT NULL DEFAULT 'endpoint'")
	require.Contains(t, sql, "channel_monitors_target_kind_check")
	require.Contains(t, sql, "CHECK (target_kind IN ('endpoint', 'gateway_group'))")
}
