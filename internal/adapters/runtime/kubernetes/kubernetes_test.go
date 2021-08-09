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

//+build integration

package kubernetes

import (
	"fmt"
	"os"
	"testing"

	kube "k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
)

var kubernetesContainer *gnomock.Container

func getKubernetesClientset(t *testing.T) kube.Interface {
	kubeconfig, err := k3s.Config(kubernetesContainer)
	require.NoError(t, err)

	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	return client
}

func TestMain(m *testing.M) {
	var err error
	kubernetesContainer, err = gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.19.11")),
	)
	if err != nil {
		panic(fmt.Sprintf("error creating kubernetes docker instance: %s\n", err))
	}

	code := m.Run()

	_ = gnomock.Stop(kubernetesContainer)
	os.Exit(code)
}
