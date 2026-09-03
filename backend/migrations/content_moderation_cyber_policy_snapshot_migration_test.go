package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationCyberPolicySnapshotMigration(t *testing.T) {
	content, err := FS.ReadFile("235_content_moderation_cyber_policy_snapshot.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "cyber_policy_source VARCHAR(32) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "cyber_policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "idx_content_moderation_logs_cyber_group_count")
	require.Contains(t, sql, "WHERE action = 'cyber_policy'")
}
