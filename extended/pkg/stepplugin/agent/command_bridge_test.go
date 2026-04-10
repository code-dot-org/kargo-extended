package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/extended/pkg/stepplugin/executor"
)

func TestRunAgentInitWritesReadableTokenPerContainer(t *testing.T) {
	originalAuthDir := authDir
	originalTargets := os.Getenv(common.EnvVarPluginTargets)
	t.Cleanup(func() {
		authDir = originalAuthDir
		require.NoError(t, os.Setenv(common.EnvVarPluginTargets, originalTargets))
	})

	tempDir := t.TempDir()
	authDir = tempDir

	require.NoError(
		t,
		os.Setenv(
			common.EnvVarPluginTargets,
			`{"send-message":{"address":"http://localhost:9765","containerName":"send-message-step-plugin"}}`,
		),
	)

	require.NoError(t, runAgentInit(context.Background()))

	tokenPath := filepath.Join(
		tempDir,
		"send-message-step-plugin",
		common.AuthFilename,
	)
	info, err := os.Stat(tokenPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(authFileMode), info.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Dir(tokenPath))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(authDirMode), dirInfo.Mode().Perm())

	token, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func TestRunAgentInitDeduplicatesContainerNames(t *testing.T) {
	originalAuthDir := authDir
	originalTargets := os.Getenv(common.EnvVarPluginTargets)
	t.Cleanup(func() {
		authDir = originalAuthDir
		require.NoError(t, os.Setenv(common.EnvVarPluginTargets, originalTargets))
	})

	tempDir := t.TempDir()
	authDir = tempDir

	targets := map[string]executor.PluginTarget{
		"send-message": {
			Address:       "http://localhost:9765",
			ContainerName: "shared-plugin",
		},
		"send-message-alt": {
			Address:       "http://localhost:9765",
			ContainerName: "shared-plugin",
		},
	}
	targetsJSON, err := json.Marshal(targets)
	require.NoError(t, err)
	require.NoError(t, os.Setenv(common.EnvVarPluginTargets, string(targetsJSON)))

	require.NoError(t, runAgentInit(context.Background()))

	matches, err := filepath.Glob(filepath.Join(tempDir, "*", common.AuthFilename))
	require.NoError(t, err)
	require.Len(t, matches, 1)
}
