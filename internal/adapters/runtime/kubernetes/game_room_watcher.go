package kubernetes

import (
	"context"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/runtime"
	"github.com/topfreegames/maestro/internal/core/entities"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var (
	defaultResyncTime = 30 * time.Second
	eventsChanSize    = 100
)

type kubernetesWatcher struct {
	resultsChan chan runtime.RuntimeGameInstanceEvent
	stopChan    chan struct{}
	stopOnce    sync.Once
}

func (kw *kubernetesWatcher) ResultChan() chan runtime.RuntimeGameInstanceEvent {
	return kw.resultsChan
}

func (kw *kubernetesWatcher) Stop() {
	kw.stopOnce.Do(func() {
		close(kw.stopChan)
	})
}

func (kw *kubernetesWatcher) processEvent(eventType runtime.RuntimeGameInstanceEventType, obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	kw.resultsChan <- runtime.RuntimeGameInstanceEvent{
		Type:     eventType,
		Instance: convertPod(pod),
	}
}

func (kw *kubernetesWatcher) addFunc(obj interface{}) {
	kw.processEvent(runtime.RuntimeGameInstanceEventTypeAdded, obj)
}

func (kw *kubernetesWatcher) updateFunc(obj interface{}, newObj interface{}) {
	kw.processEvent(runtime.RuntimeGameInstanceEventTypeUpdated, newObj)
}

func (kw *kubernetesWatcher) deleteFunc(obj interface{}) {
	kw.processEvent(runtime.RuntimeGameInstanceEventTypeDeleted, obj)
}

func (k *kubernetes) WatchGameRooms(ctx context.Context, scheduler *entities.Scheduler) (runtime.RuntimeWatcher, error) {
	watcher := &kubernetesWatcher{
		resultsChan: make(chan runtime.RuntimeGameInstanceEvent, eventsChanSize),
		stopChan:    make(chan struct{}),
	}

	podsInformer := informers.NewSharedInformerFactoryWithOptions(
		k.clientset,
		defaultResyncTime,
		informers.WithNamespace(scheduler.ID),
	).Core().V1().Pods().Informer()

	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.addFunc,
		UpdateFunc: watcher.updateFunc,
		DeleteFunc: watcher.deleteFunc,
	})

	go podsInformer.Run(watcher.stopChan)
	return watcher, nil
}
