package workflows

import (
	"context"
	"fmt"
	"slices"
	"time"

	"agentd/bundle"
	"agentd/control"
	"agentd/registry"
	"agentd/session"
	"agentd/store"
)

type Activities struct {
	RegistrySyncer registry.Syncer
	BundleResolver bundle.Resolver
	RegistryStore  store.RegistryStore
	BundleStore    store.BundleStore
	Containers     store.ContainerStore
	Sessions       store.SessionStore
	Artifacts      store.ArtifactStore
	Broker         session.Broker
	Nodes          map[string]control.NodeClient
}

type BundleRequest struct {
	Agent    control.AgentCatalogEntry `json:"agent"`
	Platform string                    `json:"platform,omitempty"`
}

func (a *Activities) RegistrySync(ctx context.Context, source string) ([]control.AgentCatalogEntry, error) {
	return a.RegistrySyncer.Sync(ctx, source)
}

func (a *Activities) ResolveBundle(ctx context.Context, req BundleRequest) (control.Bundle, error) {
	bundle, err := a.BundleResolver.Resolve(req.Agent, req.Platform)
	if err != nil {
		return control.Bundle{}, err
	}
	if err := a.BundleStore.PutBundle(ctx, bundle); err != nil {
		return control.Bundle{}, err
	}
	return bundle, nil
}

func (a *Activities) ProvisionContainer(ctx context.Context, req control.CreateContainerRequest) (control.Container, error) {
	node, err := a.node(req.Profile.NodeID)
	if err != nil {
		return control.Container{}, err
	}
	container, err := node.ProvisionContainer(ctx, req)
	if err != nil {
		return control.Container{}, err
	}
	if err := a.Containers.UpsertContainer(ctx, container); err != nil {
		return control.Container{}, err
	}
	_ = a.Sessions.AppendAudit(ctx, audit("container", container.ID, "provisioned"))
	return container, nil
}

func (a *Activities) EnsureAgent(ctx context.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
	container, node, err := a.containerNode(ctx, req.ContainerID)
	if err != nil {
		return control.AgentInstance{}, err
	}
	agent, err := node.EnsureAgent(ctx, req)
	if err != nil {
		return control.AgentInstance{}, err
	}
	if err := a.Containers.UpsertAgentInstance(ctx, agent); err != nil {
		return control.AgentInstance{}, err
	}
	if !slices.Contains(container.BundleDigests, req.Plan.Bundle.Digest) {
		container.BundleDigests = append(container.BundleDigests, req.Plan.Bundle.Digest)
		container.UpdatedAt = time.Now().UTC()
		_ = a.Containers.UpsertContainer(ctx, container)
	}
	_ = a.Sessions.AppendAudit(ctx, audit("agent_instance", agent.ID, "ready"))
	return agent, nil
}

func (a *Activities) CreateSession(ctx context.Context, req control.CreateSessionRequest) (control.Session, error) {
	_, node, err := a.containerNode(ctx, req.ContainerID)
	if err != nil {
		return control.Session{}, err
	}
	sess, err := node.CreateSession(ctx, req)
	if err != nil {
		return control.Session{}, err
	}
	if err := a.Sessions.UpsertSession(ctx, sess); err != nil {
		return control.Session{}, err
	}
	_ = a.Sessions.AppendAudit(ctx, audit("session", sess.ID, "created"))
	return sess, nil
}

func (a *Activities) LoadSession(ctx context.Context, req control.LoadSessionRequest) (control.Session, error) {
	_, node, err := a.containerNode(ctx, req.ContainerID)
	if err != nil {
		return control.Session{}, err
	}
	sess, err := node.LoadSession(ctx, req)
	if err != nil {
		return control.Session{}, err
	}
	if err := a.Sessions.UpsertSession(ctx, sess); err != nil {
		return control.Session{}, err
	}
	_ = a.Sessions.AppendAudit(ctx, audit("session", sess.ID, "loaded"))
	return sess, nil
}

