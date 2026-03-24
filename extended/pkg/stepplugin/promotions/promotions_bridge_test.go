package promotions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/promotion"
)

type fakeEngine struct {
	ctx   promotion.Context
	fresh bool
	meta  promotion.StepRunnerMetadata
}

func (f *fakeEngine) Promote(
	context.Context,
	promotion.Context,
	[]promotion.Step,
) (promotion.Result, error) {
	return promotion.Result{}, nil
}

func (f *fakeEngine) PreparePromotionContext(
	context.Context,
	*kargoapi.Promotion,
	*kargoapi.Stage,
	string,
	string,
) (promotion.Context, bool, error) {
	return f.ctx, f.fresh, nil
}

func (f *fakeEngine) StepMetadata(
	context.Context,
	string,
	string,
) (promotion.StepRunnerMetadata, error) {
	return f.meta, nil
}

func TestPreparePromotionContextDelegatesToEngine(t *testing.T) {
	expected := promotion.Context{WorkDir: "/workspace"}
	ctx, fresh, err := PreparePromotionContext(
		t.Context(),
		&fakeEngine{ctx: expected, fresh: true},
		&kargoapi.Promotion{},
		nil,
		"actor",
		"https://ui",
	)
	require.NoError(t, err)
	require.Equal(t, expected, ctx)
	require.True(t, fresh)
}

func TestCalculateRequeueIntervalUsesEngineMetadata(t *testing.T) {
	requeue := CalculateRequeueInterval(
		t.Context(),
		&fakeEngine{
			meta: promotion.StepRunnerMetadata{
				DefaultTimeout: time.Minute,
			},
		},
		&kargoapi.Promotion{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "demo",
			},
			Spec: kargoapi.PromotionSpec{
				Steps: []kargoapi.PromotionStep{{
					Uses: "plugin-step",
				}},
			},
			Status: kargoapi.PromotionStatus{
				CurrentStep: 0,
				StepExecutionMetadata: []kargoapi.StepExecutionMetadata{{
					StartedAt: &metav1.Time{
						Time: time.Now().Add(-30 * time.Second),
					},
				}},
			},
		},
		nil,
	)
	require.Less(t, requeue, 5*time.Minute)
	require.Greater(t, requeue, time.Duration(0))
}
