package controller

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/akuity/kargo/extended/pkg/stepplugin"
	"github.com/akuity/kargo/extended/pkg/stepplugin/registry"
	"github.com/akuity/kargo/pkg/credentials"
	kargoos "github.com/akuity/kargo/pkg/os"
	"github.com/akuity/kargo/pkg/promotion"
	"github.com/akuity/kargo/pkg/types"
)

type discoveryWatcher interface {
	manager.Runnable
	manager.LeaderElectionRunnable
	Store() *registry.Store
}

type promotionEngineManager interface {
	Add(manager.Runnable) error
	GetAPIReader() client.Reader
	GetClient() client.Client
	GetConfig() *rest.Config
}

var newWatcherFn = func(config *rest.Config) (discoveryWatcher, error) {
	return registry.NewWatcher(config)
}

func NewPromotionEngine(
	mgr promotionEngineManager,
	argoCDClient client.Client,
	credsDB credentials.Database,
	cacheFunc promotion.ExprDataCacheFn,
) (promotion.Engine, error) {
	systemResourcesNamespace := kargoos.GetEnv(
		"SYSTEM_RESOURCES_NAMESPACE",
		"kargo-system-resources",
	)
	enabled := types.MustParseBool(kargoos.GetEnv("STEP_PLUGINS_ENABLED", "true"))
	if !enabled {
		return stepplugin.NewEngine(
			mgr.GetClient(),
			mgr.GetAPIReader(),
			argoCDClient,
			credsDB,
			cacheFunc,
			systemResourcesNamespace,
			false,
		), nil
	}

	watcher, err := newWatcherFn(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	if err := mgr.Add(watcher); err != nil {
		return nil, err
	}

	resolver := registry.NewWatchedResolver(
		watcher.Store(),
		promotion.DefaultStepRunnerRegistry,
		systemResourcesNamespace,
		true,
	)
	return stepplugin.NewEngineWithResolver(
		mgr.GetClient(),
		mgr.GetAPIReader(),
		argoCDClient,
		credsDB,
		cacheFunc,
		resolver,
	), nil
}
