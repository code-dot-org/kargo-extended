package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/extended/pkg/stepplugin/executor"
	argocd "github.com/akuity/kargo/pkg/controller/argocd/api/v1alpha1"
	"github.com/akuity/kargo/pkg/credentials"
	credsdb "github.com/akuity/kargo/pkg/credentials/kubernetes"
	kargoos "github.com/akuity/kargo/pkg/os"
	"github.com/akuity/kargo/pkg/server/kubernetes"
	"github.com/akuity/kargo/pkg/types"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "promotion-agent",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgentMain(cmd.Context())
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "init",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgentInit(cmd.Context())
		},
	})
	return cmd
}

func runAgentInit(context.Context) error {
	targets, err := pluginTargetsFromEnv()
	if err != nil {
		return err
	}

	containerNames := map[string]struct{}{}
	for _, target := range targets {
		containerNames[target.ContainerName] = struct{}{}
	}
	for containerName := range containerNames {
		filename := filepath.Join(common.AuthDir, containerName, common.AuthFilename)
		if err := os.MkdirAll(filepath.Dir(filename), 0o770); err != nil {
			return err
		}
		if err := os.WriteFile(filename, []byte(rand.String(32)), 0o400); err != nil {
			return err
		}
	}
	return nil
}

func runAgentMain(ctx context.Context) error {
	targets, err := pluginTargetsFromEnv()
	if err != nil {
		return err
	}

	kargoClient, err := newKargoClient(ctx)
	if err != nil {
		return err
	}
	argoCDClient, err := newArgoCDClient(ctx)
	if err != nil {
		return err
	}
	dispatcher, err := executor.NewDispatcher(
		kargoClient,
		argoCDClient,
		credsdb.NewDatabase(
			kargoClient,
			nil,
			credentials.DefaultProviderRegistry,
			credsdb.DatabaseConfigFromEnv(),
		),
		targets,
	)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(common.APIPathPrefix+common.MethodStepExecute, func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		defer r.Body.Close()

		var req executor.StepExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := dispatcher.ExecuteStep(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", common.AgentContainerPort),
		Handler:           mux,
		ReadHeaderTimeout: time.Duration(common.AgentReadHeaderTimeoutSec) * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func pluginTargetsFromEnv() (map[string]executor.PluginTarget, error) {
	raw := os.Getenv(common.EnvVarPluginTargets)
	if raw == "" {
		return map[string]executor.PluginTarget{}, nil
	}
	var targets map[string]executor.PluginTarget
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return nil, err
	}
	for kind, target := range targets {
		if kind == "" {
			return nil, fmt.Errorf("plugin target kind cannot be empty")
		}
		if target.Address == "" {
			return nil, fmt.Errorf("plugin target %q has empty address", kind)
		}
		if target.ContainerName == "" {
			return nil, fmt.Errorf("plugin target %q has empty container name", kind)
		}
	}
	return targets, nil
}

func newKargoClient(ctx context.Context) (ctrlclient.Client, error) {
	restCfg, err := kubernetes.GetRestConfig(ctx, os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, fmt.Errorf("error loading control plane REST config: %w", err)
	}
	kubernetes.ConfigureQPSBurst(
		ctx,
		restCfg,
		types.MustParseFloat32(kargoos.GetEnv("KUBE_API_QPS", "50.0")),
		types.MustParseInt(kargoos.GetEnv("KUBE_API_BURST", "300")),
	)

	scheme := runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = kargoapi.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewClient(
		ctx,
		restCfg,
		kubernetes.ClientOptions{
			SkipAuthorization: true,
			Scheme:            scheme,
		},
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newArgoCDClient(ctx context.Context) (ctrlclient.Client, error) {
	if !types.MustParseBool(kargoos.GetEnv("ARGOCD_INTEGRATION_ENABLED", "true")) {
		return nil, nil
	}

	kubeconfig := os.Getenv("ARGOCD_KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	restCfg, err := kubernetes.GetRestConfig(ctx, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error loading Argo CD REST config: %w", err)
	}
	kubernetes.ConfigureQPSBurst(
		ctx,
		restCfg,
		types.MustParseFloat32(kargoos.GetEnv("KUBE_API_QPS", "50.0")),
		types.MustParseInt(kargoos.GetEnv("KUBE_API_BURST", "300")),
	)

	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		argocd.AddToScheme,
	} {
		schemeErr := addToScheme(scheme)
		if schemeErr != nil {
			return nil, schemeErr
		}
	}
	client, err := ctrlclient.New(restCfg, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return client, nil
}
