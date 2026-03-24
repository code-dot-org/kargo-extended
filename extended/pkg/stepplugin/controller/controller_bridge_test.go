package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	"github.com/akuity/kargo/extended/pkg/stepplugin"
	"github.com/akuity/kargo/extended/pkg/stepplugin/registry"
	"github.com/akuity/kargo/pkg/promotion"

	_ "github.com/akuity/kargo/pkg/promotion/runner/builtin"
)

func TestNewPromotionEngineDefaultsStepPluginsOn(t *testing.T) {
	t.Setenv("SYSTEM_RESOURCES_NAMESPACE", "system")

	origNewWatcherFn := newWatcherFn
	t.Cleanup(func() {
		newWatcherFn = origNewWatcherFn
	})

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kargoapi.AddToScheme(scheme))

	cm := testPluginConfigMap(t)
	fakeMgr := &fakePromotionEngineManager{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
		reader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
	}

	store := registry.NewStore()
	store.SetSynced(true)
	plug, err := argoplugin.FromConfigMap(cm)
	require.NoError(t, err)
	store.Upsert(plug)
	newWatcherFn = func(*rest.Config) (discoveryWatcher, error) {
		return fakeDiscoveryWatcher{store: store}, nil
	}

	engine, err := NewPromotionEngine(
		fakeMgr,
		nil,
		nil,
		promotion.DefaultExprDataCacheFn,
	)

	require.NoError(t, err)
	stepPluginEngine, ok := engine.(*stepplugin.Engine)
	require.True(t, ok)
	require.Len(t, fakeMgr.runnables, 1)

	md, err := stepPluginEngine.StepMetadata(t.Context(), "demo", "mkdir")
	require.NoError(t, err)
	require.Equal(t, uint32(1), md.DefaultErrorThreshold)
}

func TestNewPromotionEngineCanDisableStepPlugins(t *testing.T) {
	t.Setenv("STEP_PLUGINS_ENABLED", "false")
	t.Setenv("SYSTEM_RESOURCES_NAMESPACE", "system")

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kargoapi.AddToScheme(scheme))

	cm := testPluginConfigMap(t)
	fakeMgr := &fakePromotionEngineManager{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
		reader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
	}

	engine, err := NewPromotionEngine(
		fakeMgr,
		nil,
		nil,
		promotion.DefaultExprDataCacheFn,
	)
	require.NoError(t, err)

	stepPluginEngine, ok := engine.(*stepplugin.Engine)
	require.True(t, ok)
	require.Empty(t, fakeMgr.runnables)

	_, err = stepPluginEngine.StepMetadata(t.Context(), "demo", "mkdir")
	require.ErrorContains(t, err, `no step runner registered for kind "mkdir"`)
}

func TestNewPromotionEngineReturnsWatcherBootstrapError(t *testing.T) {
	t.Setenv("STEP_PLUGINS_ENABLED", "true")

	origNewWatcherFn := newWatcherFn
	t.Cleanup(func() {
		newWatcherFn = origNewWatcherFn
	})

	newWatcherFn = func(*rest.Config) (discoveryWatcher, error) {
		return nil, context.DeadlineExceeded
	}

	engine, err := NewPromotionEngine(
		&fakePromotionEngineManager{},
		nil,
		nil,
		promotion.DefaultExprDataCacheFn,
	)
	require.Nil(t, engine)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNewPromotionEngineReturnsWatcherAddError(t *testing.T) {
	t.Setenv("STEP_PLUGINS_ENABLED", "true")

	origNewWatcherFn := newWatcherFn
	t.Cleanup(func() {
		newWatcherFn = origNewWatcherFn
	})

	store := registry.NewStore()
	store.SetSynced(true)
	newWatcherFn = func(*rest.Config) (discoveryWatcher, error) {
		return fakeDiscoveryWatcher{store: store}, nil
	}

	addErr := errors.New("boom")
	engine, err := NewPromotionEngine(
		&fakePromotionEngineManager{addErr: addErr},
		nil,
		nil,
		promotion.DefaultExprDataCacheFn,
	)
	require.Nil(t, engine)
	require.ErrorIs(t, err, addErr)
}

func testPluginConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()

	cm, err := argoplugin.ToConfigMap(&spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kargo-extended.code.org/v1alpha1",
			Kind:       "StepPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mkdir",
			Namespace: "system",
		},
		Spec: spec.PluginSpec{
			Sidecar: spec.Sidecar{
				Container: corev1.Container{
					Name:  "mkdir-step-plugin",
					Image: "example.com/mkdir:latest",
					Ports: []corev1.ContainerPort{{ContainerPort: 9765}},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(65534)),
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
			Steps: []spec.Step{{Kind: "mkdir"}},
		},
	})
	require.NoError(t, err)
	return cm
}

type fakePromotionEngineManager struct {
	client    client.Client
	reader    client.Reader
	addErr    error
	runnables []manager.Runnable
}

func (m *fakePromotionEngineManager) Add(r manager.Runnable) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.runnables = append(m.runnables, r)
	return nil
}

func (m *fakePromotionEngineManager) GetAPIReader() client.Reader {
	return m.reader
}

func (m *fakePromotionEngineManager) GetClient() client.Client {
	return m.client
}

func (m *fakePromotionEngineManager) GetConfig() *rest.Config {
	return &rest.Config{Host: "https://example.invalid"}
}

type fakeDiscoveryWatcher struct {
	store *registry.Store
}

func (w fakeDiscoveryWatcher) NeedLeaderElection() bool {
	return true
}

func (w fakeDiscoveryWatcher) Start(context.Context) error {
	return nil
}

func (w fakeDiscoveryWatcher) Store() *registry.Store {
	return w.store
}
