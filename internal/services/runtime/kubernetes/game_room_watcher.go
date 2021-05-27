package kubernetes

import (
	"context"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/entities"
	"github.com/topfreegames/maestro/internal/services/runtime"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var (
	defaultResyncTime = 30 * time.Second
	eventsChanSize    = 100
)

type kubernetesWatcher struct {
	resultsChan chan runtime.RuntimeEvent
	stopChan    chan struct{}
	stopOnce    sync.Once
}

func (kw *kubernetesWatcher) ResultChan() chan runtime.RuntimeEvent {
	return kw.resultsChan
}

func (kw *kubernetesWatcher) Stop() {
	kw.stopOnce.Do(func() {
		close(kw.stopChan)
	})
}

func (kw *kubernetesWatcher) processEvent(eventType runtime.RuntimeEventType, obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	kw.resultsChan <- runtime.RuntimeEvent{
		Type:       eventType,
		GameRoomID: pod.ObjectMeta.Name,
		Status:     convertPodStatus(pod),
	}
}

func (kw *kubernetesWatcher) addFunc(obj interface{}) {
	kw.processEvent(runtime.RuntimeEventAdded, obj)
}

func (kw *kubernetesWatcher) updateFunc(obj interface{}, newObj interface{}) {
	kw.processEvent(runtime.RuntimeEventUpdated, obj)
}

func (kw *kubernetesWatcher) deleteFunc(obj interface{}) {
	kw.processEvent(runtime.RuntimeEventDeleted, obj)
}

func (k *kubernetes) WatchGameRooms(ctx context.Context, scheduler *entities.Scheduler) (runtime.RuntimeWatcher, error) {
	watcher := &kubernetesWatcher{
		resultsChan: make(chan runtime.RuntimeEvent, eventsChanSize),
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
