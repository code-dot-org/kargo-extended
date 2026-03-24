package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/promotion"
)

func TestWireRequestRoundTrip(t *testing.T) {
	req := promotion.StepExecutionRequest{
		Context: promotion.StepContext{
			WorkDir:      "/workspace",
			Project:      "demo",
			Promotion:    "promo",
			PromotionUID: "1234",
			Config: promotion.Config{
				"path": "demo/subdir",
			},
		},
		Step: promotion.Step{
			Kind:  "mkdir",
			Alias: "mkdir",
		},
	}

	roundTrip := FromWireStepExecuteRequest(ToWireStepExecuteRequest(req))
	require.Equal(t, req.Context.WorkDir, roundTrip.Context.WorkDir)
	require.Equal(t, req.Context.PromotionUID, roundTrip.Context.PromotionUID)
	require.Equal(t, req.Context.Config, roundTrip.Context.Config)
	require.Equal(t, req.Step.Kind, roundTrip.Step.Kind)
}

func TestWireResponseRoundTripTerminalError(t *testing.T) {
	retryAfter := 10 * time.Second
	result, err := FromWireStepExecuteResponse(StepExecuteResponse{
		Status:     kargoapi.PromotionStepStatusErrored,
		Message:    "boom",
		Error:      "boom",
		Terminal:   true,
		RetryAfter: &metav1.Duration{Duration: retryAfter},
	})
	require.Error(t, err)
	require.True(t, promotion.IsTerminal(err))
	require.NotNil(t, result.RetryAfter)
	require.Equal(t, retryAfter, *result.RetryAfter)
}
