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

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	defaultResyncTime = 30 * time.Second
	eventsChanSize    = 100
)

type kubernetesWatcher struct {
	mu          sync.Mutex
	clientSet   kube.Interface
	ctx         context.Context
	resultsChan chan game_room.InstanceEvent
	err         error
	stopped     bool
	stopChan    chan struct{}
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
	kw.mu.Lock()
	defer kw.mu.Unlock()

	if kw.stopped {
		return
	}

	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	instance, err := kw.convertInstance(pod)
	if err != nil {
		kw.stopWithError(err)
		return
	}
	zap.L().Info("Received kubernetes event on runtime watcher", zap.Any("convertedPod", pod), zap.Any("convertedInstance", instance), zap.Any("obj", obj))
	kw.resultsChan <- game_room.InstanceEvent{
		Type:     eventType,
		Instance: instance,
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
		clientSet:   k.clientSet,
		ctx:         ctx,
		resultsChan: make(chan game_room.InstanceEvent, eventsChanSize),
		stopChan:    make(chan struct{}),
	}

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
	return watcher, nil
}
