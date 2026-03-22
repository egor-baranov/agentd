package local

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agentd/control"
	"agentd/runtime/rpc"
	"agentd/runtime/supervisor"
	"agentd/session"
	"google.golang.org/grpc"
)

type Runner struct {
	nodeID string
	root   string
	logger *slog.Logger

	mu         sync.RWMutex
	containers map[string]*containerHandle
}

type containerHandle struct {
	container  control.Container
	root       string
	workspace  string
	socket     string
	server     *grpc.Server
	listener   net.Listener
	conn       *grpc.ClientConn
	supervisor *rpc.SupervisorClient
}

func NewRunner(nodeID, root string, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{nodeID: nodeID, root: root, logger: logger, containers: map[string]*containerHandle{}}
}

func (r *Runner) ProvisionContainer(ctx context.Context, req control.CreateContainerRequest) (control.Container, error) {
	id := req.ID
	if id == "" {
		id = control.NewID("ctr")
	}
	root := filepath.Join(r.root, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return control.Container{}, err
	}
	workspace, err := supervisor.EnsureWorkspace(root)
	if err != nil {
		return control.Container{}, err
	}
	socket := supervisor.SupervisorSocket(root)
	_ = os.Remove(socket)
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return control.Container{}, err
	}
	mgr := supervisor.NewManager(id, workspace, r.logger.With("container_id", id))
	grpcServer := grpc.NewServer()
	rpc.RegisterSupervisorServer(grpcServer, mgr)
	go func() { _ = grpcServer.Serve(lis) }()
	conn, err := rpc.DialUnix(ctx, socket)
	if err != nil {
		grpcServer.Stop()
		_ = lis.Close()
		return control.Container{}, err
	}
	capacity := req.Capacity
	if capacity == 0 {
		capacity = 4
	}
	now := time.Now().UTC()
	container := control.Container{
		ID:           id,
		NodeID:       r.nodeID,
		State:        control.ContainerReady,
		Pinned:       req.Pinned,
		Profile:      req.Profile,
		Metadata:     req.Metadata,
		Capacity:     capacity,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}
	if container.Profile.WorkspaceRoot == "" {
		container.Profile.WorkspaceRoot = workspace
	}
	handle := &containerHandle{container: container, root: root, workspace: workspace, socket: socket, server: grpcServer, listener: lis, conn: conn, supervisor: rpc.NewSupervisorClient(conn)}
	r.mu.Lock()
	r.containers[id] = handle
	r.mu.Unlock()
	return container, nil
}

func (r *Runner) GetContainer(ctx context.Context, containerID string) (control.ContainerSnapshot, error) {
	h, err := r.handle(containerID)
	if err != nil {
		return control.ContainerSnapshot{}, err
	}
	agents, err := h.supervisor.ListAgents(ctx, &rpc.Empty{})
	if err != nil {
		return control.ContainerSnapshot{}, err
	}
	sessions, err := h.supervisor.ListSessions(ctx, &rpc.ListSessionsRequest{})
	if err != nil {
		return control.ContainerSnapshot{}, err
	}
	return control.ContainerSnapshot{Container: h.container, Agents: agents.Agents, Sessions: sessions.Sessions}, nil
}

func (r *Runner) HibernateContainer(_ context.Context, containerID string) (control.Container, error) {
	h, err := r.handle(containerID)
	if err != nil {
		return control.Container{}, err
	}
	h.container.State = control.ContainerHibernating
	h.container.UpdatedAt = time.Now().UTC()
	return h.container, nil
}

func (r *Runner) DeleteContainer(_ context.Context, containerID string) error {
	r.mu.Lock()
	h, ok := r.containers[containerID]
	if ok {
		delete(r.containers, containerID)
	}
	r.mu.Unlock()
	if !ok {
		return control.ErrNotFound
	}
	h.server.Stop()
	_ = h.listener.Close()
	_ = h.conn.Close()
	_ = os.Remove(h.socket)
	return os.RemoveAll(h.root)
}

func (r *Runner) EnsureAgent(ctx context.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
	h, err := r.handle(req.ContainerID)
	if err != nil {
		return control.AgentInstance{}, err
	}
	resp, err := h.supervisor.EnsureAgent(ctx, &req)
	if err != nil {
		return control.AgentInstance{}, err
	}
	return resp.Agent, nil
}

func (r *Runner) CreateSession(ctx context.Context, req control.CreateSessionRequest) (control.Session, error) {
	h, err := r.handle(req.ContainerID)
	if err != nil {
		return control.Session{}, err
	}
	resp, err := h.supervisor.CreateSession(ctx, &req)
	if err != nil {
		return control.Session{}, err
	}
	return resp.Session, nil
}

func (r *Runner) LoadSession(ctx context.Context, req control.LoadSessionRequest) (control.Session, error) {
	h, err := r.handle(req.ContainerID)
	if err != nil {
		return control.Session{}, err
	}
	resp, err := h.supervisor.LoadSession(ctx, &req)
	if err != nil {
		return control.Session{}, err
	}
	return resp.Session, nil
}

func (r *Runner) Prompt(ctx context.Context, req control.PromptRequest, publish func(event session.Event) error) (control.Run, error) {
	h, err := r.handleBySession(ctx, req.SessionID)
	if err != nil {
		return control.Run{}, err
	}
	stream, err := h.supervisor.Prompt(ctx, &req)
	if err != nil {
		return control.Run{}, err
	}
	var final control.Run
	for {
		item, err := stream.Recv()
		if err != nil {
			if final.ID != "" {
				return final, nil
			}
			return control.Run{}, err
		}
		final = item.Run
		if publish != nil && item.Event.Type != "" {
			if err := publish(item.Event); err != nil {
				return control.Run{}, err
			}
		}
		if item.Final {
			return item.Run, nil
		}
	}
}

func (r *Runner) Cancel(ctx context.Context, req control.CancelRequest) error {
	h, err := r.handleBySession(ctx, req.SessionID)
	if err != nil {
		return err
	}
	_, err = h.supervisor.Cancel(ctx, &req)
	return err
}

func (r *Runner) handle(containerID string) (*containerHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.containers[containerID]
	if !ok {
		return nil, control.ErrNotFound
	}
	return h, nil
}

func (r *Runner) handleBySession(ctx context.Context, sessionID string) (*containerHandle, error) {
	r.mu.RLock()
	handles := make([]*containerHandle, 0, len(r.containers))
	for _, h := range r.containers {
		handles = append(handles, h)
	}
	r.mu.RUnlock()
	for _, h := range handles {
		resp, err := h.supervisor.ListSessions(ctx, &rpc.ListSessionsRequest{})
		if err != nil {
			continue
		}
		for _, sess := range resp.Sessions {
			if sess.ID == sessionID {
				return h, nil
			}
		}
	}
	return nil, control.ErrNotFound
}
