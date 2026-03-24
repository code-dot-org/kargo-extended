package plugin

import (
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

// Adapted from Argo Workflows:
// /Users/seth/src/argo-workflows/workflow/util/plugin/configmap.go
// at commit 03ebaaca08c692015338fc88e2fbdff75840cc34.

func ToConfigMap(p *spec.Plugin) (*corev1.ConfigMap, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	containerYAML, err := yaml.Marshal(p.Spec.Sidecar.Container)
	if err != nil {
		return nil, err
	}
	stepsYAML, err := yaml.Marshal(p.Spec.Steps)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.Name + stepplugincommon.ConfigMapNameSuffix,
			Namespace:   p.Namespace,
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Data: map[string]string{
			"sidecar.automountServiceAccountToken": fmt.Sprint(p.Spec.Sidecar.AutomountServiceAccountToken),
			"sidecar.container":                    string(containerYAML),
			"steps.yaml":                           string(stepsYAML),
		},
	}
	maps.Copy(cm.Annotations, p.Annotations)
	maps.Copy(cm.Labels, p.Labels)
	cm.Labels[stepplugincommon.ConfigMapLabelKey] = stepplugincommon.ConfigMapLabelValue
	return cm, nil
}

func FromConfigMap(cm *corev1.ConfigMap) (*spec.Plugin, error) {
	p := &spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kargo-extended.code.org/v1alpha1",
			Kind:       stepplugincommon.ConfigMapLabelValue,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.TrimSuffix(cm.Name, stepplugincommon.ConfigMapNameSuffix),
			Namespace:   cm.Namespace,
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
	}
	maps.Copy(p.Annotations, cm.Annotations)
	maps.Copy(p.Labels, cm.Labels)
	delete(p.Labels, stepplugincommon.ConfigMapLabelKey)

	p.Spec.Sidecar.AutomountServiceAccountToken =
		cm.Data["sidecar.automountServiceAccountToken"] == "true"
	if err := yaml.UnmarshalStrict(
		[]byte(cm.Data["sidecar.container"]),
		&p.Spec.Sidecar.Container,
	); err != nil {
		return nil, err
	}
	if err := yaml.UnmarshalStrict([]byte(cm.Data["steps.yaml"]), &p.Spec.Steps); err != nil {
		return nil, err
	}
	return p, p.Validate()
}
