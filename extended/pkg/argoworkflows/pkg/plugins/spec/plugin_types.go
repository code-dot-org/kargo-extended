package spec

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Adapted from Argo Workflows:
// /Users/seth/src/argo-workflows/pkg/plugins/spec/plugin_types.go
// at commit 03ebaaca08c692015338fc88e2fbdff75840cc34.

type Plugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              PluginSpec `json:"spec"`
}

func (p Plugin) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if err := p.Spec.Sidecar.Validate(); err != nil {
		return fmt.Errorf("sidecar is invalid: %w", err)
	}
	if len(p.Spec.Steps) == 0 {
		return fmt.Errorf("at least one step is mandatory")
	}
	seenKinds := map[string]struct{}{}
	for i, step := range p.Spec.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %d is invalid: %w", i, err)
		}
		if _, exists := seenKinds[step.Kind]; exists {
			return fmt.Errorf("step kind %q is duplicated", step.Kind)
		}
		seenKinds[step.Kind] = struct{}{}
	}
	return nil
}

type PluginSpec struct {
	Sidecar Sidecar `json:"sidecar"`
	Steps   []Step  `json:"steps"`
}

type Sidecar struct {
	AutomountServiceAccountToken bool             `json:"automountServiceAccountToken,omitempty"`
	Container                    corev1.Container `json:"container"`
}

func (s Sidecar) Validate() error {
	c := s.Container
	if len(c.Ports) < 1 {
		return fmt.Errorf("at least one port is mandatory")
	}
	if c.Resources.Requests == nil {
		return fmt.Errorf("resources requests are mandatory")
	}
	if c.Resources.Limits == nil {
		return fmt.Errorf("resources limits are mandatory")
	}
	if c.SecurityContext == nil {
		return fmt.Errorf("security context is mandatory")
	}
	return nil
}

type Step struct {
	Kind                  string          `json:"kind"`
	DefaultTimeout        metav1.Duration `json:"defaultTimeout,omitempty"`
	DefaultErrorThreshold uint32          `json:"defaultErrorThreshold,omitempty"`
}

func (s Step) Validate() error {
	if strings.TrimSpace(s.Kind) == "" {
		return fmt.Errorf("kind is required")
	}
	return nil
}
