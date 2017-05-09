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

	"github.com/topfreegames/maestro/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

const serviceYaml = `
apiVersion: v1
kind: Service
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    name: {{.Name}}
spec:
  selector:
    app: {{.Name}}
  ports:
    {{range .Ports}}
    - protocol: {{.Protocol}}
      port: {{.ContainerPort}}
      targetPort: {{.ContainerPort}}
      name: "{{.Name}}"
    {{end}}
  type: NodePort
`

// Service represents a service
type Service struct {
	Name      string
	Namespace string
	Ports     []*Port
}

// NewService is the service constructor
func NewService(name, namespace string, ports []*Port) *Service {
	return &Service{
		Name:      name,
		Namespace: namespace,
		Ports:     ports,
	}
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

	decoder := api.Codecs.UniversalDecoder()
	obj, _, err := decoder.Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return nil, err
	}

	src := obj.(*api.Service)
	dst := &v1.Service{}

	err = api.Scheme.Convert(src, dst, 0)
	if err != nil {
		return nil, err
	}

	svc, err := clientset.CoreV1().Services(s.Namespace).Create(dst)
	if err != nil {
		return nil, errors.NewKubernetesError("create service error", err)
	}
	return svc, nil
}

// Delete deletes a service from kubernetes
func (s *Service) Delete(clientset kubernetes.Interface) error {
	err := clientset.CoreV1().Services(s.Namespace).Delete(s.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete service error", err)
	}

	return nil
}
