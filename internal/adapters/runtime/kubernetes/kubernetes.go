package kubernetes

import (
	"github.com/topfreegames/maestro/internal/core/ports"
	kube "k8s.io/client-go/kubernetes"
)

var _ ports.Runtime = (*kubernetes)(nil)

type kubernetes struct {
	clientset kube.Interface
}

func New(clientset kube.Interface) *kubernetes {
	return &kubernetes{clientset}
}
