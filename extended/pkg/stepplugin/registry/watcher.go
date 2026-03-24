package registry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
	"github.com/akuity/kargo/pkg/logging"
)

type Watcher struct {
	clientset kubernetes.Interface
	store     *Store
}

// NewWatcher returns a StepPlugin ConfigMap watcher backed by the provided
// REST config.
func NewWatcher(config *rest.Config) (*Watcher, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating StepPlugin watcher clientset: %w", err)
	}
	return newWatcher(clientset), nil
}

func newWatcher(clientset kubernetes.Interface) *Watcher {
	return &Watcher{
		clientset: clientset,
		store:     NewStore(),
	}
}

func (w *Watcher) NeedLeaderElection() bool {
	return true
}

// Store returns the in-memory StepPlugin store populated by this watcher.
func (w *Watcher) Store() *Store {
	return w.store
}

func (w *Watcher) Start(ctx context.Context) error {
	informer := corev1informer.NewFilteredConfigMapInformer(
		w.clientset,
		metav1.NamespaceAll,
		0,
		toolscache.Indexers{},
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf(
				"%s=%s",
				stepplugincommon.ConfigMapLabelKey,
				stepplugincommon.ConfigMapLabelValue,
			)
		},
	)

	logger := logging.LoggerFromContext(ctx)
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			w.handleUpsert(logger, obj)
		},
		UpdateFunc: func(_, obj any) {
			w.handleUpsert(logger, obj)
		},
		DeleteFunc: func(obj any) {
			w.handleDelete(logger, obj)
		},
	})
	if err != nil {
		return fmt.Errorf(
			"error adding StepPlugin ConfigMap informer handler: %w",
			err,
		)
	}

	go informer.Run(ctx.Done())
	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("error waiting for StepPlugin ConfigMap informer cache to sync")
	}
	w.store.SetSynced(true)

	<-ctx.Done()
	return nil
}

func (w *Watcher) handleUpsert(logger *logging.Logger, obj any) {
	cm, ok := extractConfigMap(obj)
	if !ok {
		logger.Error(fmt.Errorf("unexpected object type %T", obj), "failed to extract StepPlugin ConfigMap")
		return
	}

	plugin, err := argoplugin.FromConfigMap(cm)
	if err != nil {
		w.store.recordInvalid(cm.Namespace, cm.Name, err)
		logger.Error(
			err,
			"failed to parse StepPlugin ConfigMap",
			"namespace",
			cm.Namespace,
			"name",
			cm.Name,
		)
		return
	}

	w.store.Upsert(plugin)
}

func (w *Watcher) handleDelete(logger *logging.Logger, obj any) {
	cm, ok := extractConfigMap(obj)
	if !ok {
		logger.Error(fmt.Errorf("unexpected object type %T", obj), "failed to extract deleted StepPlugin ConfigMap")
		return
	}
	w.store.deleteByConfigMap(cm.Namespace, cm.Name)
}

func extractConfigMap(obj any) (*corev1.ConfigMap, bool) {
	if cm, ok := obj.(*corev1.ConfigMap); ok {
		return cm, true
	}
	tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}
	cm, ok := tombstone.Obj.(*corev1.ConfigMap)
	return cm, ok
}

var _ manager.Runnable = (*Watcher)(nil)
var _ manager.LeaderElectionRunnable = (*Watcher)(nil)