func (a *Activities) Prompt(ctx context.Context, req control.PromptRequest) (control.Run, error) {
	sess, err := a.Sessions.GetSession(ctx, req.SessionID)
	if err != nil {
		return control.Run{}, err
	}
	_, node, err := a.containerNode(ctx, sess.ContainerID)
	if err != nil {
		return control.Run{}, err
	}
	run := control.Run{ID: req.RunID, SessionID: sess.ID, ContainerID: sess.ContainerID, AgentInstanceID: sess.AgentInstanceID, State: control.RunDispatching, Input: req.Input, StartedAt: time.Now().UTC()}
	if run.ID == "" {
		run.ID = control.NewID("run")
		req.RunID = run.ID
	}
	sess.State = control.SessionStreaming
	sess.ActiveRunID = run.ID
	sess.UpdatedAt = time.Now().UTC()
	_ = a.Sessions.UpsertSession(ctx, sess)
	_ = a.Sessions.UpsertRun(ctx, run)
	_ = a.publish(ctx, session.Event{SessionID: sess.ID, RunID: run.ID, Type: "run.started", Sequence: 1, Time: time.Now().UTC()})

	final, err := node.Prompt(ctx, req, func(event session.Event) error {
		return a.publish(ctx, event)
	})
	if err != nil {
		run.State = control.RunFailed
		run.Error = err.Error()
		run.CompletedAt = ptr(time.Now().UTC())
		run.Sequence++
		_ = a.Sessions.UpsertRun(ctx, run)
		sess.State = control.SessionFailed
		sess.UpdatedAt = time.Now().UTC()
		_ = a.Sessions.UpsertSession(ctx, sess)
		return run, nil
	}
	if err := a.Sessions.UpsertRun(ctx, final); err != nil {
		return control.Run{}, err
	}
	sess.ActiveRunID = ""
	sess.UpdatedAt = time.Now().UTC()
	if final.State == control.RunFailed {
		sess.State = control.SessionFailed
	} else {
		sess.State = control.SessionWaitingInput
	}
	if err := a.Sessions.UpsertSession(ctx, sess); err != nil {
		return control.Run{}, err
	}
	_ = a.Sessions.AppendAudit(ctx, audit("run", final.ID, string(final.State)))
	return final, nil
}

func (a *Activities) Cancel(ctx context.Context, req control.CancelRequest) error {
	sess, err := a.Sessions.GetSession(ctx, req.SessionID)
	if err != nil {
		return err
	}
	_, node, err := a.containerNode(ctx, sess.ContainerID)
	if err != nil {
		return err
	}
	sess.State = control.SessionCancelling
	sess.UpdatedAt = time.Now().UTC()
	_ = a.Sessions.UpsertSession(ctx, sess)
	_ = a.publish(ctx, session.Event{SessionID: req.SessionID, RunID: req.RunID, Type: "run.cancelling", Time: time.Now().UTC()})
	return node.Cancel(ctx, req)
}

func (a *Activities) Hibernate(ctx context.Context, containerID string) (control.Container, error) {
	container, node, err := a.containerNode(ctx, containerID)
	if err != nil {
		return control.Container{}, err
	}
	updated, err := node.HibernateContainer(ctx, containerID)
	if err != nil {
		return control.Container{}, err
	}
	updated.BundleDigests = container.BundleDigests
	if err := a.Containers.UpsertContainer(ctx, updated); err != nil {
		return control.Container{}, err
	}
	return updated, nil
}

func (a *Activities) Delete(ctx context.Context, containerID string) error {
	container, node, err := a.containerNode(ctx, containerID)
	if err != nil {
		return err
	}
	if err := node.DeleteContainer(ctx, containerID); err != nil {
		return err
	}
	container.State = control.ContainerDeleted
	container.UpdatedAt = time.Now().UTC()
	return a.Containers.UpsertContainer(ctx, container)
}

func (a *Activities) publish(ctx context.Context, event session.Event) error {
	if a.Broker == nil {
		return nil
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	return a.Broker.Publish(ctx, event)
}

func (a *Activities) containerNode(ctx context.Context, containerID string) (control.Container, control.NodeClient, error) {
	container, err := a.Containers.GetContainer(ctx, containerID)
	if err != nil {
		return control.Container{}, nil, err
	}
	node, err := a.node(container.NodeID)
	return container, node, err
}

func (a *Activities) node(nodeID string) (control.NodeClient, error) {
	if nodeID != "" {
		if client, ok := a.Nodes[nodeID]; ok {
			return client, nil
		}
	}
	if len(a.Nodes) == 1 {
		for _, client := range a.Nodes {
			return client, nil
		}
	}
	return nil, fmt.Errorf("node client not configured for %q", nodeID)
}

func audit(entityType, entityID, action string) control.AuditRecord {
	return control.AuditRecord{ID: control.NewID("audit"), EntityType: entityType, EntityID: entityID, Action: action, CreatedAt: time.Now().UTC()}
}

func ptr[T any](v T) *T { return &v }
