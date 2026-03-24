package orchestrator

import (
	"context"
	"fmt"
	"slices"
	"strings"

	gocache "github.com/patrickmn/go-cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/health"
	"github.com/akuity/kargo/pkg/kargo"
	"github.com/akuity/kargo/pkg/promotion"
)

type MetadataResolver interface {
	StepMetadata(
		ctx context.Context,
		projectNamespace string,
		stepKind string,
	) (promotion.StepRunnerMetadata, error)
}

type Orchestrator struct {
	executor         promotion.StepExecutor
	metadataResolver MetadataResolver
	client           client.Client
	cacheFunc        promotion.ExprDataCacheFn
}

func New(
	executor promotion.StepExecutor,
	metadataResolver MetadataResolver,
	kargoClient client.Client,
	cacheFunc promotion.ExprDataCacheFn,
) *Orchestrator {
	return &Orchestrator{
		executor:         executor,
		metadataResolver: metadataResolver,
		client:           kargoClient,
		cacheFunc:        cacheFunc,
	}
}

func (o *Orchestrator) ExecuteSteps(
	ctx context.Context,
	promoCtx promotion.Context,
	steps []promotion.Step,
) (promotion.Result, error) {
	if promoCtx.State == nil {
		promoCtx.State = make(promotion.State)
	}

	var healthChecks []health.Criteria

	for i := promoCtx.StartFromStep; i < int64(len(steps)); i++ {
		step := steps[i]
		meta := promoCtx.SetCurrentStep(step)

		select {
		case <-ctx.Done():
			if meta.StartedAt != nil && meta.FinishedAt == nil {
				meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
					"step %q was canceled due to context cancellation: %s",
					step.Alias,
					ctx.Err(),
				).Finished()
			}
			return promotion.Result{
				Status:                kargoapi.PromotionPhaseErrored,
				Message:               fmt.Sprintf("execution canceled: %s", ctx.Err()),
				CurrentStep:           i,
				StepExecutionMetadata: promoCtx.StepExecutionMetadata,
				State:                 promoCtx.State,
				HealthChecks:          healthChecks,
			}, nil
		default:
		}

		processor := promotion.NewStepEvaluator(o.client, o.newCache())

		if meta.StartedAt == nil {
			skip, err := processor.ShouldSkip(ctx, promoCtx, step)
			switch {
			case err != nil:
				meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
					"error checking if step %q should be skipped: %s",
					step.Alias,
					err,
				)
				continue
			case skip:
				meta.WithStatus(kargoapi.PromotionStepStatusSkipped)
				continue
			}
		}

		stepMeta, err := o.metadataResolver.StepMetadata(
			ctx,
			promoCtx.Project,
			step.Kind,
		)
		if err != nil {
			meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
				"error getting runner for step kind %q",
				step.Kind,
			)
			continue
		}

		meta.Started()

		stepCtx, err := processor.BuildStepContext(ctx, promoCtx, step)
		if err != nil {
			meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
				"failed to build step context: %s",
				err,
			)
			continue
		}

		result, err := o.executor.ExecuteStep(ctx, promotion.StepExecutionRequest{
			Context: *stepCtx,
			Step:    step,
		})

		o.propagateStepOutput(promoCtx, step, stepMeta, result)

		if !result.Status.Valid() {
			meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
				"step %q returned an invalid status: %s",
				step.Alias,
				result.Status,
			).Finished()
			continue
		}

		err = o.reconcileResultWithMetadata(promoCtx, step, result, err)
		if !o.determineStepCompletion(promoCtx, step, stepMeta, err) {
			return promotion.Result{
				Status:                kargoapi.PromotionPhaseRunning,
				CurrentStep:           i,
				StepExecutionMetadata: promoCtx.StepExecutionMetadata,
				State:                 promoCtx.State,
				HealthChecks:          healthChecks,
				RetryAfter:            result.RetryAfter,
			}, err
		}

		if meta.Status == kargoapi.PromotionStepStatusSucceeded && result.HealthCheck != nil {
			healthChecks = append(healthChecks, *result.HealthCheck)
		}
	}

	status, msg := promotion.DetermineFinalPhase(steps, promoCtx.StepExecutionMetadata)
	return promotion.Result{
		Status:                status,
		Message:               msg,
		CurrentStep:           int64(len(steps)) - 1,
		StepExecutionMetadata: promoCtx.StepExecutionMetadata,
		State:                 promoCtx.State,
		HealthChecks:          healthChecks,
	}, nil
}

