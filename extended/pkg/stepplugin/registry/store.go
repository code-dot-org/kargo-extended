package registry

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

type Store struct {
	mu      sync.RWMutex
	synced  bool
	plugins map[string]map[string]*spec.Plugin
	invalid map[string]map[string]error
}

// NewStore returns an empty watched StepPlugin store.
func NewStore() *Store {
	return &Store{
		plugins: map[string]map[string]*spec.Plugin{},
		invalid: map[string]map[string]error{},
	}
}

func (s *Store) Plugins(
	_ context.Context,
	namespace string,
) ([]*spec.Plugin, error) {
	if namespace == "" {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.synced {
		return nil, fmt.Errorf("StepPlugin registry has not synced yet")
	}
	if invalid := s.invalid[namespace]; len(invalid) != 0 {
		names := make([]string, 0, len(invalid))
		for name := range invalid {
			names = append(names, name)
		}
		slices.Sort(names)
		return nil, fmt.Errorf(
			"invalid StepPlugin ConfigMaps in namespace %q: %s",
			namespace,
			strings.Join(names, ", "),
		)
	}

	byName := s.plugins[namespace]
	plugins := make([]*spec.Plugin, 0, len(byName))
	for _, plugin := range byName {
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// SetSynced records whether the store has completed its initial informer sync.
func (s *Store) SetSynced(synced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synced = synced
}

// Upsert adds or replaces a plugin in the watched store.
func (s *Store) Upsert(plugin *spec.Plugin) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plugins[plugin.Namespace]; !ok {
		s.plugins[plugin.Namespace] = map[string]*spec.Plugin{}
	}
	delete(s.invalid[plugin.Namespace], configMapName(plugin.Name))
	s.plugins[plugin.Namespace][plugin.Name] = plugin
}

func (s *Store) recordInvalid(namespace, configMapName string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.invalid[namespace]; !ok {
		s.invalid[namespace] = map[string]error{}
	}
	s.invalid[namespace][configMapName] = err
	delete(s.plugins[namespace], pluginNameFromConfigMap(configMapName))
}

func (s *Store) deleteByConfigMap(namespace, configMapName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.invalid[namespace], configMapName)
	delete(s.plugins[namespace], pluginNameFromConfigMap(configMapName))
}

func configMapName(pluginName string) string {
	return pluginName + stepplugincommon.ConfigMapNameSuffix
}

func pluginNameFromConfigMap(configMapName string) string {
	return strings.TrimSuffix(configMapName, stepplugincommon.ConfigMapNameSuffix)
}
