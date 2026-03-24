package promotions

import (
	"context"
	"time"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/stepplugin/internal/promotionctx"
	"github.com/akuity/kargo/pkg/logging"
	"github.com/akuity/kargo/pkg/promotion"
)

type contextPreparer interface {
	PreparePromotionContext(
		ctx context.Context,
		promo *kargoapi.Promotion,
		stage *kargoapi.Stage,
		actor string,
		uiBaseURL string,
	) (promotion.Context, bool, error)
}

type stepMetadataResolver interface {
	StepMetadata(
		ctx context.Context,
		projectNamespace string,
		stepKind string,
	) (promotion.StepRunnerMetadata, error)
}

func PreparePromotionContext(
	ctx context.Context,
	engine promotion.Engine,
	promo *kargoapi.Promotion,
	stage *kargoapi.Stage,
	actor string,
	uiBaseURL string,
) (promotion.Context, bool, error) {
	if preparer, ok := engine.(contextPreparer); ok {
		return preparer.PreparePromotionContext(ctx, promo, stage, actor, uiBaseURL)
	}

	return promotionctx.NewBuiltinPromotionContext(
		promo,
		stage,
		actor,
		uiBaseURL,
	)
}

func CalculateRequeueInterval(
	ctx context.Context,
	engine promotion.Engine,
	p *kargoapi.Promotion,
	suggestedRequeueInterval *time.Duration,
) time.Duration {
	requeueInterval := 5 * time.Minute
	if suggestedRequeueInterval != nil {
		requeueInterval = *suggestedRequeueInterval
	}
	if int(p.Status.CurrentStep) >= len(p.Spec.Steps) {
		return requeueInterval
	}

	step := p.Spec.Steps[p.Status.CurrentStep]
	resolver, ok := engine.(stepMetadataResolver)
	if !ok {
		reg, err := promotion.DefaultStepRunnerRegistry.Get(step.Uses)
		if err != nil {
			logging.LoggerFromContext(ctx).Error(err, err.Error())
			return requeueInterval
		}
		return requeueIntervalForTimeout(
			requeueInterval,
			p,
			step.Retry.GetTimeout(reg.Metadata.DefaultTimeout),
		)
	}

	meta, err := resolver.StepMetadata(ctx, p.Namespace, step.Uses)
	if err != nil {
		logging.LoggerFromContext(ctx).Error(err, err.Error())
		return requeueInterval
	}
	return requeueIntervalForTimeout(
		requeueInterval,
		p,
		step.Retry.GetTimeout(meta.DefaultTimeout),
	)
}

func requeueIntervalForTimeout(
	requeueInterval time.Duration,
	p *kargoapi.Promotion,
	timeout time.Duration,
) time.Duration {
	if timeout == 0 {
		return requeueInterval
	}
	if int(p.Status.CurrentStep) >= len(p.Status.StepExecutionMetadata) {
		return requeueInterval
	}
	md := p.Status.StepExecutionMetadata[p.Status.CurrentStep]
	targetTimeout := md.StartedAt.Add(timeout)
	if targetTimeout.Before(time.Now().Add(requeueInterval)) {
		return time.Until(targetTimeout)
	}
	return requeueInterval
}
