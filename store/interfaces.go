package store

import (
	"context"
	"io"

	"agentd/control"
)

type RegistryStore interface {
	UpsertCatalog(ctx context.Context, entries []control.AgentCatalogEntry) error
	ListCatalog(ctx context.Context) ([]control.AgentCatalogEntry, error)
	GetCatalogEntry(ctx context.Context, id string) (control.AgentCatalogEntry, error)
}

type BundleStore interface {
	PutBundle(ctx context.Context, bundle control.Bundle) error
	GetBundle(ctx context.Context, digest string) (control.Bundle, error)
	ListBundles(ctx context.Context) ([]control.Bundle, error)
}

type ContainerStore interface {
	UpsertContainer(ctx context.Context, container control.Container) error
	GetContainer(ctx context.Context, id string) (control.Container, error)
	ListContainers(ctx context.Context) ([]control.Container, error)
	UpsertAgentInstance(ctx context.Context, agent control.AgentInstance) error
	GetAgentInstance(ctx context.Context, id string) (control.AgentInstance, error)
	ListAgentInstances(ctx context.Context, containerID string) ([]control.AgentInstance, error)
}

type SessionStore interface {
	UpsertSession(ctx context.Context, session control.Session) error
	GetSession(ctx context.Context, id string) (control.Session, error)
	ListSessions(ctx context.Context, containerID string) ([]control.Session, error)
	UpsertRun(ctx context.Context, run control.Run) error
	GetRun(ctx context.Context, id string) (control.Run, error)
	ListRuns(ctx context.Context, sessionID string) ([]control.Run, error)
	AppendAudit(ctx context.Context, record control.AuditRecord) error
	ListAudit(ctx context.Context, entityType, entityID string) ([]control.AuditRecord, error)
}

type ArtifactStore interface {
	Put(ctx context.Context, key, contentType string, body io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
}
