//+build integration

package kubernetes

import (
	"fmt"
	"os"
	"testing"
	"time"

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
		k3s.Preset(k3s.WithVersion("v1.18.19")),
	)
	if err != nil {
		panic(fmt.Sprintf("error creating kubernetes docker instance: %s\n", err))
	}

	code := m.Run()

	_ = gnomock.Stop(kubernetesContainer)
	os.Exit(code)
}
