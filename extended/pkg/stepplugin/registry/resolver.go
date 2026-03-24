package registry

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	"github.com/akuity/kargo/pkg/promotion"
)

type ResolvedPluginStep struct {
	PluginName      string
	PluginNamespace string
	Plugin          *spec.Plugin
	Step            spec.Step
	Metadata        promotion.StepRunnerMetadata
}

type Resolver struct {
	source                   source
	builtinRegistry          promotion.StepRunnerRegistry
	systemResourcesNamespace string
	enabled                  bool
}

type source interface {
	Plugins(ctx context.Context, namespace string) ([]*spec.Plugin, error)
}

func NewResolver(
	kargoClient client.Reader,
	builtinRegistry promotion.StepRunnerRegistry,
	systemResourcesNamespace string,
	enabled bool,
) *Resolver {
	return &Resolver{
		source:                   &liveSource{reader: kargoClient},
		builtinRegistry:          builtinRegistry,
		systemResourcesNamespace: systemResourcesNamespace,
		enabled:                  enabled,
	}
}

// NewWatchedResolver builds a resolver backed by a watched in-memory store.
func NewWatchedResolver(
	store *Store,
	builtinRegistry promotion.StepRunnerRegistry,
	systemResourcesNamespace string,
	enabled bool,
) *Resolver {
	return &Resolver{
		source:                   store,
		builtinRegistry:          builtinRegistry,
		systemResourcesNamespace: systemResourcesNamespace,
		enabled:                  enabled,
	}
}

func (r *Resolver) Enabled() bool {
	return r != nil && r.enabled
}

func (r *Resolver) HasPluginSteps(
	ctx context.Context,
	projectNamespace string,
	steps []promotion.Step,
) (bool, error) {
	if !r.Enabled() {
		return false, nil
	}
	plugins, err := r.resolveEffectiveStepKinds(ctx, projectNamespace)
	if err != nil {
		return false, err
	}
	for _, step := range steps {
		if _, ok := plugins[step.Kind]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (r *Resolver) StepMetadata(
	ctx context.Context,
	projectNamespace string,
	stepKind string,
) (promotion.StepRunnerMetadata, error) {
	if reg, err := r.builtinRegistry.Get(stepKind); err == nil {
		return reg.Metadata, nil
	}
	if !r.Enabled() {
		return promotion.StepRunnerMetadata{},
			fmt.Errorf("no step runner registered for kind %q", stepKind)
	}

	plugins, err := r.resolveEffectiveStepKinds(ctx, projectNamespace)
	if err != nil {
		return promotion.StepRunnerMetadata{}, err
	}
	pluginStep, ok := plugins[stepKind]
	if !ok {
		return promotion.StepRunnerMetadata{},
			fmt.Errorf("no step runner registered for kind %q", stepKind)
	}
	return pluginStep.Metadata, nil
}

func (r *Resolver) ResolveStep(
	ctx context.Context,
	projectNamespace string,
	stepKind string,
) (*ResolvedPluginStep, bool, error) {
	if !r.Enabled() {
		return nil, false, nil
	}
	plugins, err := r.resolveEffectiveStepKinds(ctx, projectNamespace)
	if err != nil {
		return nil, false, err
	}
	pluginStep, ok := plugins[stepKind]
	return pluginStep, ok, nil
}

func (r *Resolver) ResolvePromotion(
	ctx context.Context,
	projectNamespace string,
	steps []promotion.Step,
) (map[string]*ResolvedPluginStep, error) {
	if !r.Enabled() {
		return map[string]*ResolvedPluginStep{}, nil
	}
	all, err := r.resolveEffectiveStepKinds(ctx, projectNamespace)
	if err != nil {
		return nil, err
	}
	result := map[string]*ResolvedPluginStep{}
	for _, step := range steps {
		if pluginStep, ok := all[step.Kind]; ok {
			result[step.Kind] = pluginStep
		}
	}
	return result, nil
}

func (r *Resolver) resolveEffectiveStepKinds(
	ctx context.Context,
	projectNamespace string,
) (map[string]*ResolvedPluginStep, error) {
	effectivePlugins, err := r.resolveEffectivePlugins(ctx, projectNamespace)
	if err != nil {
		return nil, err
	}

	stepsByKind := map[string]*ResolvedPluginStep{}
	for _, plugin := range effectivePlugins {
		for _, pluginStep := range plugin.Spec.Steps {
			if _, err := r.builtinRegistry.Get(pluginStep.Kind); err == nil {
				return nil, fmt.Errorf(
					"plugin %q in namespace %q declares step kind %q, which collides with a builtin step",
					plugin.Name,
					plugin.Namespace,
					pluginStep.Kind,
				)
			}
			if existing, ok := stepsByKind[pluginStep.Kind]; ok {
				return nil, fmt.Errorf(
					"plugin %q in namespace %q declares step kind %q, which already belongs to plugin %q in namespace %q",
					plugin.Name,
					plugin.Namespace,
					pluginStep.Kind,
					existing.PluginName,
					existing.PluginNamespace,
				)
			}
			stepsByKind[pluginStep.Kind] = &ResolvedPluginStep{
				PluginName:      plugin.Name,
				PluginNamespace: plugin.Namespace,
				Plugin:          plugin,
				Step:            pluginStep,
				Metadata: promotion.StepRunnerMetadata{
					DefaultTimeout:        pluginStep.DefaultTimeout.Duration,
					DefaultErrorThreshold: defaultErrorThreshold(pluginStep),
				},
			}
		}
	}
	return stepsByKind, nil
}

func defaultErrorThreshold(step spec.Step) uint32 {
	if step.DefaultErrorThreshold == 0 {
		return 1
	}
	return step.DefaultErrorThreshold
}

func (r *Resolver) resolveEffectivePlugins(
	ctx context.Context,
	projectNamespace string,
) (map[string]*spec.Plugin, error) {
	plugins := map[string]*spec.Plugin{}

	systemPlugins, err := r.source.Plugins(ctx, r.systemResourcesNamespace)
	if err != nil {
		return nil, err
	}
	for _, plugin := range systemPlugins {
		plugins[plugin.Name] = plugin
	}

	if projectNamespace != "" && projectNamespace != r.systemResourcesNamespace {
		projectPlugins, err := r.source.Plugins(ctx, projectNamespace)
		if err != nil {
			return nil, err
		}
		for _, plugin := range projectPlugins {
			plugins[plugin.Name] = plugin
		}
	}

	return plugins, nil
}
