package agentpod

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
				}, {
					Name: "GOMEMLIMIT",
					ValueFrom: &corev1.EnvVarSource{
						ResourceFieldRef: &corev1.ResourceFieldSelector{
							ContainerName: "controller",
							Resource:      "limits.memory",
						},
					},
				}},
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "controller-env"},
					},
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "tmp-data",
						MountPath: "/tmp",
					},
					{
						Name:      "kube-api-access-current",
						MountPath: stepplugincommon.ServiceAccountMountPath,
						ReadOnly:  true,
					},
				},
			}},
			InitContainers: []corev1.Container{{
				Name:  "bootstrap",
				Image: "alpine:3.22",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kube-api-access-current",
					MountPath: stepplugincommon.ServiceAccountMountPath,
					ReadOnly:  true,
				}},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "tmp-data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "git",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "controller-only",
						},
					},
				},
			},
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
		mirroredDependencies{
			envFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mirroredDependencyName("cm", promo, "controller-env"),
					},
				},
			}},
			env: currentPod.Spec.Containers[0].Env,
			volumes: []corev1.Volume{
				currentPod.Spec.Volumes[0],
				{
					Name: mirroredDependencyName("secret", promo, "controller-only"),
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: mirroredDependencyName(
								"secret",
								promo,
								"controller-only",
							),
						},
					},
				},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      "tmp-data",
					MountPath: "/tmp",
				},
				{
					Name:      mirroredDependencyName("secret", promo, "controller-only"),
					MountPath: "/etc/kargo/git",
					ReadOnly:  true,
				},
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, "promotion-agent-promo-uid", pod.Name)
	require.Equal(t, "project-a", pod.Namespace)
	require.Equal(t, "true", pod.Labels["kargo-extended.code.org/promotion-agent"])
	require.Equal(t, "demo-promo", pod.Labels["kargo-extended.code.org/promotion"])
	require.Equal(t, stepplugincommon.AgentContainerName, pod.Annotations["kubectl.kubernetes.io/default-container"])
	require.Empty(t, pod.Spec.ServiceAccountName)
	require.False(t, ptr.Deref(pod.Spec.AutomountServiceAccountToken, true))

	mainContainer := mustFindContainer(t, pod.Spec.Containers, stepplugincommon.AgentContainerName)
	require.Equal(t, []string{"promotion-agent"}, mainContainer.Args)
	require.Equal(t, "ghcr.io/example/kargo:dev", mainContainer.Image)
	require.Contains(t, mainContainer.Env, corev1.EnvVar{Name: "EXISTING_ENV", Value: "present"})
	require.Equal(
		t,
		[]corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mirroredDependencyName("cm", promo, "controller-env"),
				},
			},
		}},
		mainContainer.EnvFrom,
	)
	require.Equal(
		t,
		stepplugincommon.AgentContainerName,
		mustFindEnvVar(t, mainContainer.Env, "GOMEMLIMIT").
			ValueFrom.ResourceFieldRef.ContainerName,
	)
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
			Name:      "tmp-data",
			MountPath: "/tmp",
		},
	)
	require.Contains(
		t,
		mainContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      mirroredDependencyName("secret", promo, "controller-only"),
			MountPath: "/etc/kargo/git",
			ReadOnly:  true,
		},
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
	require.Equal(
		t,
		1,
		countMountPath(
			mainContainer.VolumeMounts,
			stepplugincommon.ServiceAccountMountPath,
		),
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
	require.Equal(
		t,
		"step-plugin-token-init",
		mustFindEnvVar(t, pod.Spec.InitContainers[0].Env, "GOMEMLIMIT").
			ValueFrom.ResourceFieldRef.ContainerName,
	)
	require.Contains(
		t,
		pod.Spec.InitContainers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      stepplugincommon.AuthVolumeName,
			MountPath: stepplugincommon.AuthDir,
		},
	)
	require.Equal(
		t,
		1,
		countMountPath(
			pod.Spec.InitContainers[0].VolumeMounts,
			stepplugincommon.ServiceAccountMountPath,
		),
	)
	require.Contains(
		t,
		pod.Spec.InitContainers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "tmp-data",
			MountPath: "/tmp",
		},
	)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.WorkDirVolumeName)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.AuthVolumeName)
	require.Contains(t, volumeNames(pod.Spec.Volumes), stepplugincommon.ServiceAccountVolumeName)
	require.Contains(t, volumeNames(pod.Spec.Volumes), "tmp-data")
	require.Contains(
		t,
		volumeNames(pod.Spec.Volumes),
		mirroredDependencyName("secret", promo, "controller-only"),
	)
}

