package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/pkg/credentials"
	"github.com/akuity/kargo/pkg/promotion"
)

var authDir = stepplugincommon.AuthDir

type PluginTarget struct {
	Address       string `json:"address"`
	ContainerName string `json:"containerName"`
}

type Dispatcher struct {
	executor promotion.StepExecutor
	plugins  map[string]*argoplugin.Client
}

func NewDispatcher(
	kargoClient client.Client,
	argoCDClient client.Client,
	credsDB credentials.Database,
	targets map[string]PluginTarget,
) (*Dispatcher, error) {
	plugins := make(map[string]*argoplugin.Client, len(targets))
	for kind, target := range targets {
		token, err := readAuthFile(target.ContainerName)
		if err != nil {
			return nil, err
		}

		pluginClient, err := argoplugin.New(
			target.Address,
			token,
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
			return nil, err
		}
		plugins[kind] = pluginClient
	}

	return &Dispatcher{
		executor: promotion.NewLocalStepExecutor(
			promotion.DefaultStepRunnerRegistry,
			kargoClient,
			argoCDClient,
			credsDB,
		),
		plugins: plugins,
	}, nil
}

func (d *Dispatcher) ExecuteStep(
	ctx context.Context,
	req StepExecuteRequest,
) StepExecuteResponse {
	if plug, ok := d.plugins[req.Step.Kind]; ok {
		var resp StepExecuteResponse
		err := plug.Call(ctx, stepplugincommon.MethodStepExecute, req, &resp)
		if err == nil {
			if resp.Status == "" {
				return StepExecuteResponse{
					Status:   kargoapi.PromotionStepStatusErrored,
					Message:  "plugin does not implement step.execute",
					Error:    "plugin does not implement step.execute",
					Terminal: true,
				}
			}
			return resp
		}
		if isTransient(err) {
			retryAfter := time.Duration(stepplugincommon.DefaultAgentRetrySeconds) * time.Second
			return StepExecuteResponse{
				Status:     kargoapi.PromotionStepStatusRunning,
				Message:    err.Error(),
				RetryAfter: &metav1.Duration{Duration: retryAfter},
			}
		}
		return StepExecuteResponse{
			Status:   kargoapi.PromotionStepStatusErrored,
			Message:  err.Error(),
			Error:    err.Error(),
			Terminal: true,
		}
	}

	result, err := d.executor.ExecuteStep(ctx, FromWireStepExecuteRequest(req))
	return ToWireStepExecuteResponse(result, err)
}

func isTransient(err error) bool {
	var tempErr interface{ Temporary() bool }
	if errors.As(err, &tempErr) && tempErr.Temporary() {
		return true
	}
	return strings.Contains(err.Error(), "connection refused")
}

func readAuthFile(containerName string) (string, error) {
	data, err := os.ReadFile(
		filepath.Join(authDir, containerName, stepplugincommon.AuthFilename),
	)
	if err != nil {
		return "", fmt.Errorf(
			"error reading StepPlugin auth file for container %q: %w",
			containerName,
			err,
		)
	}
	return string(data), nil
}
