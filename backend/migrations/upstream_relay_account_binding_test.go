package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamRelayAccountBindingMigrationHandlesDuplicateActiveCandidates(t *testing.T) {
	content, err := FS.ReadFile("172_upstream_relay_account_binding.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "ROW_NUMBER() OVER")
	require.Contains(t, sql, "PARTITION BY connector_id, account_id, upstream_group_id")
	require.Contains(t, sql, "ORDER BY updated_at DESC, id DESC")
	require.Contains(t, sql, "WHERE c.id = r.id")
	require.Contains(t, sql, "AND r.rn > 1")
	require.Contains(t, sql, "deleted_at = NOW()")
	require.Contains(t, sql, "enabled = FALSE")
	require.Contains(t, sql, "uq_upstream_relay_candidate_connector_account_upstream_group_active")
	require.Contains(t, sql, "ON upstream_relay_candidates (connector_id, account_id, upstream_group_id)")
	require.Contains(t, sql, "DROP COLUMN IF EXISTS target_group_id")

	softDeleteIndex := strings.Index(sql, "UPDATE upstream_relay_candidates c")
	uniqueIndex := strings.Index(sql, "CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_relay_candidate_connector_account_upstream_group_active")
	require.NotEqual(t, -1, softDeleteIndex)
	require.NotEqual(t, -1, uniqueIndex)
	require.Less(t, softDeleteIndex, uniqueIndex, "重复活跃候选必须在创建账号维度唯一索引前软删除")
}
