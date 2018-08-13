// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"bytes"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/topfreegames/maestro/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

// TODO: setup livenessProbe
const serviceYaml = `
apiVersion: v1
kind: Service
metadata:
  labels:
    name: {{.Name}}
  name: {{.Name}}
  namespace: {{.Namespace}}
spec:
  ports:
    - port: {{.Port.ContainerPort}}
      targetPort: {{.Port.ContainerPort}}
      name: {{.Port.Name}}
      protocol: {{.Port.Protocol}}
  selector:
    app: {{.Name}}
  type: NodePort
`

// Service represents a service
type Service struct {
	Name      string
	Namespace string
	Port      *Port
}

// NewService is the service constructor
func NewService(
	name string,
	configYaml *ConfigYAML,
) *Service {
	service := &Service{
		Name:      name,
		Namespace: configYaml.Name,
		Port:      configYaml.Ports[0],
	}

	return service
}

// Create creates a service in Kubernetes
func (s *Service) Create(clientset kubernetes.Interface) (*v1.Service, error) {
	tmpl, err := template.New("create").Parse(serviceYaml)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, s)
	if err != nil {
		return nil, err
	}

	k8sService := v1.Service{}
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(buf.Bytes()), len(buf.Bytes())).Decode(&k8sService)
	if err != nil {
		return nil, errors.NewKubernetesError("error unmarshaling pod", err)
	}

	service, err := clientset.CoreV1().Services(s.Namespace).Create(&k8sService)
	if err != nil {
		return nil, errors.NewKubernetesError("create pod error", err)
	}

	return service, nil
}

// Delete deletes a pod from kubernetes.
func (s *Service) Delete(clientset kubernetes.Interface,
	reason string,
	configYaml *ConfigYAML,
) error {
	err := clientset.CoreV1().Services(s.Namespace).Delete(s.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete service error", err)
	}
	return nil
}
