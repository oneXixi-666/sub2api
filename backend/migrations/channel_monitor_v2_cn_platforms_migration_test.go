package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2CNPlatformsMigration(t *testing.T) {
	content, err := FS.ReadFile("232_channel_monitor_v2_add_cn_platforms.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "UPDATE channel_monitor_v2_config")
	require.Contains(t, sql, "jsonb_array_elements(platforms)")
	for _, platform := range []string{"kimi", "zhipu", "deepseek"} {
		require.Contains(t, sql, `item->>'platform' = '`+platform+`'`)
		require.Contains(t, sql, `{"platform":"`+platform+`","enabled":true,"models":[]}`)
	}
	require.Contains(t, sql, "version = version + 1")
}
