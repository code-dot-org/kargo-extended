package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRootCommandIncludesBuild(t *testing.T) {
	cmd := NewRootCommand()
	buildCmd, _, err := cmd.Find([]string{"build"})
	require.NoError(t, err)
	require.NotNil(t, buildCmd)
	require.Equal(t, "build", buildCmd.Name())
}

func TestBuildCommandBuildsConfigMapAndReadme(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plugin.yaml"),
		[]byte(`apiVersion: kargo-extended.code.org/v1alpha1
kind: StepPlugin
metadata:
  name: mkdir
  namespace: kargo-system-resources
spec:
  sidecar:
    automountServiceAccountToken: false
    container:
      name: mkdir-step-plugin
      image: python:alpine
      command:
      - python
      - -u
      - -c
      ports:
      - containerPort: 9765
      securityContext:
        runAsNonRoot: true
      resources:
        requests:
          cpu: 50m
          memory: 32Mi
        limits:
          cpu: 100m
          memory: 64Mi
  steps:
  - kind: mkdir
`),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "server.py"),
		[]byte(`print("hello from plugin")`),
		0o644,
	))

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetArgs([]string{"build", dir})
	require.NoError(t, cmd.Execute())

	configMapData, err := os.ReadFile(
		filepath.Join(dir, "mkdir-step-plugin-configmap.yaml"),
	)
	require.NoError(t, err)
	require.Contains(t, string(configMapData), "kargo-extended.code.org/configmap-type: StepPlugin")
	require.Contains(t, string(configMapData), "steps.yaml")
	require.Contains(t, string(configMapData), "print(\"hello from plugin\")")

	readmeData, err := os.ReadFile(filepath.Join(dir, "README.md"))
	require.NoError(t, err)
	require.Contains(t, string(readmeData), "kubectl apply -f mkdir-step-plugin-configmap.yaml")
}
