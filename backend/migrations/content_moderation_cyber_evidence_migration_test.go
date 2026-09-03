package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationCyberEvidenceMigration(t *testing.T) {
	content, err := FS.ReadFile("234_content_moderation_cyber_evidence.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, column := range []string{
		"input_snapshot TEXT NOT NULL DEFAULT ''",
		"input_hash VARCHAR(64) NOT NULL DEFAULT ''",
		"input_length INT NOT NULL DEFAULT 0",
		"message_count INT NOT NULL DEFAULT 0",
		"input_truncated BOOLEAN NOT NULL DEFAULT FALSE",
		"protocol VARCHAR(64) NOT NULL DEFAULT ''",
		"audit_stage VARCHAR(32) NOT NULL DEFAULT ''",
		"turn_number INT NOT NULL DEFAULT 0",
		"cyber_policy_mode VARCHAR(16) NOT NULL DEFAULT ''",
	} {
		require.Contains(t, sql, column)
	}
	require.Contains(t, sql, "WHERE action = 'cyber_policy' AND input_hash <> ''")
}
