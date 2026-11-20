package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorDisplayGroupsMigrationOrder(t *testing.T) {
	content, err := FS.ReadFile("252_channel_monitor_display_groups.sql")
	require.NoError(t, err)
	raw := string(content)

	down := strings.Index(raw, "DELETE FROM schema_migrations")
	addColumn := strings.Index(raw, "ADD COLUMN")
	require.GreaterOrEqual(t, down, 0)
	require.Greater(t, addColumn, down)

	sql := executableSQL(raw)
	order := []string{
		"ADD COLUMN",
		"CREATE TABLE",
		"SET group_name = TRIM(group_name)",
		"INSERT INTO channel_monitor_display_groups",
		"idx_channel_monitor_display_groups_sort",
	}
	last := -1
	for _, piece := range order {
		at := strings.Index(sql, piece)
		require.Greater(t, at, last, piece)
		last = at
	}

	require.NotContains(t, sql, "DROP")
	require.NotContains(t, sql, "DELETE")
	require.NotContains(t, sql, "SET group_name = ''")
	require.Contains(t, sql, "SET group_name = TRIM(group_name)")
}
