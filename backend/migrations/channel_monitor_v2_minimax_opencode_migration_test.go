package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2MiniMaxOpenCodePlatformsMigration(t *testing.T) {
	content, err := FS.ReadFile("239_channel_monitor_v2_add_minimax_opencode.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "UPDATE channel_monitor_v2_config")
	require.Contains(t, sql, "jsonb_array_elements(platforms)")
	for _, platform := range []string{"minimax", "opencode_go"} {
		require.Contains(t, sql, `item->>'platform' = '`+platform+`'`)
		require.Contains(t, sql, `{"platform":"`+platform+`","enabled":true,"models":[]}`)
	}
	require.Contains(t, sql, "version = version + 1")
}
