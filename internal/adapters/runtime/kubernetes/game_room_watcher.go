package kubernetes

import (
	"context"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var (
	defaultResyncTime = 30 * time.Second
	eventsChanSize    = 100
)

type kubernetesWatcher struct {
	resultsChan chan game_room.InstanceEvent
	stopChan    chan struct{}
	stopOnce    sync.Once
}

func (kw *kubernetesWatcher) ResultChan() chan game_room.InstanceEvent {
	return kw.resultsChan
}

func (kw *kubernetesWatcher) Stop() {
	kw.stopOnce.Do(func() {
		close(kw.stopChan)
	})
}

func (kw *kubernetesWatcher) processEvent(eventType game_room.InstanceEventType, obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	kw.resultsChan <- game_room.InstanceEvent{
		Type:     eventType,
		Instance: convertPod(pod),
	}
}

func (kw *kubernetesWatcher) addFunc(obj interface{}) {
	kw.processEvent(game_room.InstanceEventTypeAdded, obj)
}

func (kw *kubernetesWatcher) updateFunc(obj interface{}, newObj interface{}) {
	kw.processEvent(game_room.InstanceEventTypeUpdated, newObj)
}

func (kw *kubernetesWatcher) deleteFunc(obj interface{}) {
	kw.processEvent(game_room.InstanceEventTypeDeleted, obj)
}

func (k *kubernetes) WatchGameRoomInstances(ctx context.Context, scheduler *entities.Scheduler) (ports.RuntimeWatcher, error) {
	watcher := &kubernetesWatcher{
		resultsChan: make(chan game_room.InstanceEvent, eventsChanSize),
		stopChan:    make(chan struct{}),
	}

	podsInformer := informers.NewSharedInformerFactoryWithOptions(
		k.clientset,
		defaultResyncTime,
		informers.WithNamespace(scheduler.Name),
	).Core().V1().Pods().Informer()

	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.addFunc,
		UpdateFunc: watcher.updateFunc,
		DeleteFunc: watcher.deleteFunc,
	})

	go podsInformer.Run(watcher.stopChan)
	return watcher, nil
}