func (o *Orchestrator) newCache() *gocache.Cache {
	if o.cacheFunc == nil {
		return nil
	}
	return o.cacheFunc()
}

func (o *Orchestrator) propagateStepOutput(
	promoCtx promotion.Context,
	step promotion.Step,
	stepMeta promotion.StepRunnerMetadata,
	result promotion.StepResult,
) {
	promoCtx.State[step.Alias] = result.Output

	if slices.Contains(
		stepMeta.RequiredCapabilities,
		promotion.StepCapabilityTaskOutputPropagation,
	) {
		if aliasNamespace := getAliasNamespace(step.Alias); aliasNamespace != "" {
			if promoCtx.State[aliasNamespace] == nil {
				promoCtx.State[aliasNamespace] = make(map[string]any)
			}
			for k, v := range result.Output {
				promoCtx.State[aliasNamespace].(map[string]any)[k] = v // nolint: forcetypeassert
			}
		}
	}
}

func (o *Orchestrator) reconcileResultWithMetadata(
	promoCtx promotion.Context,
	step promotion.Step,
	result promotion.StepResult,
	err error,
) error {
	meta := promoCtx.GetCurrentStep()
	meta.WithStatus(result.Status).WithMessage(result.Message)

	if err != nil {
		if meta.Status != kargoapi.PromotionStepStatusFailed {
			meta.Status = kargoapi.PromotionStepStatusErrored
		}
		meta.WithMessage(err.Error())
		return err
	}

	if result.Status == kargoapi.PromotionStepStatusErrored {
		message := meta.Message
		if message == "" {
			message = "no details provided"
		}
		return fmt.Errorf("step %q errored: %s", step.Alias, message)
	}

	return nil
}

func (o *Orchestrator) determineStepCompletion(
	promoCtx promotion.Context,
	step promotion.Step,
	stepMeta promotion.StepRunnerMetadata,
	err error,
) bool {
	meta := promoCtx.GetCurrentStep()

	switch {
	case meta.Status == kargoapi.PromotionStepStatusSucceeded ||
		meta.Status == kargoapi.PromotionStepStatusSkipped:
		meta.Finished()
		return true
	case meta.Status == kargoapi.PromotionStepStatusFailed && promotion.IsTerminal(err):
		meta.WithMessage(err.Error()).Finished()
		return true
	case promotion.IsTerminal(err):
		meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
			"an unrecoverable error occurred: %s",
			err,
		).Finished()
		return true
	case err != nil:
		meta.Error()
		errorThreshold := step.Retry.GetErrorThreshold(stepMeta.DefaultErrorThreshold)
		if meta.ErrorCount >= errorThreshold {
			status := kargoapi.PromotionStepStatusErrored
			if meta.Status == kargoapi.PromotionStepStatusFailed {
				status = kargoapi.PromotionStepStatusFailed
			}
			meta.WithStatus(status).WithMessagef(
				"step %q met error threshold of %d: %s",
				step.Alias,
				errorThreshold,
				meta.Message,
			).Finished()
			return true
		}
	}

	timeout := step.Retry.GetTimeout(stepMeta.DefaultTimeout)
	if timeout > 0 && metav1.Now().Sub(meta.StartedAt.Time) > timeout {
		meta.WithStatus(kargoapi.PromotionStepStatusErrored).WithMessagef(
			"step %q timed out after %s",
			step.Alias,
			timeout.String(),
		).Finished()
		return true
	}

	return false
}

func getAliasNamespace(alias string) string {
	parts := strings.Split(alias, kargo.PromotionAliasSeparator)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}
