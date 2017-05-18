// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/fields"
)

// GetServiceNames return n services that are currently running
func GetServiceNames(n int, namespace string, clientset kubernetes.Interface) ([]string, error) {
	services, err := clientset.CoreV1().Services(namespace).List(metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
	})
	if err != nil {
		return nil, err
	}
	serviceNames := []string{}
	for _, service := range services.Items {
		serviceNames = append(serviceNames, service.Name)
	}
	return serviceNames[:n], nil
}
