package agentpod

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/extended/pkg/stepplugin/executor"
	"github.com/akuity/kargo/extended/pkg/stepplugin/registry"
	"github.com/akuity/kargo/pkg/promotion"
)

var (
	defaultControllerPodNameFn = os.Hostname
	defaultNamespaceFilePath   = filepath.Join(
		common.ServiceAccountMountPath,
		"namespace",
	)
)

type Runtime struct {
	client            client.Client
	controllerPodName func() (string, error)
	namespaceFilePath string
}

func NewRuntime(kargoClient client.Client) *Runtime {
	return NewRuntimeWithContext(
		kargoClient,
		defaultControllerPodNameFn,
		defaultNamespaceFilePath,
	)
}

func NewRuntimeWithContext(
	kargoClient client.Client,
	controllerPodName func() (string, error),
	namespaceFilePath string,
) *Runtime {
	return &Runtime{
		client:            kargoClient,
		controllerPodName: controllerPodName,
		namespaceFilePath: namespaceFilePath,
	}
}

func (r *Runtime) EnsureAgentPod(
	ctx context.Context,
	promo *kargoapi.Promotion,
	pluginSteps map[string]*registry.ResolvedPluginStep,
) (bool, error) {
	pod := &corev1.Pod{}
	key := client.ObjectKey{
		Namespace: promo.Namespace,
		Name:      agentPodName(string(promo.UID)),
	}
	if err := r.client.Get(ctx, key, pod); err == nil {
		switch pod.Status.Phase {
		case corev1.PodFailed, corev1.PodSucceeded:
			deleteErr := r.client.Delete(ctx, pod)
			if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				return false, deleteErr
			}
		default:
			return false, nil
		}
	} else if !apierrors.IsNotFound(err) {
		return false, err
	}

	currentPod, err := r.currentControllerPod(ctx)
	if err != nil {
		return false, err
	}
	agentPod, err := buildAgentPod(currentPod, promo, pluginSteps)
	if err != nil {
		return false, err
	}
	if err := r.client.Create(ctx, agentPod); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Runtime) DeleteAgentPod(
	ctx context.Context,
	promoCtx promotion.Context,
) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: promoCtx.Project,
			Name:      agentPodName(promoCtx.PromotionUID),
		},
	}
	if err := r.client.Delete(ctx, pod); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *Runtime) Address(
	ctx context.Context,
	promoCtx promotion.Context,
) (string, bool, error) {
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: promoCtx.Project,
		Name:      agentPodName(promoCtx.PromotionUID),
	}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if pod.Status.PodIP == "" || pod.Status.Phase != corev1.PodRunning || !isPodReady(pod) {
		return "", false, nil
	}
	return fmt.Sprintf("http://%s:%d", pod.Status.PodIP, common.AgentContainerPort), true, nil
}

func (r *Runtime) currentControllerPod(ctx context.Context) (*corev1.Pod, error) {
	podName, err := r.controllerPodName()
	if err != nil {
		return nil, err
	}
	namespaceBytes, err := os.ReadFile(r.namespaceFilePath)
	if err != nil {
		return nil, err
	}
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: strings.TrimSpace(string(namespaceBytes)),
		Name:      podName,
	}, pod); err != nil {
		return nil, err
	}
	return pod, nil
}

