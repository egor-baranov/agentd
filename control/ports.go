package control

import (
	"context"

	"agentd/session"
)

type Scheduler interface {
	SelectPlacement(ctx context.Context, plan ExecutionPlan) (Placement, error)
}

type NodeClient interface {
	ProvisionContainer(ctx context.Context, req CreateContainerRequest) (Container, error)
	GetContainer(ctx context.Context, containerID string) (ContainerSnapshot, error)
	HibernateContainer(ctx context.Context, containerID string) (Container, error)
	DeleteContainer(ctx context.Context, containerID string) error
	EnsureAgent(ctx context.Context, req InstallAgentRequest) (AgentInstance, error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
	LoadSession(ctx context.Context, req LoadSessionRequest) (Session, error)
	Prompt(ctx context.Context, req PromptRequest, publish func(session.Event) error) (Run, error)
	Cancel(ctx context.Context, req CancelRequest) error
}

type ACPClient interface {
	Initialize(ctx context.Context) (ACPNegotiation, error)
	NewSession(ctx context.Context, req SessionSpec) (ACPNegotiation, error)
	LoadSession(ctx context.Context, sessionID string) (ACPNegotiation, error)
	Prompt(ctx context.Context, sessionID, runID string, input SessionInput) error
	Cancel(ctx context.Context, sessionID, runID string) error
	ListSessions(ctx context.Context) ([]string, error)
	Close() error
}

type AuthBroker interface {
	Start(ctx context.Context, mode AuthMode, agent AgentInstance) (string, error)
}

type Orchestrator interface {
	SyncRegistry(ctx context.Context, source string) ([]AgentCatalogEntry, error)
	ListAgents(ctx context.Context) ([]AgentCatalogEntry, error)
	GetAgent(ctx context.Context, id string) (AgentCatalogEntry, error)
	CreateContainer(ctx context.Context, req CreateContainerRequest) (Container, error)
	GetContainer(ctx context.Context, id string) (ContainerSnapshot, error)
	HibernateContainer(ctx context.Context, id string) (Container, error)
	DeleteContainer(ctx context.Context, id string) error
	InstallAgent(ctx context.Context, req InstallAgentRequest) (AgentInstance, error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
	GetSession(ctx context.Context, id string) (SessionDetail, error)
	PromptSession(ctx context.Context, req PromptRequest) (Run, error)
	CancelRun(ctx context.Context, req CancelRequest) error
}
