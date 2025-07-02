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

package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/ports"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	testingk8s "k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesSchedulerStorage_CreateScheduler(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	scheduler := &entities.Scheduler{
		Name:  "test-scheduler",
		Game:  "test-game",
		State: entities.StateCreating,
		Spec: game_room.Spec{
			Version: "v1.0.0",
		},
		MaxSurge:      "10%",
		RoomsReplicas: 3,
		PortRange: &port.PortRange{
			Start: 10000,
			End:   20000,
		},
		MatchAllocation: &allocation.MatchAllocation{
			MaxMatches: 1,
		},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	assert.NoError(t, err)

	// Verify scheduler was created
	created, err := storage.GetScheduler(ctx, "test-scheduler")
	assert.NoError(t, err)
	assert.Equal(t, scheduler.Name, created.Name)
	assert.Equal(t, scheduler.Game, created.Game)
	assert.Equal(t, scheduler.Spec.Version, created.Spec.Version)
}

func TestKubernetesSchedulerStorage_UpdateScheduler(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create initial scheduler
	scheduler := &entities.Scheduler{
		Name:  "test-scheduler",
		Game:  "test-game",
		State: entities.StateCreating,
		Spec: game_room.Spec{
			Version: "v1.0.0",
		},
		MaxSurge:        "10%",
		RoomsReplicas:   3,
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Update scheduler
	scheduler.Spec.Version = "v2.0.0"
	scheduler.RoomsReplicas = 5
	scheduler.State = entities.StateInSync

	err = storage.UpdateScheduler(ctx, scheduler)
	assert.NoError(t, err)

	// Verify update
	updated, err := storage.GetScheduler(ctx, "test-scheduler")
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", updated.Spec.Version)
	assert.Equal(t, 5, updated.RoomsReplicas)
	assert.Equal(t, entities.StateInSync, updated.State)
}

func TestKubernetesSchedulerStorage_GetSchedulerVersions(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create scheduler with version
	scheduler := &entities.Scheduler{
		Name:  "test-scheduler",
		Game:  "test-game",
		State: entities.StateCreating,
		Spec: game_room.Spec{
			Version: "v1.0.0",
		},
		MaxSurge:        "10%",
		RoomsReplicas:   3,
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Update to create a new version
	scheduler.Spec.Version = "v2.0.0"
	err = storage.UpdateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Get versions
	versions, err := storage.GetSchedulerVersions(ctx, "test-scheduler")
	assert.NoError(t, err)
	assert.Len(t, versions, 2)
	
	// Check active version
	assert.Equal(t, "v2.0.0", versions[0].Version)
	assert.True(t, versions[0].IsActive)
	
	// Check previous version
	assert.Equal(t, "v1.0.0", versions[1].Version)
	assert.False(t, versions[1].IsActive)
}

func TestKubernetesSchedulerStorage_DeleteScheduler(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create scheduler
	scheduler := &entities.Scheduler{
		Name:            "test-scheduler",
		Game:            "test-game",
		State:           entities.StateCreating,
		Spec:            game_room.Spec{Version: "v1.0.0"},
		MaxSurge:        "10%",
		RoomsReplicas:   3,
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Delete scheduler
	err = storage.DeleteScheduler(ctx, "", scheduler)
	assert.NoError(t, err)

	// Verify deletion
	_, err = storage.GetScheduler(ctx, "test-scheduler")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestKubernetesSchedulerStorage_GetSchedulersWithFilter(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create multiple schedulers
	schedulers := []*entities.Scheduler{
		{
			Name:            "scheduler-1",
			Game:            "game-a",
			State:           entities.StateInSync,
			Spec:            game_room.Spec{Version: "v1.0.0"},
			MaxSurge:        "10%",
			RoomsReplicas:   3,
			MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
		},
		{
			Name:            "scheduler-2",
			Game:            "game-b",
			State:           entities.StateInSync,
			Spec:            game_room.Spec{Version: "v2.0.0"},
			MaxSurge:        "10%",
			RoomsReplicas:   5,
			MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
		},
		{
			Name:            "scheduler-3",
			Game:            "game-a",
			State:           entities.StateCreating,
			Spec:            game_room.Spec{Version: "v1.0.0"},
			MaxSurge:        "10%",
			RoomsReplicas:   2,
			MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
		},
	}

	for _, s := range schedulers {
		err := storage.CreateScheduler(ctx, s)
		require.NoError(t, err)
	}

	// Test filter by game
	filter := &filters.SchedulerFilter{Game: "game-a"}
	result, err := storage.GetSchedulersWithFilter(ctx, filter)
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// Test filter by name
	filter = &filters.SchedulerFilter{Name: "scheduler-2"}
	result, err = storage.GetSchedulersWithFilter(ctx, filter)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "scheduler-2", result[0].Name)
}

func TestKubernetesSchedulerStorage_Transaction(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create scheduler
	scheduler := &entities.Scheduler{
		Name:            "test-scheduler",
		Game:            "test-game",
		State:           entities.StateCreating,
		Spec:            game_room.Spec{Version: "v1.0.0"},
		MaxSurge:        "10%",
		RoomsReplicas:   3,
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Test successful transaction
	err = storage.RunWithTransaction(ctx, func(txID ports.TransactionID) error {
		return storage.DeleteScheduler(ctx, txID, scheduler)
	})
	assert.NoError(t, err)

	// Verify deletion was committed
	_, err = storage.GetScheduler(ctx, "test-scheduler")
	assert.Error(t, err)
}

func TestKubernetesSchedulerStorage_GetSchedulerWithFilter_Version(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	kubeClient := testingk8s.NewSimpleClientset()
	storage := NewKubernetesSchedulerStorage(dynamicClient, kubeClient, "test-namespace")

	// Create scheduler
	scheduler := &entities.Scheduler{
		Name:            "test-scheduler",
		Game:            "test-game",
		State:           entities.StateCreating,
		Spec:            game_room.Spec{Version: "v1.0.0"},
		MaxSurge:        "10%",
		RoomsReplicas:   3,
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	err := storage.CreateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Update to new version
	scheduler.Spec.Version = "v2.0.0"
	err = storage.UpdateScheduler(ctx, scheduler)
	require.NoError(t, err)

	// Get current version
	filter := &filters.SchedulerFilter{
		Name:    "test-scheduler",
		Version: "v2.0.0",
	}
	result, err := storage.GetSchedulerWithFilter(ctx, filter)
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", result.Spec.Version)

	// Get previous version
	filter.Version = "v1.0.0"
	result, err = storage.GetSchedulerWithFilter(ctx, filter)
	assert.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Spec.Version)
}