func buildAgentPod(
	currentPod *corev1.Pod,
	promo *kargoapi.Promotion,
	pluginSteps map[string]*registry.ResolvedPluginStep,
) (*corev1.Pod, error) {
	if len(currentPod.Spec.Containers) == 0 {
		return nil, fmt.Errorf("controller pod %q has no containers", currentPod.Name)
	}

	controllerContainer := currentPod.Spec.Containers[0]
	for _, container := range currentPod.Spec.Containers {
		if container.Name == "controller" {
			controllerContainer = container
			break
		}
	}

	pluginContainers, pluginTargets, err := buildPluginContainers(pluginSteps)
	if err != nil {
		return nil, err
	}
	targetsJSON, err := json.Marshal(pluginTargets)
	if err != nil {
		return nil, err
	}

	mainContainer := controllerContainer.DeepCopy()
	mainContainer.Name = common.AgentContainerName
	mainContainer.Args = []string{"promotion-agent"}
	mainContainer.Ports = []corev1.ContainerPort{{
		ContainerPort: common.AgentContainerPort,
	}}
	mainContainer.Env = append(mainContainer.Env, corev1.EnvVar{
		Name:  common.EnvVarPluginTargets,
		Value: string(targetsJSON),
	})
	mainContainer.VolumeMounts = appendMounts(
		mainContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      common.WorkDirVolumeName,
			MountPath: common.WorkDir,
		},
		corev1.VolumeMount{
			Name:      common.AuthVolumeName,
			MountPath: common.AuthDir,
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      common.ServiceAccountVolumeName,
			MountPath: common.ServiceAccountMountPath,
			ReadOnly:  true,
		},
	)

	initContainer := mainContainer.DeepCopy()
	initContainer.Name = "step-plugin-token-init"
	initContainer.Ports = nil
	initContainer.Args = []string{"promotion-agent", "init"}

	volumes := append([]corev1.Volume{}, currentPod.Spec.Volumes...)
	volumes = appendVolume(volumes, corev1.Volume{
		Name: common.WorkDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumes = appendVolume(volumes, corev1.Volume{
		Name: common.AuthVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumes = appendVolume(volumes, serviceAccountVolume())

	annotations := map[string]string{}
	for k, v := range currentPod.Annotations {
		annotations[k] = v
	}
	annotations["kubectl.kubernetes.io/default-container"] = common.AgentContainerName

	labels := map[string]string{}
	for k, v := range currentPod.Labels {
		labels[k] = v
	}
	labels["kargo-extended.code.org/promotion-agent"] = "true"
	labels["kargo-extended.code.org/promotion"] = promo.Name

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        agentPodName(string(promo.UID)),
			Namespace:   promo.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(promo, kargoapi.GroupVersion.WithKind("Promotion")),
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName:           currentPod.Spec.ServiceAccountName,
			ImagePullSecrets:             slices.Clone(currentPod.Spec.ImagePullSecrets),
			AutomountServiceAccountToken: ptr.To(false),
			RestartPolicy:                corev1.RestartPolicyOnFailure,
			Volumes:                      volumes,
			InitContainers: append(
				slices.Clone(currentPod.Spec.InitContainers),
				*initContainer,
			),
			Containers:        append(pluginContainers, *mainContainer),
			NodeSelector:      currentPod.Spec.NodeSelector,
			Tolerations:       slices.Clone(currentPod.Spec.Tolerations),
			Affinity:          currentPod.Spec.Affinity,
			PriorityClassName: currentPod.Spec.PriorityClassName,
			SecurityContext:   currentPod.Spec.SecurityContext,
			DNSConfig:         currentPod.Spec.DNSConfig,
		},
	}
	return pod, nil
}

func buildPluginContainers(
	pluginSteps map[string]*registry.ResolvedPluginStep,
) ([]corev1.Container, map[string]executor.PluginTarget, error) {
	pluginsByKey := map[string]*registry.ResolvedPluginStep{}
	for _, step := range pluginSteps {
		key := step.PluginNamespace + "/" + step.PluginName
		if _, ok := pluginsByKey[key]; !ok {
			pluginsByKey[key] = step
		}
	}

	containers := make([]corev1.Container, 0, len(pluginsByKey))
	targets := map[string]executor.PluginTarget{}
	containerNames := map[string]struct{}{}
	for _, step := range pluginsByKey {
		container := step.Plugin.Spec.Sidecar.Container.DeepCopy()
		if _, exists := containerNames[container.Name]; exists {
			return nil, nil,
				fmt.Errorf("duplicate StepPlugin sidecar container name %q", container.Name)
		}
		containerNames[container.Name] = struct{}{}
		container.VolumeMounts = appendMounts(
			container.VolumeMounts,
			corev1.VolumeMount{
				Name:      common.WorkDirVolumeName,
				MountPath: common.WorkDir,
			},
			corev1.VolumeMount{
				Name:      common.AuthVolumeName,
				MountPath: common.AuthDir,
				ReadOnly:  true,
				SubPath:   container.Name,
			},
		)
		if step.Plugin.Spec.Sidecar.AutomountServiceAccountToken {
			container.VolumeMounts = appendMounts(
				container.VolumeMounts,
				corev1.VolumeMount{
					Name:      common.ServiceAccountVolumeName,
					MountPath: common.ServiceAccountMountPath,
					ReadOnly:  true,
				},
			)
		}
		containers = append(containers, *container)
		for _, pluginStep := range step.Plugin.Spec.Steps {
			if len(container.Ports) == 0 {
				return nil, nil, fmt.Errorf(
					"StepPlugin sidecar container %q has no ports",
					container.Name,
				)
			}
			targets[pluginStep.Kind] = executor.PluginTarget{
				Address: fmt.Sprintf(
					"http://localhost:%d",
					container.Ports[0].ContainerPort,
				),
				ContainerName: container.Name,
			}
		}
	}
	return containers, targets, nil
}

func serviceAccountVolume() corev1.Volume {
	return corev1.Volume{
		Name: common.ServiceAccountVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Path: "token",
						},
					},
					{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "kube-root-ca.crt",
							},
							Items: []corev1.KeyToPath{{
								Key:  "ca.crt",
								Path: "ca.crt",
							}},
						},
					},
					{
						DownwardAPI: &corev1.DownwardAPIProjection{
							Items: []corev1.DownwardAPIVolumeFile{{
								Path: "namespace",
								FieldRef: &corev1.ObjectFieldSelector{
									APIVersion: "v1",
									FieldPath:  "metadata.namespace",
								},
							}},
						},
					},
				},
			},
		},
	}
}

func agentPodName(promoUID string) string {
	if promoUID == "" {
		return "promotion-agent-" + shortHash("")
	}
	return "promotion-agent-" + promoUID
}

func shortHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:10]
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func appendVolume(volumes []corev1.Volume, volume corev1.Volume) []corev1.Volume {
	for _, existing := range volumes {
		if existing.Name == volume.Name {
			return volumes
		}
	}
	return append(volumes, volume)
}

func appendMounts(
	mounts []corev1.VolumeMount,
	extras ...corev1.VolumeMount,
) []corev1.VolumeMount {
	for _, extra := range extras {
		exists := false
		for _, existing := range mounts {
			if existing.Name == extra.Name && existing.MountPath == extra.MountPath {
				exists = true
				break
			}
		}
		if !exists {
			mounts = append(mounts, extra)
		}
	}
	return mounts
}
