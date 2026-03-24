package agentpod

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
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
	podReader         client.Reader
	controllerPodName func() (string, error)
	namespaceFilePath string
}

type mirroredDependencies struct {
	envFrom      []corev1.EnvFromSource
	env          []corev1.EnvVar
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
}

func NewRuntime(
	kargoClient client.Client,
	podReader client.Reader,
) *Runtime {
	return NewRuntimeWithContext(
		kargoClient,
		podReader,
		defaultControllerPodNameFn,
		defaultNamespaceFilePath,
	)
}

func NewRuntimeWithContext(
	kargoClient client.Client,
	podReader client.Reader,
	controllerPodName func() (string, error),
	namespaceFilePath string,
) *Runtime {
	if podReader == nil {
		podReader = kargoClient
	}
	return &Runtime{
		client:            kargoClient,
		podReader:         podReader,
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
	if err := r.podReader.Get(ctx, key, pod); err == nil {
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
	deps, err := r.ensureMirroredDependencies(ctx, currentPod, promo)
	if err != nil {
		return false, err
	}
	agentPod, err := buildAgentPod(currentPod, promo, pluginSteps, deps)
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
	if err := r.podReader.Get(ctx, client.ObjectKey{
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
	if err := r.podReader.Get(ctx, client.ObjectKey{
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
	deps mirroredDependencies,
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
	mainContainer.Env = retargetContainerEnv(
		deps.env,
		mainContainer.Name,
	)
	mainContainer.EnvFrom = slices.Clone(deps.envFrom)
	mainContainer.VolumeMounts = slices.Clone(deps.volumeMounts)
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
	initContainer.Env = retargetContainerEnv(
		initContainer.Env,
		initContainer.Name,
	)
	initContainer.VolumeMounts = makeAuthMountWritable(initContainer.VolumeMounts)

	volumes := slices.Clone(deps.volumes)
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
			ImagePullSecrets:             slices.Clone(currentPod.Spec.ImagePullSecrets),
			AutomountServiceAccountToken: ptr.To(false),
			RestartPolicy:                corev1.RestartPolicyOnFailure,
			Volumes:                      volumes,
			InitContainers:               []corev1.Container{*initContainer},
			Containers:                   append(pluginContainers, *mainContainer),
			NodeSelector:                 currentPod.Spec.NodeSelector,
			Tolerations:                  slices.Clone(currentPod.Spec.Tolerations),
			Affinity:                     currentPod.Spec.Affinity,
			PriorityClassName:            currentPod.Spec.PriorityClassName,
			SecurityContext:              currentPod.Spec.SecurityContext,
			DNSConfig:                    currentPod.Spec.DNSConfig,
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

func retargetContainerEnv(
	env []corev1.EnvVar,
	containerName string,
) []corev1.EnvVar {
	result := make([]corev1.EnvVar, len(env))
	for i, item := range env {
		result[i] = *item.DeepCopy()
		if result[i].ValueFrom == nil || result[i].ValueFrom.ResourceFieldRef == nil {
			continue
		}
		if result[i].ValueFrom.ResourceFieldRef.ContainerName == "" {
			continue
		}
		result[i].ValueFrom.ResourceFieldRef.ContainerName = containerName
	}
	return result
}

func makeAuthMountWritable(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	result := make([]corev1.VolumeMount, len(mounts))
	for i, mount := range mounts {
		result[i] = *mount.DeepCopy()
		if result[i].MountPath != common.AuthDir {
			continue
		}
		result[i].ReadOnly = false
	}
	return result
}

func appendMounts(
	mounts []corev1.VolumeMount,
	extras ...corev1.VolumeMount,
) []corev1.VolumeMount {
	for _, extra := range extras {
		exists := false
		for _, existing := range mounts {
			if existing.MountPath == extra.MountPath {
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

func (r *Runtime) ensureMirroredDependencies(
	ctx context.Context,
	currentPod *corev1.Pod,
	promo *kargoapi.Promotion,
) (mirroredDependencies, error) {
	deps := mirroredDependencies{}
	controllerContainer, err := findControllerContainer(currentPod)
	if err != nil {
		return deps, err
	}
	volumesByName := map[string]corev1.Volume{}
	for _, volume := range currentPod.Spec.Volumes {
		volumesByName[volume.Name] = volume
	}

	deps.env, err = r.mirrorEnvRefs(ctx, currentPod.Namespace, promo, controllerContainer.Env)
	if err != nil {
		return deps, err
	}
	deps.envFrom, err = r.mirrorEnvFrom(ctx, currentPod.Namespace, promo, controllerContainer.EnvFrom)
	if err != nil {
		return deps, err
	}
	deps.volumeMounts, deps.volumes, err = r.mirrorMountedVolumes(
		ctx,
		currentPod.Namespace,
		promo,
		controllerContainer.VolumeMounts,
		volumesByName,
	)
	if err != nil {
		return deps, err
	}
	return deps, nil
}

func findControllerContainer(currentPod *corev1.Pod) (corev1.Container, error) {
	if len(currentPod.Spec.Containers) == 0 {
		return corev1.Container{}, fmt.Errorf(
			"controller pod %q has no containers",
			currentPod.Name,
		)
	}
	controllerContainer := currentPod.Spec.Containers[0]
	for _, container := range currentPod.Spec.Containers {
		if container.Name == "controller" {
			controllerContainer = container
			break
		}
	}
	return controllerContainer, nil
}

func (r *Runtime) mirrorEnvRefs(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	env []corev1.EnvVar,
) ([]corev1.EnvVar, error) {
	result := make([]corev1.EnvVar, len(env))
	for i, item := range env {
		result[i] = *item.DeepCopy()
		if result[i].ValueFrom == nil {
			continue
		}
		if ref := result[i].ValueFrom.ConfigMapKeyRef; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredConfigMap(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
		if ref := result[i].ValueFrom.SecretKeyRef; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredSecret(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
	}
	return result, nil
}

func (r *Runtime) mirrorEnvFrom(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	envFrom []corev1.EnvFromSource,
) ([]corev1.EnvFromSource, error) {
	result := make([]corev1.EnvFromSource, len(envFrom))
	for i, item := range envFrom {
		result[i] = *item.DeepCopy()
		if ref := result[i].ConfigMapRef; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredConfigMap(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
		if ref := result[i].SecretRef; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredSecret(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
	}
	return result, nil
}

func (r *Runtime) mirrorMountedVolumes(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	mounts []corev1.VolumeMount,
	volumesByName map[string]corev1.Volume,
) ([]corev1.VolumeMount, []corev1.Volume, error) {
	resultMounts := make([]corev1.VolumeMount, 0, len(mounts))
	resultVolumes := make([]corev1.Volume, 0, len(mounts))
	for _, mount := range mounts {
		if mount.MountPath == common.ServiceAccountMountPath {
			continue
		}
		volume, ok := volumesByName[mount.Name]
		if !ok {
			return nil, nil, fmt.Errorf("controller volume %q not found", mount.Name)
		}
		rewritten, err := r.mirrorVolume(ctx, sourceNamespace, promo, volume)
		if err != nil {
			return nil, nil, err
		}
		resultVolumes = appendVolume(resultVolumes, rewritten)
		rewrittenMount := *mount.DeepCopy()
		rewrittenMount.Name = rewritten.Name
		resultMounts = append(resultMounts, rewrittenMount)
	}
	return resultMounts, resultVolumes, nil
}

func (r *Runtime) mirrorVolume(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	volume corev1.Volume,
) (corev1.Volume, error) {
	rewritten := *volume.DeepCopy()
	switch {
	case rewritten.EmptyDir != nil:
		return rewritten, nil
	case rewritten.Secret != nil && rewritten.Secret.SecretName != "":
		name, err := r.ensureMirroredSecret(
			ctx,
			sourceNamespace,
			promo,
			rewritten.Secret.SecretName,
		)
		if err != nil {
			return corev1.Volume{}, err
		}
		rewritten.Name = mirroredDependencyName("secvol", promo, volume.Name)
		rewritten.Secret.SecretName = name
		return rewritten, nil
	case rewritten.ConfigMap != nil && rewritten.ConfigMap.Name != "":
		name, err := r.ensureMirroredConfigMap(
			ctx,
			sourceNamespace,
			promo,
			rewritten.ConfigMap.Name,
		)
		if err != nil {
			return corev1.Volume{}, err
		}
		rewritten.Name = mirroredDependencyName("cmvol", promo, volume.Name)
		rewritten.ConfigMap.Name = name
		return rewritten, nil
	case rewritten.Projected != nil:
		projected, err := r.mirrorProjectedVolume(
			ctx,
			sourceNamespace,
			promo,
			rewritten.Projected,
		)
		if err != nil {
			return corev1.Volume{}, err
		}
		rewritten.Name = mirroredDependencyName("projvol", promo, volume.Name)
		rewritten.Projected = projected
		return rewritten, nil
	default:
		return corev1.Volume{}, fmt.Errorf(
			"unsupported controller volume source for %q",
			volume.Name,
		)
	}
}

func (r *Runtime) mirrorProjectedVolume(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	projected *corev1.ProjectedVolumeSource,
) (*corev1.ProjectedVolumeSource, error) {
	result := projected.DeepCopy()
	for i := range result.Sources {
		if ref := result.Sources[i].ConfigMap; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredConfigMap(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
		if ref := result.Sources[i].Secret; ref != nil && ref.Name != "" {
			name, err := r.ensureMirroredSecret(ctx, sourceNamespace, promo, ref.Name)
			if err != nil {
				return nil, err
			}
			ref.Name = name
		}
	}
	return result, nil
}

func (r *Runtime) ensureMirroredConfigMap(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	sourceName string,
) (string, error) {
	targetName := mirroredDependencyName("cm", promo, sourceName)
	source := &corev1.ConfigMap{}
	if err := r.podReader.Get(ctx, client.ObjectKey{
		Namespace: sourceNamespace,
		Name:      sourceName,
	}, source); err != nil {
		return "", err
	}
	target := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: promo.Namespace, Name: targetName}
	existingResourceVersion := ""
	exists := false
	if err := r.client.Get(ctx, key, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}
		target = &corev1.ConfigMap{
			ObjectMeta: mirroredObjectMeta(targetName, promo),
		}
	} else {
		exists = true
		existingResourceVersion = target.ResourceVersion
	}
	target.ObjectMeta = mirroredObjectMeta(targetName, promo)
	target.ResourceVersion = existingResourceVersion
	target.Immutable = source.Immutable
	target.Data = maps.Clone(source.Data)
	target.BinaryData = maps.Clone(source.BinaryData)
	if err := r.applyMirroredObject(ctx, key, target, exists); err != nil {
		return "", err
	}
	return targetName, nil
}

func (r *Runtime) ensureMirroredSecret(
	ctx context.Context,
	sourceNamespace string,
	promo *kargoapi.Promotion,
	sourceName string,
) (string, error) {
	targetName := mirroredDependencyName("secret", promo, sourceName)
	source := &corev1.Secret{}
	if err := r.podReader.Get(ctx, client.ObjectKey{
		Namespace: sourceNamespace,
		Name:      sourceName,
	}, source); err != nil {
		return "", err
	}
	target := &corev1.Secret{}
	key := client.ObjectKey{Namespace: promo.Namespace, Name: targetName}
	existingResourceVersion := ""
	exists := false
	if err := r.client.Get(ctx, key, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}
		target = &corev1.Secret{
			ObjectMeta: mirroredObjectMeta(targetName, promo),
		}
	} else {
		exists = true
		existingResourceVersion = target.ResourceVersion
	}
	target.ObjectMeta = mirroredObjectMeta(targetName, promo)
	target.ResourceVersion = existingResourceVersion
	target.Immutable = source.Immutable
	target.Type = source.Type
	target.Data = maps.Clone(source.Data)
	target.StringData = nil
	if err := r.applyMirroredObject(ctx, key, target, exists); err != nil {
		return "", err
	}
	return targetName, nil
}

func (r *Runtime) applyMirroredObject(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	exists bool,
) error {
	obj.SetNamespace(key.Namespace)
	obj.SetName(key.Name)
	if !exists {
		obj.SetResourceVersion("")
		return r.client.Create(ctx, obj)
	}
	return r.client.Update(ctx, obj)
}

func mirroredObjectMeta(name string, promo *kargoapi.Promotion) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: promo.Namespace,
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(promo, kargoapi.GroupVersion.WithKind("Promotion")),
		},
	}
}

func mirroredDependencyName(
	kind string,
	promo *kargoapi.Promotion,
	sourceName string,
) string {
	return fmt.Sprintf(
		"sp-%s-%s",
		kind,
		shortHash(string(promo.UID)+":"+sourceName),
	)
}
