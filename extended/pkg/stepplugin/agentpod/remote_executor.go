package agentpod

import (
	"context"
	"errors"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	"github.com/akuity/kargo/extended/pkg/stepplugin/common"
	wire "github.com/akuity/kargo/extended/pkg/stepplugin/executor"
	"github.com/akuity/kargo/pkg/promotion"
)

type RemoteExecutor struct {
	runtime *Runtime
}

func NewRemoteExecutor(runtime *Runtime) *RemoteExecutor {
	return &RemoteExecutor{runtime: runtime}
}

func (e *RemoteExecutor) ExecuteStep(
	ctx context.Context,
	req promotion.StepExecutionRequest,
) (promotion.StepResult, error) {
	address, ready, err := e.runtime.Address(ctx, promotion.Context{
		Project:      req.Context.Project,
		PromotionUID: req.Context.PromotionUID,
	})
	if err != nil {
		return promotion.StepResult{
			Status: kargoapi.PromotionStepStatusErrored,
		}, err
	}
	if !ready {
		retryAfter := time.Duration(common.DefaultAgentRetrySeconds) * time.Second
		return promotion.StepResult{
			Status:     kargoapi.PromotionStepStatusRunning,
			Message:    "waiting for promotion agent pod",
			RetryAfter: &retryAfter,
		}, nil
	}

	client, err := argoplugin.New(
		address,
		"",
		30*time.Second,
		wait.Backoff{
			Duration: 250 * time.Millisecond,
			Factor:   2,
			Jitter:   0.1,
			Steps:    3,
			Cap:      2 * time.Second,
		},
	)
	if err != nil {
		return promotion.StepResult{
			Status: kargoapi.PromotionStepStatusErrored,
		}, err
	}
	var resp wire.StepExecuteResponse
	if err := client.Call(
		ctx,
		common.MethodStepExecute,
		wire.ToWireStepExecuteRequest(req),
		&resp,
	); err != nil {
		if isTransient(err) {
			retryAfter := time.Duration(common.DefaultAgentRetrySeconds) * time.Second
			return promotion.StepResult{
				Status:     kargoapi.PromotionStepStatusRunning,
				Message:    err.Error(),
				RetryAfter: &retryAfter,
			}, nil
		}
		return promotion.StepResult{
			Status: kargoapi.PromotionStepStatusErrored,
		}, err
	}
	if resp.Status == "" {
		return promotion.StepResult{
				Status: kargoapi.PromotionStepStatusErrored,
			}, &promotion.TerminalError{
				Err: errors.New("promotion agent does not implement step.execute"),
			}
	}
	return wire.FromWireStepExecuteResponse(resp)
}

func isTransient(err error) bool {
	var tempErr interface{ Temporary() bool }
	if errors.As(err, &tempErr) && tempErr.Temporary() {
		return true
	}
	return strings.Contains(err.Error(), "connection refused")
}
