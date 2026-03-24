package executor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

func TestDispatcherExecuteStepUsesPluginTarget(t *testing.T) {
	originalAuthDir := authDir
	authDir = t.TempDir()
	t.Cleanup(func() {
		authDir = originalAuthDir
	})

	require.NoError(
		t,
		os.MkdirAll(filepath.Join(authDir, "mkdir-step-plugin"), 0o755),
	)
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(authDir, "mkdir-step-plugin", stepplugincommon.AuthFilename),
			[]byte("plugin-secret"),
			0o600,
		),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			stepplugincommon.APIPathPrefix+stepplugincommon.MethodStepExecute,
			r.URL.Path,
		)
		require.Equal(t, "Bearer plugin-secret", r.Header.Get("Authorization"))

		var req StepExecuteRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "mkdir", req.Step.Kind)
		require.Equal(t, "/workspace", req.Context.WorkDir)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(StepExecuteResponse{
			Status:  kargoapi.PromotionStepStatusSucceeded,
			Message: "created",
			Output: map[string]any{
				"path": "demo/subdir",
			},
		}))
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(
		nil,
		nil,
		nil,
		map[string]PluginTarget{
			"mkdir": {
				Address:       server.URL,
				ContainerName: "mkdir-step-plugin",
			},
		},
	)
	require.NoError(t, err)

	resp := dispatcher.ExecuteStep(
		t.Context(),
		StepExecuteRequest{
			Context: StepContext{
				WorkDir: "/workspace",
			},
			Step: Step{
				Kind: "mkdir",
			},
		},
	)
	require.Equal(t, kargoapi.PromotionStepStatusSucceeded, resp.Status)
	require.Equal(t, "created", resp.Message)
	require.Equal(t, "demo/subdir", resp.Output["path"])
}
