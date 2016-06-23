package orchestrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestReplicatedOrchestrator(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := New(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		s1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, s1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Create a second service.
	err = s.Update(func(tx store.Tx) error {
		s2 := &api.Service{
			ID: "id2",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name2",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 1,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, s2))
		return nil
	})
	assert.NoError(t, err)

	observedTask3 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name2")

	// Update a service to scale it out to 3 instances
	err = s.Update(func(tx store.Tx) error {
		s2 := &api.Service{
			ID: "id2",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name2",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 3,
					},
				},
			},
		}
		assert.NoError(t, store.UpdateService(tx, s2))
		return nil
	})
	assert.NoError(t, err)

	observedTask4 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask4.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name2")

	observedTask5 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask5.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask5.ServiceAnnotations.Name, "name2")

	// Now scale it back down to 1 instance
	err = s.Update(func(tx store.Tx) error {
		s2 := &api.Service{
			ID: "id2",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name2",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 1,
					},
				},
			},
		}
		assert.NoError(t, store.UpdateService(tx, s2))
		return nil
	})
	assert.NoError(t, err)

	observedDeletion1 := watchShutdownTask(t, watch)
	assert.Equal(t, observedDeletion1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedDeletion1.ServiceAnnotations.Name, "name2")

	observedDeletion2 := watchShutdownTask(t, watch)
	assert.Equal(t, observedDeletion2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedDeletion2.ServiceAnnotations.Name, "name2")

	// There should be one remaining task attached to service id2/name2.
	var liveTasks []*api.Task
	s.View(func(readTx store.ReadTx) {
		var tasks []*api.Task
		tasks, err = store.FindTasks(readTx, store.ByServiceID("id2"))
		for _, t := range tasks {
			if t.DesiredState == api.TaskStateRunning {
				liveTasks = append(liveTasks, t)
			}
		}
	})
	assert.NoError(t, err)
	assert.Len(t, liveTasks, 1)

	// Delete the remaining task directly. It should be recreated by the
	// orchestrator.
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.DeleteTask(tx, liveTasks[0].ID))
		return nil
	})
	assert.NoError(t, err)

	observedTask6 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask6.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask6.ServiceAnnotations.Name, "name2")

	// Delete the service. Its remaining task should go away.
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.DeleteService(tx, "id2"))
		return nil
	})
	assert.NoError(t, err)

	deletedTask := watchTaskDelete(t, watch)
	assert.Equal(t, deletedTask.Status.State, api.TaskStateNew)
	assert.Equal(t, deletedTask.ServiceAnnotations.Name, "name2")
}

func watchTaskCreate(t *testing.T, watch chan events.Event) *api.Task {
	for {
		select {
		case event := <-watch:
			if task, ok := event.(state.EventCreateTask); ok {
				return task.Task
			}
			if _, ok := event.(state.EventUpdateTask); ok {
				assert.FailNow(t, "got EventUpdateTask when expecting EventCreateTask", fmt.Sprint(event))
			}
		case <-time.After(time.Second):
			assert.FailNow(t, "no task creation")
		}
	}
}

func watchTaskUpdate(t *testing.T, watch chan events.Event) *api.Task {
	for {
		select {
		case event := <-watch:
			if task, ok := event.(state.EventUpdateTask); ok {
				return task.Task
			}
			if _, ok := event.(state.EventCreateTask); ok {
				assert.FailNow(t, "got EventCreateTask when expecting EventUpdateTask", fmt.Sprint(event))
			}
		case <-time.After(time.Second):
			assert.FailNow(t, "no task update")
		}
	}
}

func watchTaskDelete(t *testing.T, watch chan events.Event) *api.Task {
	for {
		select {
		case event := <-watch:
			if task, ok := event.(state.EventDeleteTask); ok {
				return task.Task
			}
		case <-time.After(time.Second):
			assert.FailNow(t, "no task deletion")
		}
	}
}

func expectTaskUpdate(t *testing.T, watch chan events.Event) {
	for {
		select {
		case event := <-watch:
			if _, ok := event.(state.EventUpdateTask); !ok {
				assert.FailNow(t, "expected task update event, got", fmt.Sprint(event))
			}
			return
		case <-time.After(time.Second):
			assert.FailNow(t, "no task update event")
		}
	}
}

func expectDeleteService(t *testing.T, watch chan events.Event) {
	for {
		select {
		case event := <-watch:
			if _, ok := event.(state.EventDeleteService); !ok {
				assert.FailNow(t, "expected service delete event, got", fmt.Sprint(event))
			}
			return
		case <-time.After(time.Second):
			assert.FailNow(t, "no service delete event")
		}
	}
}

func expectCommit(t *testing.T, watch chan events.Event) {
	for {
		select {
		case event := <-watch:
			if _, ok := event.(state.EventCommit); !ok {
				assert.FailNow(t, "expected commit event, got", fmt.Sprint(event))
			}
			return
		case <-time.After(time.Second):
			assert.FailNow(t, "no commit event")
		}
	}

}

func watchShutdownTask(t *testing.T, watch chan events.Event) *api.Task {
	for {
		select {
		case event := <-watch:
			if task, ok := event.(state.EventUpdateTask); ok && task.Task.DesiredState == api.TaskStateShutdown {
				return task.Task
			}
			if _, ok := event.(state.EventCreateTask); ok {
				assert.FailNow(t, "got EventCreateTask when expecting EventUpdateTask", fmt.Sprint(event))
			}
		case <-time.After(time.Second):
			assert.FailNow(t, "no task deletion")
		}
	}
}

func skipEvents(t *testing.T, watch chan events.Event) {
	for {
		select {
		case <-watch:
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}
