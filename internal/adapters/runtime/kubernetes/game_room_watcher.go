// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package kubernetes

import (
	"context"
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/topfreegames/maestro/internal/core/logs"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	defaultResyncTime = 30 * time.Second
	eventsChanSize    = 2000
)

type kubernetesWatcher struct {
	mu               sync.Mutex
	clientSet        kube.Interface
	ctx              context.Context
	resultsChan      chan game_room.InstanceEvent
	err              error
	stopped          bool
	stopChan         chan struct{}
	instanceCacheMap map[string]*game_room.Instance
	logger           *zap.Logger
	queue            workqueue.RateLimitingInterface
	store            cache.Store
	schedulerName    string
}

type QueueItem struct {
	obj       interface{}
	eventType game_room.InstanceEventType
}

func NewKubernetesWatcher(ctx context.Context, clientSet kube.Interface, schedulerName string) *kubernetesWatcher {
	return &kubernetesWatcher{
		clientSet:        clientSet,
		ctx:              ctx,
		resultsChan:      make(chan game_room.InstanceEvent, eventsChanSize),
		stopChan:         make(chan struct{}),
		instanceCacheMap: map[string]*game_room.Instance{},
		logger:           zap.L(),
		queue:            workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		store: cache.NewStore(func(obj interface{}) (string, error) {
			item := obj.(*QueueItem)
			return cache.MetaNamespaceKeyFunc(item.obj)
		}),
		schedulerName: schedulerName,
	}
}

func (kw *kubernetesWatcher) ResultChan() chan game_room.InstanceEvent {
	kw.mu.Lock()
	defer kw.mu.Unlock()

	return kw.resultsChan
}

func (kw *kubernetesWatcher) Stop() {
	kw.mu.Lock()
	defer kw.mu.Unlock()

	kw.stopWithError(nil)
}

func (kw *kubernetesWatcher) Err() error {
	kw.mu.Lock()
	defer kw.mu.Unlock()

	return kw.err
}

func (kw *kubernetesWatcher) stopWithError(err error) {
	if kw.stopped {
		return
	}
	kw.logger.Error("Watcher stopped with error", zap.Error(err))

	kw.stopped = true
	kw.err = err
	close(kw.resultsChan)
	close(kw.stopChan)
}

func (kw *kubernetesWatcher) convertInstance(pod *v1.Pod) (*game_room.Instance, error) {
	if pod.Spec.NodeName == "" {
		return convertPod(pod, nil)
	}

	node, err := kw.clientSet.CoreV1().Nodes().Get(kw.ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return convertPod(pod, node)
}

func (kw *kubernetesWatcher) processEvent(eventType game_room.InstanceEventType, obj interface{}) {
	if kw.stopped {
		return
	}

	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	var instance *game_room.Instance
	var err error

	instance, exists := kw.instanceCacheMap[pod.Name]
	if !exists {
		instance, err = kw.convertInstance(pod)
		if err != nil {
			kw.logger.Error("Error converting pod to game room instance", zap.String(logs.LogFieldInstanceID, pod.ObjectMeta.Name), zap.Error(err))
			reportInstanceConversionFailed(kw.schedulerName)
			return
		}
		// Caching before ready consists in lack of information about the instance.
		if instance.Status.Type == game_room.InstanceReady {
			kw.instanceCacheMap[pod.Name] = instance
		}
	} else {
		tempInstance := &game_room.Instance{
			ID:              instance.ID,
			SchedulerID:     instance.SchedulerID,
			Status:          convertPodStatus(pod),
			Address:         instance.Address,
			ResourceVersion: pod.ResourceVersion,
		}
		instance = tempInstance

		kw.instanceCacheMap[pod.Name] = instance
	}

	if eventType == game_room.InstanceEventTypeDeleted {
		delete(kw.instanceCacheMap, pod.Name)
	}

	instanceEvent := game_room.InstanceEvent{
		Instance: instance,
		Type:     eventType,
	}

	kw.resultsChan <- instanceEvent
}

func (kw *kubernetesWatcher) addFunc(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	_ = kw.store.Add(&QueueItem{
		obj:       obj,
		eventType: game_room.InstanceEventTypeAdded,
	})
	kw.queue.Add(key)
}

func (kw *kubernetesWatcher) updateFunc(obj interface{}, newObj interface{}) {
	oldPod := obj.(*v1.Pod)
	newPod := newObj.(*v1.Pod)
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		return
	}

	_ = kw.store.Add(&QueueItem{
		obj:       newObj,
		eventType: game_room.InstanceEventTypeUpdated,
	})
	kw.queue.Add(key)

}

func (kw *kubernetesWatcher) deleteFunc(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	_ = kw.store.Add(&QueueItem{
		obj:       obj,
		eventType: game_room.InstanceEventTypeDeleted,
	})
	kw.queue.Add(key)
}

func (k *kubernetes) WatchGameRoomInstances(ctx context.Context, scheduler *entities.Scheduler) (ports.RuntimeWatcher, error) {
	watcher := NewKubernetesWatcher(ctx, k.clientSet, scheduler.Name)

	podsInformer := informers.NewSharedInformerFactoryWithOptions(
		k.clientSet,
		defaultResyncTime,
		informers.WithNamespace(scheduler.Name),
	).Core().V1().Pods().Informer()

	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.addFunc,
		UpdateFunc: watcher.updateFunc,
		DeleteFunc: watcher.deleteFunc,
	})

	go podsInformer.Run(watcher.stopChan)

	// Same amount of routines reading from the channel
	for i := 0; i < 200; i++ {
		go watcher.runWorker()
	}
	return watcher, nil
}

func (kw *kubernetesWatcher) runWorker() {
	for {
		key, quit := kw.queue.Get()
		if quit {
			return
		}

		item, _, _ := kw.store.GetByKey(key.(string))

		convertedItem := item.(*QueueItem)
		kw.processEvent(convertedItem.eventType, convertedItem.obj)
		kw.queue.Forget(key)

		kw.queue.Done(key)
	}
}
