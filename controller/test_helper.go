// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

// GetPodNames return n pods that are currently running
func GetPodNames(n int, namespace string, clientset kubernetes.Interface) ([]string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
	})
	if err != nil {
		return nil, err
	}
	podNames := []string{}
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.GetName())
	}
	return podNames[:n], nil
}