func TestEnsureMirroredDependenciesCopiesReferencedObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kargoapi.AddToScheme(scheme))

	promo := &kargoapi.Promotion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-promo",
			Namespace: "project-a",
			UID:       types.UID("promo-uid"),
		},
	}
	currentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-0",
			Namespace: "kargo",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "controller",
				Env: []corev1.EnvVar{{
					Name: "FROM_SECRET",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "git-secret"},
							Key:                  "token",
						},
					},
				}},
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "controller-env"},
					},
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "git",
					MountPath: "/etc/kargo/git",
					ReadOnly:  true,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "git",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "git-secret"},
				},
			}},
		},
	}
	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-env", Namespace: "kargo"},
		Data:       map[string]string{"A": "B"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "git-secret", Namespace: "kargo"},
		Data:       map[string][]byte{"token": []byte("abc")},
		Type:       corev1.SecretTypeOpaque,
	}
	kargoClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(promo.DeepCopy(), cfg, secret).
		Build()

	rt := NewRuntimeWithContext(
		kargoClient,
		kargoClient,
		func() (string, error) { return "controller-0", nil },
		"/unused",
	)

	deps, err := rt.ensureMirroredDependencies(
		t.Context(),
		currentPod,
		promo,
	)
	require.NoError(t, err)
	require.Len(t, deps.envFrom, 1)
	require.Equal(
		t,
		mirroredDependencyName("cm", promo, "controller-env"),
		deps.envFrom[0].ConfigMapRef.Name,
	)
	require.Equal(
		t,
		mirroredDependencyName("secret", promo, "git-secret"),
		deps.env[0].ValueFrom.SecretKeyRef.Name,
	)
	require.Len(t, deps.volumes, 1)
	require.Equal(
		t,
		mirroredDependencyName("secvol", promo, "git"),
		deps.volumes[0].Name,
	)
	require.Equal(
		t,
		mirroredDependencyName("secret", promo, "git-secret"),
		deps.volumes[0].Secret.SecretName,
	)

	mirroredConfigMap := &corev1.ConfigMap{}
	require.NoError(t, kargoClient.Get(
		t.Context(),
		types.NamespacedName{
			Namespace: promo.Namespace,
			Name:      mirroredDependencyName("cm", promo, "controller-env"),
		},
		mirroredConfigMap,
	))
	require.Equal(t, cfg.Data, mirroredConfigMap.Data)

	mirroredSecret := &corev1.Secret{}
	require.NoError(t, kargoClient.Get(
		t.Context(),
		types.NamespacedName{
			Namespace: promo.Namespace,
			Name:      mirroredDependencyName("secret", promo, "git-secret"),
		},
		mirroredSecret,
	))
	require.Equal(t, secret.Data, mirroredSecret.Data)
	require.Len(t, mirroredSecret.OwnerReferences, 1)
	require.Equal(t, string(promo.UID), string(mirroredSecret.OwnerReferences[0].UID))
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
	return mustFindEnvVar(t, env, name).Value
}

func mustFindEnvVar(
	t *testing.T,
	env []corev1.EnvVar,
	name string,
) corev1.EnvVar {
	t.Helper()
	for _, item := range env {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("env var %q not found", name)
	return corev1.EnvVar{}
}

func volumeNames(volumes []corev1.Volume) []string {
	names := make([]string, len(volumes))
	for i, volume := range volumes {
		names[i] = volume.Name
	}
	return names
}

func countMountPath(mounts []corev1.VolumeMount, path string) int {
	count := 0
	for _, mount := range mounts {
		if mount.MountPath == path {
			count++
		}
	}
	return count
}
