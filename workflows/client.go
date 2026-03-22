package workflows

import (
	"context"
	"fmt"

	"agentd/control"
	"go.temporal.io/sdk/client"
)

type Client struct {
	Raw       client.Client
	TaskQueue string
}

func (c Client) StartContainer(ctx context.Context, req control.CreateContainerRequest) (control.Container, error) {
	id := req.ID
	if id == "" {
		id = control.NewID("ctr")
		req.ID = id
	}
	start := c.Raw.NewWithStartWorkflowOperation(client.StartWorkflowOptions{ID: ContainerWorkflowID(id), TaskQueue: c.taskQueue()}, ContainerWorkflow, ContainerWorkflowInput{ContainerID: id})
	update, err := c.Raw.UpdateWithStartWorkflow(ctx, client.UpdateWithStartWorkflowOptions{
		StartWorkflowOperation: start,
		UpdateOptions:          client.UpdateWorkflowOptions{WorkflowID: ContainerWorkflowID(id), UpdateName: UpdateStart, Args: []interface{}{req}, WaitForStage: client.WorkflowUpdateStageCompleted},
	})
	if err != nil {
		return control.Container{}, err
	}
	var container control.Container
	return container, update.Get(ctx, &container)
}

func (c Client) InstallAgent(ctx context.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
	var agent control.AgentInstance
	if err := c.update(ctx, ContainerWorkflowID(req.ContainerID), UpdateInstallAgent, &agent, req); err != nil {
		return control.AgentInstance{}, err
	}
	return agent, nil
}

func (c Client) CreateSession(ctx context.Context, req control.CreateSessionRequest) (control.Session, error) {
	var sess control.Session
	if err := c.update(ctx, ContainerWorkflowID(req.ContainerID), UpdateCreateSession, &sess, req); err != nil {
		return control.Session{}, err
	}
	return sess, nil
}

func (c Client) LoadSession(ctx context.Context, req control.LoadSessionRequest) (control.Session, error) {
	var sess control.Session
	if err := c.update(ctx, ContainerWorkflowID(req.ContainerID), UpdateLoadSession, &sess, req); err != nil {
		return control.Session{}, err
	}
	return sess, nil
}

func (c Client) Prompt(ctx context.Context, req control.PromptRequest) (control.Run, error) {
	var run control.Run
	if err := c.update(ctx, SessionWorkflowID(req.SessionID), UpdatePrompt, &run, req); err != nil {
		return control.Run{}, err
	}
	return run, nil
}

func (c Client) Cancel(ctx context.Context, req control.CancelRequest) error {
	return c.update(ctx, SessionWorkflowID(req.SessionID), UpdateCancel, nil, req)
}

func (c Client) HibernateContainer(ctx context.Context, containerID string) (control.Container, error) {
	var container control.Container
	if err := c.update(ctx, ContainerWorkflowID(containerID), UpdateHibernate, &container); err != nil {
		return control.Container{}, err
	}
	return container, nil
}

func (c Client) DeleteContainer(ctx context.Context, containerID string) error {
	return c.update(ctx, ContainerWorkflowID(containerID), UpdateDelete, nil)
}

func (c Client) SyncRegistry(ctx context.Context, source string) ([]control.AgentCatalogEntry, error) {
	run, err := c.Raw.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: control.NewID("registry-sync"), TaskQueue: c.taskQueue()}, RegistrySyncWorkflow, source)
	if err != nil {
		return nil, err
	}
	var entries []control.AgentCatalogEntry
	return entries, run.Get(ctx, &entries)
}

func (c Client) ResolveBundle(ctx context.Context, req BundleRequest) (control.Bundle, error) {
	run, err := c.Raw.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: control.NewID("bundle"), TaskQueue: c.taskQueue()}, BundleWorkflow, req)
	if err != nil {
		return control.Bundle{}, err
	}
	var bundle control.Bundle
	return bundle, run.Get(ctx, &bundle)
}

func (c Client) update(ctx context.Context, workflowID, updateName string, out any, args ...any) error {
	h, err := c.Raw.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{WorkflowID: workflowID, UpdateName: updateName, Args: args, WaitForStage: client.WorkflowUpdateStageCompleted})
	if err != nil {
		return err
	}
	if err := h.Get(ctx, out); err != nil {
		return fmt.Errorf("update %s on %s: %w", updateName, workflowID, err)
	}
	return nil
}

func (c Client) taskQueue() string {
	if c.TaskQueue == "" {
		return TaskQueueDefault
	}
	return c.TaskQueue
}
