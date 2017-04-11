// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// Namespace represents a namespace
type Namespace struct {
	Name string
}

// NewNamespace is the namespace constructor
func NewNamespace(name string) *Namespace {
	return &Namespace{
		Name: name,
	}
}

// Create creates a namespace in Kubernetes
func (n *Namespace) Create(clientset kubernetes.Interface) error {
	namespace := &v1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: n.Name,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(namespace)
	return err
}

// Exists returns true if namespace is already created in Kubernetes
func (n *Namespace) Exists(clientset kubernetes.Interface) bool {
	_, err := clientset.CoreV1().Namespaces().Get(n.Name)
	return err == nil
}
