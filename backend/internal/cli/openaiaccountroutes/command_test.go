package openaiaccountroutes

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRunRouteLifecycle(t *testing.T) {
	routesPath := filepath.Join(t.TempDir(), "routes.json")
	t.Setenv(service.OpenAIAPIKeyAccountRoutesFileEnv, routesPath)

	var output bytes.Buffer
	require.NoError(t, Run("sub2api "+Name, []string{"set", "--api-key-id", "9", "--account-id", "30"}, &output))
	require.Equal(t, "set api_key_id=9 account_id=30\n", output.String())

	output.Reset()
	require.NoError(t, Run("sub2api "+Name, []string{"set", "--api-key-id", "2", "--account-id", "70"}, &output))

	output.Reset()
	require.NoError(t, Run("sub2api "+Name, []string{"list"}, &output))
	require.Equal(t, "api_key_id=2 account_id=70\napi_key_id=9 account_id=30\n", output.String())

	output.Reset()
	require.NoError(t, Run("sub2api "+Name, []string{"delete", "--api-key-id", "9"}, &output))
	require.Equal(t, "deleted api_key_id=9\n", output.String())

	output.Reset()
	require.NoError(t, Run("sub2api "+Name, []string{"list"}, &output))
	require.Equal(t, "api_key_id=2 account_id=70\n", output.String())

	info, err := os.Stat(routesPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRunValidatesArguments(t *testing.T) {
	var output bytes.Buffer
	require.EqualError(t, Run("sub2api "+Name, nil, &output), "usage: sub2api openai-account-routes <list|set|delete> [flags]")
	require.EqualError(t, Run("sub2api "+Name, []string{"set", "--api-key-id", "1"}, &output), "set requires --api-key-id and --account-id")
	require.EqualError(t, Run("sub2api "+Name, []string{"delete", "--api-key-id", "invalid"}, &output), "--api-key-id must be a positive integer")
}
