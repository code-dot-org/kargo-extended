package registry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

type liveSource struct {
	reader client.Reader
}

func (s *liveSource) Plugins(
	ctx context.Context,
	namespace string,
) ([]*spec.Plugin, error) {
	if namespace == "" {
		return nil, nil
	}

	configMaps := &corev1.ConfigMapList{}
	if err := s.reader.List(
		ctx,
		configMaps,
		client.InNamespace(namespace),
		client.MatchingLabels{
			stepplugincommon.ConfigMapLabelKey: stepplugincommon.ConfigMapLabelValue,
		},
	); err != nil {
		return nil, fmt.Errorf(
			"error listing StepPlugin ConfigMaps in namespace %q: %w",
			namespace,
			err,
		)
	}

	plugins := make([]*spec.Plugin, 0, len(configMaps.Items))
	for i := range configMaps.Items {
		plugin, err := argoplugin.FromConfigMap(&configMaps.Items[i])
		if err != nil {
			return nil, fmt.Errorf(
				"error parsing StepPlugin ConfigMap %q in namespace %q: %w",
				configMaps.Items[i].Name,
				configMaps.Items[i].Namespace,
				err,
			)
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}
