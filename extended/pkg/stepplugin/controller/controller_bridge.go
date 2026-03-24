package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuity/kargo/extended/pkg/stepplugin"
	"github.com/akuity/kargo/pkg/credentials"
	kargoos "github.com/akuity/kargo/pkg/os"
	"github.com/akuity/kargo/pkg/promotion"
	"github.com/akuity/kargo/pkg/types"
)

func NewPromotionEngine(
	kargoClient client.Client,
	argoCDClient client.Client,
	credsDB credentials.Database,
	cacheFunc promotion.ExprDataCacheFn,
) promotion.Engine {
	return stepplugin.NewEngine(
		kargoClient,
		argoCDClient,
		credsDB,
		cacheFunc,
		kargoos.GetEnv("SYSTEM_RESOURCES_NAMESPACE", "kargo-system-resources"),
		types.MustParseBool(kargoos.GetEnv("STEP_PLUGINS_ENABLED", "false")),
	)
}
