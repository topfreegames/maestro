package kubernetes

import (
	"github.com/topfreegames/maestro/internal/services/runtime"
	kube "k8s.io/client-go/kubernetes"
)

var _ runtime.Runtime = (*kubernetes)(nil)

type kubernetes struct {
	clientset kube.Interface
}

func New(clientset kube.Interface) *kubernetes {
	return &kubernetes{clientset}
}
