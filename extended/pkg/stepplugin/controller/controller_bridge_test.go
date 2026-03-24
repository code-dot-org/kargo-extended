package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/stepplugin"
	"github.com/akuity/kargo/pkg/promotion"

	_ "github.com/akuity/kargo/pkg/promotion/runner/builtin"
)

func TestNewPromotionEngineReturnsStepPluginEngine(t *testing.T) {
	t.Setenv("STEP_PLUGINS_ENABLED", "true")
	t.Setenv("SYSTEM_RESOURCES_NAMESPACE", "system")

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kargoapi.AddToScheme(scheme))

	engine := NewPromotionEngine(
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		nil,
		nil,
		nil,
		promotion.DefaultExprDataCacheFn,
	)

	stepPluginEngine, ok := engine.(*stepplugin.Engine)
	require.True(t, ok)

	_, err := stepPluginEngine.StepMetadata(t.Context(), "demo", "git-clone")
	require.NoError(t, err)
}
