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
