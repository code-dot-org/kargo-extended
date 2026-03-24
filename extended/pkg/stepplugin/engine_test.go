package stepplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	"github.com/akuity/kargo/extended/pkg/stepplugin/agentpod"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"

	_ "github.com/akuity/kargo/pkg/promotion/runner/builtin"
)

func TestEnginePreparePromotionContextLeavesBuiltinOnlyBehaviorAloneWhenDisabled(t *testing.T) {
	engine := NewEngine(
		newEngineTestClient(t, buildPluginConfigMap(t, "system", "mkdir", "mkdir")),
		nil,
		nil,
		nil,
		"system",
		false,
	)

	promo := &kargoapi.Promotion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "promo",
			Namespace: "project-a",
			UID:       types.UID("builtin-uid"),
		},
		Spec: kargoapi.PromotionSpec{
			Stage:   "stage",
			Freight: "freight",
			Steps: []kargoapi.PromotionStep{{
				Uses: "git-clone",
			}},
		},
	}
	stage := &kargoapi.Stage{ObjectMeta: metav1.ObjectMeta{Name: "stage", Namespace: "project-a"}}

	ctx, fresh, err := engine.PreparePromotionContext(
		t.Context(),
		promo,
		stage,
		"seth",
		"https://ui.example.com",
	)
	require.NoError(t, err)
	require.True(t, fresh)
	require.Equal(
		t,
		filepath.Join(os.TempDir(), "promotion-"+string(promo.UID)),
		ctx.WorkDir,
	)
	require.NotEqual(t, stepplugincommon.WorkDir, ctx.WorkDir)
	require.Equal(t, string(promo.UID), ctx.PromotionUID)
	require.NoError(t, os.RemoveAll(ctx.WorkDir))
}

func TestEnginePreparePromotionContextUsesAgentPathForPluginSteps(t *testing.T) {
	kargoClient := newEngineTestClient(
		t,
		buildPluginConfigMap(t, "system", "mkdir", "mkdir"),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller-0",
				Namespace: "kargo-system",
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "kargo-controller",
				Containers: []corev1.Container{{
					Name:    "controller",
					Image:   "ghcr.io/example/kargo:dev",
					Command: []string{"kargo"},
					Args:    []string{"controller"},
				}},
			},
		},
	)

	namespaceFile := filepath.Join(t.TempDir(), "namespace")
	require.NoError(t, os.WriteFile(namespaceFile, []byte("kargo-system"), 0o600))

	engine := NewEngine(
		kargoClient,
		nil,
		nil,
		nil,
		"system",
		true,
	)
	engine.agentRuntime = agentpod.NewRuntimeWithContext(
		kargoClient,
		func() (string, error) { return "controller-0", nil },
		namespaceFile,
	)

	promo := &kargoapi.Promotion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "promo",
			Namespace: "project-a",
			UID:       types.UID("plugin-uid"),
		},
		Spec: kargoapi.PromotionSpec{
			Stage:   "stage",
			Freight: "freight",
			Steps: []kargoapi.PromotionStep{{
				Uses: "mkdir",
			}},
		},
	}
	stage := &kargoapi.Stage{ObjectMeta: metav1.ObjectMeta{Name: "stage", Namespace: "project-a"}}

	ctx, fresh, err := engine.PreparePromotionContext(
		t.Context(),
		promo,
		stage,
		"seth",
		"https://ui.example.com",
	)
	require.NoError(t, err)
	require.True(t, fresh)
	require.Equal(t, stepplugincommon.WorkDir, ctx.WorkDir)
	require.Equal(t, string(promo.UID), ctx.PromotionUID)

	agentPod := &corev1.Pod{}
	err = kargoClient.Get(
		t.Context(),
		client.ObjectKey{
			Namespace: promo.Namespace,
			Name:      "promotion-agent-" + string(promo.UID),
		},
		agentPod,
	)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(agentPod.Name, "promotion-agent-"))
}

func newEngineTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kargoapi.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

func buildPluginConfigMap(
	t *testing.T,
	namespace string,
	name string,
	stepKind string,
) *corev1.ConfigMap {
	t.Helper()

	cm, err := argoplugin.ToConfigMap(&spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kargo-extended.code.org/v1alpha1",
			Kind:       "StepPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec.PluginSpec{
			Sidecar: spec.Sidecar{
				Container: corev1.Container{
					Name:  name + "-step-plugin",
					Image: "python:alpine3.23",
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
			},
			Steps: []spec.Step{{
				Kind: stepKind,
			}},
		},
	})
	require.NoError(t, err)
	return cm
}
