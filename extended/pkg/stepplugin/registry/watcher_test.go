package registry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	toolscache "k8s.io/client-go/tools/cache"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	"github.com/akuity/kargo/pkg/logging"
)

func TestWatcherStartSyncsExistingPlugins(t *testing.T) {
	clientset := k8sfake.NewSimpleClientset(
		buildPluginConfigMap(t, testPluginSpec(t)),
	)
	watcher := newWatcher(clientset)

	ctx, cancel := context.WithCancel(t.Context())
	ctx = logging.ContextWithLogger(
		ctx,
		logging.NewDiscardLoggerOrDie(),
	)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		plugins, err := watcher.Store().Plugins(t.Context(), "system")
		return err == nil && len(plugins) == 1 && plugins[0].Name == "mkdir"
	}, 5*time.Second, 100*time.Millisecond)

	cancel()
	require.NoError(t, <-errCh)
}

func TestWatcherRecordsInvalidConfigMap(t *testing.T) {
	invalid := buildPluginConfigMap(t, testPluginSpec(t))
	invalid.Data["sidecar.container"] = "::not-yaml::"

	clientset := k8sfake.NewSimpleClientset(invalid)
	watcher := newWatcher(clientset)

	ctx, cancel := context.WithCancel(t.Context())
	ctx = logging.ContextWithLogger(
		ctx,
		logging.NewDiscardLoggerOrDie(),
	)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		_, err := watcher.Store().Plugins(t.Context(), "system")
		return err != nil &&
			strings.Contains(err.Error(), "invalid StepPlugin ConfigMaps")
	}, 5*time.Second, 100*time.Millisecond)

	_, err := watcher.Store().Plugins(t.Context(), "system")
	require.ErrorContains(t, err, "invalid StepPlugin ConfigMaps")
	require.ErrorContains(t, err, "mkdir-step-plugin")

	cancel()
	require.NoError(t, <-errCh)
}

func TestWatcherHandleUpsertClearsInvalidWhenConfigMapBecomesValid(t *testing.T) {
	watcher := newWatcher(k8sfake.NewSimpleClientset())
	watcher.Store().SetSynced(true)
	logger := logging.NewDiscardLoggerOrDie()

	invalid := buildPluginConfigMap(t, testPluginSpec(t))
	invalid.Data["sidecar.container"] = "::not-yaml::"

	watcher.handleUpsert(logger, invalid)

	_, err := watcher.Store().Plugins(t.Context(), "system")
	require.ErrorContains(t, err, "invalid StepPlugin ConfigMaps")
	require.ErrorContains(t, err, "mkdir-step-plugin")

	valid := buildPluginConfigMap(t, testPluginSpec(t))
	watcher.handleUpsert(logger, valid)

	plugins, err := watcher.Store().Plugins(t.Context(), "system")
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, "mkdir", plugins[0].Name)
}

func TestWatcherHandleDeleteRemovesPlugin(t *testing.T) {
	watcher := newWatcher(k8sfake.NewSimpleClientset())
	watcher.Store().SetSynced(true)
	logger := logging.NewDiscardLoggerOrDie()

	cm := buildPluginConfigMap(t, testPluginSpec(t))
	watcher.handleUpsert(logger, cm)

	plugins, err := watcher.Store().Plugins(t.Context(), "system")
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	watcher.handleDelete(
		logger,
		toolscache.DeletedFinalStateUnknown{
			Key: "system/" + cm.Name,
			Obj: cm,
		},
	)

	plugins, err = watcher.Store().Plugins(t.Context(), "system")
	require.NoError(t, err)
	require.Empty(t, plugins)
}

func testPluginSpec(t *testing.T) *spec.Plugin {
	t.Helper()
	return &spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kargo-extended.code.org/v1alpha1",
			Kind:       "StepPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mkdir",
			Namespace: "system",
		},
		Spec: spec.PluginSpec{
			Sidecar: validSidecar(),
			Steps:   []spec.Step{{Kind: "mkdir"}},
		},
	}
}
