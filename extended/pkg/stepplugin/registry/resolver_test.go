package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	"github.com/akuity/kargo/pkg/promotion"

	_ "github.com/akuity/kargo/pkg/promotion/runner/builtin"
)

func TestResolverProjectNamespaceOverridesSystemNamespaceByPluginName(t *testing.T) {
	resolver := newTestResolver(
		t,
		buildPluginConfigMap(t, &spec.Plugin{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kargo-extended.code.org/v1alpha1",
				Kind:       "StepPlugin",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo",
				Namespace: "system",
			},
			Spec: spec.PluginSpec{
				Sidecar: validSidecar(),
				Steps: []spec.Step{{
					Kind:           "system-step",
					DefaultTimeout: metav1.Duration{Duration: time.Minute},
				}},
			},
		}),
		buildPluginConfigMap(t, &spec.Plugin{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kargo-extended.code.org/v1alpha1",
				Kind:       "StepPlugin",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo",
				Namespace: "project",
			},
			Spec: spec.PluginSpec{
				Sidecar: validSidecar(),
				Steps: []spec.Step{{
					Kind:           "project-step",
					DefaultTimeout: metav1.Duration{Duration: 2 * time.Minute},
				}},
			},
		}),
	)

	meta, err := resolver.StepMetadata(t.Context(), "project", "project-step")
	require.NoError(t, err)
	require.Equal(t, 2*time.Minute, meta.DefaultTimeout)

	_, err = resolver.StepMetadata(t.Context(), "project", "system-step")
	require.Error(t, err)
}

func TestResolverRejectsBuiltinCollision(t *testing.T) {
	resolver := newTestResolver(
		t,
		buildPluginConfigMap(t, &spec.Plugin{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kargo-extended.code.org/v1alpha1",
				Kind:       "StepPlugin",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo",
				Namespace: "project",
			},
			Spec: spec.PluginSpec{
				Sidecar: validSidecar(),
				Steps: []spec.Step{{
					Kind: "git-clone",
				}},
			},
		}),
	)

	_, err := resolver.ResolvePromotion(
		t.Context(),
		"project",
		[]promotion.Step{{Kind: "git-clone"}},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with a builtin step")
}

func newTestResolver(t *testing.T, configMaps ...*corev1.ConfigMap) *Resolver {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	objects := make([]client.Object, len(configMaps))
	for i, configMap := range configMaps {
		objects[i] = configMap
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
	return NewResolver(fakeClient, promotion.DefaultStepRunnerRegistry, "system", true)
}

func buildPluginConfigMap(t *testing.T, plug *spec.Plugin) *corev1.ConfigMap {
	t.Helper()
	cm, err := argoplugin.ToConfigMap(plug)
	require.NoError(t, err)
	return cm
}

func validSidecar() spec.Sidecar {
	return spec.Sidecar{
		Container: corev1.Container{
			Name:  "demo-plugin",
			Image: "ghcr.io/example/plugin:latest",
			Ports: []corev1.ContainerPort{{ContainerPort: 9765}},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: ptr.To(true),
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
	}
}
