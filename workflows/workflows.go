package workflows

import (
	"time"

	"agentd/control"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ContainerWorkflowInput struct {
	ContainerID string `json:"container_id"`
}

type SessionWorkflowInput struct {
	Session control.Session `json:"session"`
}

type containerState struct {
	Container control.Container                `json:"container"`
	Agents    map[string]control.AgentInstance `json:"agents"`
	Sessions  map[string]control.Session       `json:"sessions"`
	Deleted   bool                             `json:"deleted"`
}

type sessionState struct {
	Session control.Session `json:"session"`
	LastRun control.Run     `json:"last_run"`
}

func RegistrySyncWorkflow(ctx workflow.Context, source string) ([]control.AgentCatalogEntry, error) {
	ctx = withActivityOptions(ctx)
	var entries []control.AgentCatalogEntry
	if err := workflow.ExecuteActivity(ctx, ActivityRegistrySync, source).Get(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func BundleWorkflow(ctx workflow.Context, req BundleRequest) (control.Bundle, error) {
	ctx = withActivityOptions(ctx)
	var bundle control.Bundle
	if err := workflow.ExecuteActivity(ctx, ActivityResolveBundle, req).Get(ctx, &bundle); err != nil {
		return control.Bundle{}, err
	}
	return bundle, nil
}

func ContainerWorkflow(ctx workflow.Context, input ContainerWorkflowInput) (control.Container, error) {
	ctx = withActivityOptions(ctx)
	state := containerState{Container: control.Container{ID: input.ContainerID, State: control.ContainerRequested}, Agents: map[string]control.AgentInstance{}, Sessions: map[string]control.Session{}}
	if err := workflow.SetQueryHandler(ctx, QueryGetState, func() (containerState, error) { return state, nil }); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetQueryHandler(ctx, QueryListAgents, func() ([]control.AgentInstance, error) { return mapsValues(state.Agents), nil }); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetQueryHandler(ctx, QueryListSessions, func() ([]control.Session, error) { return mapsValues(state.Sessions), nil }); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateStart, func(ctx workflow.Context, req control.CreateContainerRequest) (control.Container, error) {
		ctx = withActivityOptions(ctx)
		if req.ID == "" {
			req.ID = input.ContainerID
		}
		var container control.Container
		if err := workflow.ExecuteActivity(ctx, ActivityProvisionContainer, req).Get(ctx, &container); err != nil {
			return control.Container{}, err
		}
		state.Container = container
		return container, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateInstallAgent, func(ctx workflow.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
		ctx = withActivityOptions(ctx)
		var agent control.AgentInstance
		if err := workflow.ExecuteActivity(ctx, ActivityEnsureAgent, req).Get(ctx, &agent); err != nil {
			return control.AgentInstance{}, err
		}
		state.Agents[agent.ID] = agent
		return agent, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateStartAgent, func(ctx workflow.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
		ctx = withActivityOptions(ctx)
		var agent control.AgentInstance
		if err := workflow.ExecuteActivity(ctx, ActivityEnsureAgent, req).Get(ctx, &agent); err != nil {
			return control.AgentInstance{}, err
		}
		state.Agents[agent.ID] = agent
		return agent, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateCreateSession, func(ctx workflow.Context, req control.CreateSessionRequest) (control.Session, error) {
		ctx = withActivityOptions(ctx)
		var sess control.Session
		if err := workflow.ExecuteActivity(ctx, ActivityCreateSession, req).Get(ctx, &sess); err != nil {
			return control.Session{}, err
		}
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{WorkflowID: SessionWorkflowID(sess.ID)})
		child := workflow.ExecuteChildWorkflow(childCtx, SessionWorkflow, SessionWorkflowInput{Session: sess})
		if err := child.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
			return control.Session{}, err
		}
		state.Sessions[sess.ID] = sess
		return sess, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateLoadSession, func(ctx workflow.Context, req control.LoadSessionRequest) (control.Session, error) {
		ctx = withActivityOptions(ctx)
		var sess control.Session
		if err := workflow.ExecuteActivity(ctx, ActivityLoadSession, req).Get(ctx, &sess); err != nil {
			return control.Session{}, err
		}
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{WorkflowID: SessionWorkflowID(sess.ID)})
		child := workflow.ExecuteChildWorkflow(childCtx, SessionWorkflow, SessionWorkflowInput{Session: sess})
		if err := child.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
			return control.Session{}, err
		}
		state.Sessions[sess.ID] = sess
		return sess, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateHibernate, func(ctx workflow.Context) (control.Container, error) {
		ctx = withActivityOptions(ctx)
		var container control.Container
		if err := workflow.ExecuteActivity(ctx, ActivityHibernate, input.ContainerID).Get(ctx, &container); err != nil {
			return control.Container{}, err
		}
		state.Container = container
		return container, nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateDelete, func(ctx workflow.Context) error {
		ctx = withActivityOptions(ctx)
		if err := workflow.ExecuteActivity(ctx, ActivityDelete, input.ContainerID).Get(ctx, nil); err != nil {
			return err
		}
		state.Deleted = true
		state.Container.State = control.ContainerDeleted
		return nil
	}); err != nil {
		return control.Container{}, err
	}
	if err := workflow.Await(ctx, func() bool { return state.Deleted && workflow.AllHandlersFinished(ctx) }); err != nil {
		return control.Container{}, err
	}
	return state.Container, nil
}

func SessionWorkflow(ctx workflow.Context, input SessionWorkflowInput) (control.Session, error) {
	ctx = withActivityOptions(ctx)
	state := sessionState{Session: input.Session}
	if err := workflow.SetQueryHandler(ctx, QueryGetState, func() (sessionState, error) { return state, nil }); err != nil {
		return control.Session{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdatePrompt, func(ctx workflow.Context, req control.PromptRequest) (control.Run, error) {
		ctx = withActivityOptions(ctx)
		var run control.Run
		if err := workflow.ExecuteActivity(ctx, ActivityPrompt, req).Get(ctx, &run); err != nil {
			return control.Run{}, err
		}
		state.LastRun = run
		state.Session.ActiveRunID = ""
		state.Session.UpdatedAt = workflow.Now(ctx)
		if run.State == control.RunFailed {
			state.Session.State = control.SessionFailed
		} else {
			state.Session.State = control.SessionWaitingInput
		}
		return run, nil
	}); err != nil {
		return control.Session{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, UpdateCancel, func(ctx workflow.Context, req control.CancelRequest) error {
		ctx = withActivityOptions(ctx)
		if err := workflow.ExecuteActivity(ctx, ActivityCancel, req).Get(ctx, nil); err != nil {
			return err
		}
		state.Session.State = control.SessionCancelling
		state.Session.UpdatedAt = workflow.Now(ctx)
		return nil
	}); err != nil {
		return control.Session{}, err
	}
	if err := workflow.Await(ctx, func() bool { return state.Session.State == control.SessionArchived }); err != nil {
		if temporal.IsCanceledError(err) {
			return state.Session, nil
		}
		return control.Session{}, err
	}
	return state.Session, nil
}

func GCWorkflow(ctx workflow.Context) error {
	_ = workflow.Sleep(ctx, time.Minute)
	return nil
}

func withActivityOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	})
}

func ContainerWorkflowID(containerID string) string { return "container/" + containerID }
func SessionWorkflowID(sessionID string) string     { return "session/" + sessionID }

func mapsValues[T any](m map[string]T) []T {
	out := make([]T, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
