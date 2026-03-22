package scheduler

import (
	"context"
	"sync"

	"agentd/control"
	"agentd/store"
)

type RoundRobin struct {
	Containers store.ContainerStore
	Nodes      []string

	mu   sync.Mutex
	next int
}

func (s *RoundRobin) SelectPlacement(ctx context.Context, plan control.ExecutionPlan) (control.Placement, error) {
	if plan.Placement.ContainerID != "" {
		return plan.Placement, nil
	}
	containers, err := s.Containers.ListContainers(ctx)
	if err != nil {
		return control.Placement{}, err
	}
	for _, container := range containers {
		if container.State != control.ContainerReady && container.State != control.ContainerIdle {
			continue
		}
		if container.CurrentRuns >= container.Capacity && container.Capacity > 0 {
			continue
		}
		if plan.ContainerProfile.NodeID != "" && container.NodeID != plan.ContainerProfile.NodeID {
			continue
		}
		return control.Placement{ContainerID: container.ID, NodeID: container.NodeID, SchedulerChosen: true}, nil
	}
	if plan.Placement.NodeID != "" {
		return control.Placement{NodeID: plan.Placement.NodeID, SchedulerChosen: true, CreateIfNotFound: true}, nil
	}
	if len(s.Nodes) == 0 {
		return control.Placement{}, control.ErrNoCapacity
	}
	s.mu.Lock()
	node := s.Nodes[s.next%len(s.Nodes)]
	s.next++
	s.mu.Unlock()
	return control.Placement{NodeID: node, SchedulerChosen: true, CreateIfNotFound: true}, nil
}
