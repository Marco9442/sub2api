package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorCustomMigration(t *testing.T) {
	content, err := FS.ReadFile("251_channel_monitor_custom.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(executableSQL(string(content))), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS metric_kind VARCHAR(16) NOT NULL DEFAULT 'latency'")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS tokens_per_sec DOUBLE PRECISION")
	require.Contains(t, sql, "SET metric_kind = 'quota'")
	require.Contains(t, sql, "quota IS NOT NULL")
	require.Contains(t, sql, "CHECK (metric_kind IN ('latency', 'ttft', 'quota'))")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS channel_monitor_custom_overrides")
	require.Contains(t, sql, "REFERENCES channel_monitors(id) ON DELETE CASCADE")
	require.NotContains(t, sql, "DROP COLUMN")
	require.NotContains(t, sql, "DROP TABLE")
}

// executableSQL 去掉注释，避免回滚说明里的语句被当成正向迁移。
func executableSQL(content string) string {
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
