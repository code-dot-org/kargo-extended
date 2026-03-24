package executor

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/health"
	"github.com/akuity/kargo/pkg/promotion"
)

type StepExecuteRequest struct {
	Context StepContext `json:"context"`
	Step    Step        `json:"step"`
}

type StepContext struct {
	UIBaseURL        string                     `json:"uiBaseURL,omitempty"`
	WorkDir          string                     `json:"workDir,omitempty"`
	SharedState      promotion.State            `json:"sharedState,omitempty"`
	Alias            string                     `json:"alias,omitempty"`
	Config           promotion.Config           `json:"config,omitempty"`
	Project          string                     `json:"project,omitempty"`
	Stage            string                     `json:"stage,omitempty"`
	Promotion        string                     `json:"promotion,omitempty"`
	PromotionUID     string                     `json:"promotionUID,omitempty"`
	PromotionActor   string                     `json:"promotionActor,omitempty"`
	FreightRequests  []kargoapi.FreightRequest  `json:"freightRequests,omitempty"`
	Freight          kargoapi.FreightCollection `json:"freight,omitempty"`
	TargetFreightRef kargoapi.FreightReference  `json:"targetFreightRef,omitempty"`
}

type Step struct {
	Kind   string           `json:"kind"`
	Alias  string           `json:"alias,omitempty"`
	Config promotion.Config `json:"config,omitempty"`
}

type StepExecuteResponse struct {
	Status      kargoapi.PromotionStepStatus `json:"status"`
	Message     string                       `json:"message,omitempty"`
	Output      map[string]any               `json:"output,omitempty"`
	HealthCheck *health.Criteria             `json:"healthCheck,omitempty"`
	RetryAfter  *metav1.Duration             `json:"retryAfter,omitempty"`
	Error       string                       `json:"error,omitempty"`
	Terminal    bool                         `json:"terminal,omitempty"`
}

func ToWireStepExecuteRequest(req promotion.StepExecutionRequest) StepExecuteRequest {
	return StepExecuteRequest{
		Context: StepContext{
			UIBaseURL:        req.Context.UIBaseURL,
			WorkDir:          req.Context.WorkDir,
			SharedState:      req.Context.SharedState.DeepCopy(),
			Alias:            req.Context.Alias,
			Config:           req.Context.Config.DeepCopy(),
			Project:          req.Context.Project,
			Stage:            req.Context.Stage,
			Promotion:        req.Context.Promotion,
			PromotionUID:     req.Context.PromotionUID,
			PromotionActor:   req.Context.PromotionActor,
			FreightRequests:  cloneFreightRequests(req.Context.FreightRequests),
			Freight:          *req.Context.Freight.DeepCopy(),
			TargetFreightRef: *req.Context.TargetFreightRef.DeepCopy(),
		},
		Step: Step{
			Kind:   req.Step.Kind,
			Alias:  req.Step.Alias,
			Config: req.Context.Config.DeepCopy(),
		},
	}
}

func FromWireStepExecuteRequest(req StepExecuteRequest) promotion.StepExecutionRequest {
	return promotion.StepExecutionRequest{
		Context: promotion.StepContext{
			UIBaseURL:        req.Context.UIBaseURL,
			WorkDir:          req.Context.WorkDir,
			SharedState:      req.Context.SharedState.DeepCopy(),
			Alias:            req.Context.Alias,
			Config:           req.Context.Config.DeepCopy(),
			Project:          req.Context.Project,
			Stage:            req.Context.Stage,
			Promotion:        req.Context.Promotion,
			PromotionUID:     req.Context.PromotionUID,
			PromotionActor:   req.Context.PromotionActor,
			FreightRequests:  cloneFreightRequests(req.Context.FreightRequests),
			Freight:          *req.Context.Freight.DeepCopy(),
			TargetFreightRef: *req.Context.TargetFreightRef.DeepCopy(),
		},
		Step: promotion.Step{
			Kind:  req.Step.Kind,
			Alias: req.Step.Alias,
		},
	}
}

func ToWireStepExecuteResponse(
	result promotion.StepResult,
	err error,
) StepExecuteResponse {
	resp := StepExecuteResponse{
		Status:      result.Status,
		Message:     result.Message,
		Output:      result.Output,
		HealthCheck: result.HealthCheck,
		Terminal:    promotion.IsTerminal(err),
	}
	if result.RetryAfter != nil {
		resp.RetryAfter = &metav1.Duration{Duration: *result.RetryAfter}
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

func FromWireStepExecuteResponse(
	resp StepExecuteResponse,
) (promotion.StepResult, error) {
	result := promotion.StepResult{
		Status:      resp.Status,
		Message:     resp.Message,
		Output:      resp.Output,
		HealthCheck: resp.HealthCheck,
	}
	if resp.RetryAfter != nil {
		retryAfter := resp.RetryAfter.Duration
		result.RetryAfter = &retryAfter
	}

	if resp.Error == "" {
		switch resp.Status {
		case kargoapi.PromotionStepStatusFailed, kargoapi.PromotionStepStatusErrored:
			if resp.Message != "" {
				if resp.Terminal || resp.Status == kargoapi.PromotionStepStatusFailed {
					return result, &promotion.TerminalError{Err: errors.New(resp.Message)}
				}
				return result, errors.New(resp.Message)
			}
		}
		return result, nil
	}

	err := errors.New(resp.Error)
	if resp.Terminal {
		err = &promotion.TerminalError{Err: err}
	}
	return result, err
}

func cloneFreightRequests(
	reqs []kargoapi.FreightRequest,
) []kargoapi.FreightRequest {
	if reqs == nil {
		return nil
	}
	cloned := make([]kargoapi.FreightRequest, len(reqs))
	for i, req := range reqs {
		cloned[i] = *req.DeepCopy()
	}
	return cloned
}
