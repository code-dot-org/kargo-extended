package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

func TestConfigMapRoundTrip(t *testing.T) {
	plug := &spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kargo-extended.code.org/v1alpha1",
			Kind:       "StepPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mkdir",
			Namespace:   "project-a",
			Labels:      map[string]string{"team": "platform"},
			Annotations: map[string]string{"owner": "kargo"},
		},
		Spec: spec.PluginSpec{
			Sidecar: spec.Sidecar{
				AutomountServiceAccountToken: true,
				Container: corev1.Container{
					Name:    "mkdir-step-plugin",
					Image:   "python:alpine3.23",
					Command: []string{"python", "-u", "-c"},
					Args:    []string{"print('ok')"},
					Ports:   []corev1.ContainerPort{{ContainerPort: 9765}},
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
			Steps: []spec.Step{
				{
					Kind:                  "mkdir",
					DefaultTimeout:        metav1.Duration{Duration: time.Minute},
					DefaultErrorThreshold: 3,
				},
				{
					Kind: "mkdir-if-missing",
				},
			},
		},
	}

	cm, err := ToConfigMap(plug)
	require.NoError(t, err)
	require.Equal(t, "mkdir-step-plugin", cm.Name)
	require.Equal(
		t,
		stepplugincommon.ConfigMapLabelValue,
		cm.Labels[stepplugincommon.ConfigMapLabelKey],
	)

	roundTripped, err := FromConfigMap(cm)
	require.NoError(t, err)
	require.Equal(t, plug.APIVersion, roundTripped.APIVersion)
	require.Equal(t, plug.Kind, roundTripped.Kind)
	require.Equal(t, plug.Name, roundTripped.Name)
	require.Equal(t, plug.Namespace, roundTripped.Namespace)
	require.Equal(t, plug.Labels, roundTripped.Labels)
	require.Equal(t, plug.Annotations, roundTripped.Annotations)
	require.Equal(
		t,
		plug.Spec.Sidecar.AutomountServiceAccountToken,
		roundTripped.Spec.Sidecar.AutomountServiceAccountToken,
	)
	require.Equal(t, plug.Spec.Sidecar.Container.Name, roundTripped.Spec.Sidecar.Container.Name)
	require.Equal(t, plug.Spec.Sidecar.Container.Image, roundTripped.Spec.Sidecar.Container.Image)
	require.Equal(t, plug.Spec.Sidecar.Container.Command, roundTripped.Spec.Sidecar.Container.Command)
	require.Equal(t, plug.Spec.Sidecar.Container.Args, roundTripped.Spec.Sidecar.Container.Args)
	require.Equal(t, plug.Spec.Sidecar.Container.Ports, roundTripped.Spec.Sidecar.Container.Ports)
	require.Equal(t, plug.Spec.Steps, roundTripped.Spec.Steps)
}

func TestFromConfigMapRejectsInvalidData(t *testing.T) {
	testCases := []struct {
		name string
		cm   *corev1.ConfigMap
	}{
		{
			name: "invalid sidecar container yaml",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mkdir-step-plugin",
					Namespace: "project-a",
					Labels: map[string]string{
						stepplugincommon.ConfigMapLabelKey: stepplugincommon.ConfigMapLabelValue,
					},
				},
				Data: map[string]string{
					"sidecar.automountServiceAccountToken": "false",
					"sidecar.container":                    "{not-valid-yaml",
					"steps.yaml":                           "- kind: mkdir\n",
				},
			},
		},
		{
			name: "duplicate step kinds",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mkdir-step-plugin",
					Namespace: "project-a",
					Labels: map[string]string{
						stepplugincommon.ConfigMapLabelKey: stepplugincommon.ConfigMapLabelValue,
					},
				},
				Data: map[string]string{
					"sidecar.automountServiceAccountToken": "false",
					"sidecar.container": `
name: mkdir-step-plugin
image: python:alpine3.23
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
`,
					"steps.yaml": `
- kind: mkdir
- kind: mkdir
`,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := FromConfigMap(testCase.cm)
			require.Error(t, err)
		})
	}
}
