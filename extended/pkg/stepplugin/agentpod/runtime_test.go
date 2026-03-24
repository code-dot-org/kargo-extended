package agentpod

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/extended/pkg/stepplugin/executor"
	"github.com/akuity/kargo/extended/pkg/stepplugin/registry"
)

func TestBuildAgentPodIncludesSharedWorkDirAndTokenMounts(t *testing.T) {
	currentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-0",
			Namespace: "kargo",
			Labels: map[string]string{
				"app": "kargo",
			},
			Annotations: map[string]string{
				"team": "platform",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "kargo-controller",
			Containers: []corev1.Container{{
				Name:    "controller",
				Image:   "ghcr.io/example/kargo:dev",
				Command: []string{"kargo"},
				Args:    []string{"controller"},
				Env: []corev1.EnvVar{{
					Name:  "EXISTING_ENV",
					Value: "present",
				}},
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "controller-env"},
					},
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "existing-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		},
	}
	promo := &kargoapi.Promotion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-promo",
			Namespace: "project-a",
			UID:       types.UID("promo-uid"),
		},
	}

	pod, err := buildAgentPod(
		currentPod,
		promo,
		map[string]*registry.ResolvedPluginStep{
			"mkdir": testResolvedPluginStep(),
		},
	)
	require.NoError(t, err)
	require.Equal(t, "promotion-agent-promo-uid", pod.Name)
	require.Equal(t, "project-a", pod.Namespace)
	require.Equal(t, "true", pod.Labels["kargo-extended.code.org/promotion-agent"])
	require.Equal(t, "demo-promo", pod.Labels["kargo-extended.code.org/promotion"])
	require.Equal(t, stepplugincommon.AgentContainerName, pod.Annotations["kubectl.kubernetes.io/default-container"])
	require.False(t, ptr.Deref(pod.Spec.AutomountServiceAccountToken, true))

	mainContainer := mustFindContainer(t, pod.Spec.Containers, stepplugincommon.AgentContainerName)
	require.Equal(t, []string{"promotion-agent"}, mainContainer.Args)
	require.Equal(t, "ghcr.io/example/kargo:dev", mainContainer.Image)
	require.Contains(t, mainContainer.Env, corev1.EnvVar{Name: "EXISTING_ENV", Value: "present"})
	targetJSON := mustFindEnv(t, mainContainer.Env, stepplugincommon.EnvVarPluginTargets)
	var targets map[string]executor.PluginTarget
	require.NoError(t, json.Unmarshal([]byte(targetJSON), &targets))
	require.Equal(
		t,
		executor.PluginTarget{
			Address:       "http://localhost:9765",
			ContainerName: "mkdir-step-plugin",
		},
		targets["mkdir"],
	)
	require.Contains(
		t,
		mainContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.WorkDirVolumeName,
			MountPath: stepplugincommon.WorkDir,
		},
	)
	require.Contains(
		t,
		mainContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.AuthVolumeName,
			MountPath: stepplugincommon.AuthDir,
			ReadOnly:  true,
		},
	)

	pluginContainer := mustFindContainer(t, pod.Spec.Containers, "mkdir-step-plugin")
	require.Contains(
		t,
		pluginContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.WorkDirVolumeName,
			MountPath: stepplugincommon.WorkDir,
		},
	)
	require.Contains(
		t,
		pluginContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.AuthVolumeName,
			MountPath: stepplugincommon.AuthDir,
			ReadOnly:  true,
			SubPath:   "mkdir-step-plugin",
		},
	)
	require.Contains(
		t,
		pluginContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.ServiceAccountVolumeName,
			MountPath: stepplugincommon.ServiceAccountMountPath,
			ReadOnly:  true,
		},
	)

	require.Len(t, pod.Spec.InitContainers, 1)
	require.Equal(t, "step-plugin-token-init", pod.Spec.InitContainers[0].Name)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.WorkDirVolumeName)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.AuthVolumeName)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.ServiceAccountVolumeName)
}

func TestBuildPluginContainersRejectsDuplicateContainerNames(t *testing.T) {
	step := testResolvedPluginStep()
	other := testResolvedPluginStep()
	other.PluginNamespace = "project-b"
	other.PluginName = "other"

	_, _, err := buildPluginContainers(
		map[string]*registry.ResolvedPluginStep{
			"mkdir":            step,
			"mkdir-if-missing": other,
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate StepPlugin sidecar container name")
}

func testResolvedPluginStep() *registry.ResolvedPluginStep {
	plug := &spec.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mkdir",
			Namespace: "project-a",
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
				{Kind: "mkdir"},
				{Kind: "mkdir-if-missing"},
			},
		},
	}
	return &registry.ResolvedPluginStep{
		PluginName:      "mkdir",
		PluginNamespace: "project-a",
		Plugin:          plug,
		Step:            plug.Spec.Steps[0],
	}
}

func mustFindContainer(
	t *testing.T,
	containers []corev1.Container,
	name string,
) corev1.Container {
	t.Helper()
	for _, container := range containers {
		if container.Name == name {
			return container
		}
	}
	t.Fatalf("container %q not found", name)
	return corev1.Container{}
}

func mustFindEnv(t *testing.T, env []corev1.EnvVar, name string) string {
	t.Helper()
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	t.Fatalf("env var %q not found", name)
	return ""
}

func volumeNames(volumes []corev1.Volume) []string {
	names := make([]string, len(volumes))
	for i, volume := range volumes {
		names[i] = volume.Name
	}
	return names
}
