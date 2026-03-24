package stepplugin

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/stepplugin/agentpod"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/extended/pkg/stepplugin/internal/promotionctx"
	"github.com/akuity/kargo/extended/pkg/stepplugin/orchestrator"
	"github.com/akuity/kargo/extended/pkg/stepplugin/registry"
	"github.com/akuity/kargo/pkg/credentials"
	"github.com/akuity/kargo/pkg/promotion"
)

type Engine struct {
	builtinEngine promotion.Engine
	resolver      *registry.Resolver
	agentRuntime  *agentpod.Runtime
	orchestrator  *orchestrator.Orchestrator
}

func NewEngine(
	kargoClient client.Client,
	argoCDClient client.Client,
	credsDB credentials.Database,
	cacheFunc promotion.ExprDataCacheFn,
	systemResourcesNamespace string,
	enabled bool,
) *Engine {
	resolver := registry.NewResolver(
		kargoClient,
		promotion.DefaultStepRunnerRegistry,
		systemResourcesNamespace,
		enabled,
	)
	agentRuntime := agentpod.NewRuntime(kargoClient)
	return &Engine{
		builtinEngine: promotion.NewLocalEngine(
			kargoClient,
			argoCDClient,
			credsDB,
			cacheFunc,
		),
		resolver:     resolver,
		agentRuntime: agentRuntime,
		orchestrator: orchestrator.New(
			agentpod.NewRemoteExecutor(agentRuntime),
			resolver,
			kargoClient,
			cacheFunc,
		),
	}
}

func (e *Engine) Promote(
	ctx context.Context,
	promoCtx promotion.Context,
	steps []promotion.Step,
) (promotion.Result, error) {
	hasPluginSteps, err := e.resolver.HasPluginSteps(ctx, promoCtx.Project, steps)
	if err != nil {
		return promotion.Result{Status: kargoapi.PromotionPhaseErrored}, err
	}
	if !hasPluginSteps {
		return e.builtinEngine.Promote(ctx, promoCtx, steps)
	}

	result, err := e.orchestrator.ExecuteSteps(ctx, promoCtx, steps)
	if result.Status != kargoapi.PromotionPhaseRunning {
		_ = e.agentRuntime.DeleteAgentPod(ctx, promoCtx)
	}
	return result, err
}

func (e *Engine) PreparePromotionContext(
	ctx context.Context,
	promo *kargoapi.Promotion,
	stage *kargoapi.Stage,
	actor string,
	uiBaseURL string,
) (promotion.Context, bool, error) {
	steps := promotion.NewSteps(promo)
	hasPluginSteps, err := e.resolver.HasPluginSteps(ctx, promo.Namespace, steps)
	if err != nil {
		return promotion.Context{}, false, err
	}

	if !hasPluginSteps {
		return promotionctx.NewBuiltinPromotionContext(
			promo,
			stage,
			actor,
			uiBaseURL,
		)
	}

	pluginSteps, err := e.resolver.ResolvePromotion(ctx, promo.Namespace, steps)
	if err != nil {
		return promotion.Context{}, false, err
	}
	created, err := e.agentRuntime.EnsureAgentPod(ctx, promo, pluginSteps)
	if err != nil {
		return promotion.Context{}, false, err
	}
	return promotion.NewContext(
		promo,
		stage,
		promotion.WithActor(actor),
		promotion.WithUIBaseURL(uiBaseURL),
		promotion.WithWorkDir(stepplugincommon.WorkDir),
	), created, nil
}

func (e *Engine) StepMetadata(
	ctx context.Context,
	projectNamespace string,
	stepKind string,
) (promotion.StepRunnerMetadata, error) {
	return e.resolver.StepMetadata(ctx, projectNamespace, stepKind)
}
