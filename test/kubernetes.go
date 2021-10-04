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

package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/orlangure/gnomock"
	"github.com/stretchr/testify/require"

	prek3s "github.com/orlangure/gnomock/preset/k3s"
	"k8s.io/client-go/kubernetes"
)

func WithK3sContainer(exec func(container *gnomock.Container)) {

	k3sVersion, ok := os.LookupEnv("K3S_VERSION")
	if !ok {
		k3sVersion = "v1.19.11"
	}

	var err error
	k3sContainer, err := gnomock.Start(prek3s.Preset(prek3s.WithVersion(k3sVersion)))
	if err != nil {
		panic(fmt.Sprintf("error creating k3s docker instance: %s\n", err))
	}

	exec(k3sContainer)

	_ = gnomock.Stop(k3sContainer)
}

func GetKubernetesClientset(t *testing.T, container *gnomock.Container) kubernetes.Interface {
	kubeconfig, err := prek3s.Config(container)
	require.NoError(t, err)

	client, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)

	return client
}
