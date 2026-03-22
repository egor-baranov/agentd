package supervisor

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"agentd/acp"
	"agentd/control"
	"agentd/runtime/rpc"
	"agentd/session"
)

type Manager struct {
	containerID string
	workspace   string
	logger      *slog.Logger

	mu       sync.RWMutex
	agents   map[string]*agentHandle
	sessions map[string]control.Session
}

type agentHandle struct {
	instance control.AgentInstance
	plan     control.ExecutionPlan
	client   *acp.StdioClient
	cmd      *exec.Cmd
	cancel   context.CancelFunc

	mu      sync.Mutex
	streams map[string]chan acp.StreamEvent
}

func NewManager(containerID, workspace string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		containerID: containerID,
		workspace:   workspace,
		logger:      logger,
		agents:      map[string]*agentHandle{},
		sessions:    map[string]control.Session{},
	}
}

func (m *Manager) EnsureAgent(ctx context.Context, req *control.InstallAgentRequest) (*rpc.AgentResponse, error) {
	key := agentKey(req.Plan)

	m.mu.RLock()
	if handle, ok := m.agents[key]; ok {
		m.mu.RUnlock()
		return &rpc.AgentResponse{Agent: handle.snapshot()}, nil
	}
	m.mu.RUnlock()

	now := time.Now().UTC()
	instance := control.AgentInstance{
		ID:           control.NewID("agent"),
		ContainerID:  m.containerID,
		AgentID:      req.Plan.Agent.ID,
		Version:      req.Plan.Agent.Version,
		BundleDigest: req.Plan.Bundle.Digest,
		State:        control.AgentInstalling,
		Cwd:          req.Plan.Session.WorkingDir,
		Env:          req.Plan.InjectionEnv,
		Limits:       req.Plan.ContainerProfile.ResourceLimits,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	mergedEnv := mergeEnv(os.Environ(), req.Plan.ContainerProfile.Env, req.Plan.Bundle.Env, req.Plan.InjectionEnv)
	stderr := newLogWriter(m.logger.With("container_id", m.containerID, "agent_id", req.Plan.Agent.ID))
	procCtx, cancel := context.WithCancel(context.Background())
	client, cmd, err := acp.StartCommand(procCtx, req.Plan.Bundle.Entrypoint[0], req.Plan.Bundle.Entrypoint[1:], req.Plan.Session.WorkingDir, mergedEnv, stderr)
	if err != nil {
		cancel()
		instance.State = control.AgentFailed
		instance.UpdatedAt = time.Now().UTC()
		return nil, err
	}
	negotiated, err := client.Initialize(ctx)
	if err != nil {
		cancel()
		_ = client.Close()
		_ = cmd.Process.Kill()
		instance.State = control.AgentFailed
		instance.UpdatedAt = time.Now().UTC()
		return nil, err
	}
	instance.State = control.AgentReady
	instance.Negotiated = negotiated
	instance.UpdatedAt = time.Now().UTC()

	handle := &agentHandle{instance: instance, plan: req.Plan, client: client, cmd: cmd, cancel: cancel, streams: map[string]chan acp.StreamEvent{}}
	client.SetNotificationHandler(handle.routeEvent)
	go handle.wait(m.logger)

	m.mu.Lock()
	m.agents[key] = handle
	m.mu.Unlock()
	return &rpc.AgentResponse{Agent: handle.snapshot()}, nil
}

func (m *Manager) CreateSession(ctx context.Context, req *control.CreateSessionRequest) (*rpc.SessionResponse, error) {
	handle, err := m.agentByID(req.AgentInstanceID)
	if err != nil {
		return nil, err
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = control.NewID("sess")
	}
	spec := req.Plan.Session
	spec.ResumeSessionID = sessionID
	negotiated, err := handle.client.NewSession(ctx, spec)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := control.Session{
		ID:              sessionID,
		ContainerID:     m.containerID,
		AgentInstanceID: handle.instance.ID,
		State:           control.SessionActive,
		Metadata:        req.Plan.Session.Metadata,
		Negotiated:      negotiated,
		MCPServers:      req.Plan.Session.MCPServers,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m.mu.Lock()
	m.sessions[sess.ID] = sess
	m.mu.Unlock()
	return &rpc.SessionResponse{Session: sess}, nil
}

func (m *Manager) LoadSession(ctx context.Context, req *control.LoadSessionRequest) (*rpc.SessionResponse, error) {
	handle, err := m.agentByID(req.AgentInstanceID)
	if err != nil {
		return nil, err
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = req.ResumeID
	}
	negotiated, err := handle.client.LoadSession(ctx, req.ResumeID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := control.Session{
		ID:              sessionID,
		ContainerID:     m.containerID,
		AgentInstanceID: handle.instance.ID,
		State:           control.SessionActive,
		Metadata:        req.Spec.Metadata,
		Negotiated:      negotiated,
		MCPServers:      req.Spec.MCPServers,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m.mu.Lock()
	m.sessions[sess.ID] = sess
	m.mu.Unlock()
	return &rpc.SessionResponse{Session: sess}, nil
}

func (m *Manager) Prompt(req *control.PromptRequest, stream rpc.Supervisor_PromptServer) error {
	sess, handle, err := m.sessionAgent(req.SessionID)
	if err != nil {
		return err
	}
	runID := req.RunID
	if runID == "" {
		runID = control.NewID("run")
	}
	run := control.Run{ID: runID, SessionID: sess.ID, ContainerID: m.containerID, AgentInstanceID: sess.AgentInstanceID, State: control.RunDispatching, Input: req.Input, StartedAt: time.Now().UTC()}
	events := handle.subscribe(runID)
	defer handle.unsubscribe(runID)
	if err := handle.client.Prompt(stream.Context(), sess.ID, runID, req.Input); err != nil {
		run.State = control.RunFailed
		run.Error = err.Error()
		run.CompletedAt = ptr(time.Now().UTC())
		return stream.Send(&rpc.PromptStreamItem{Run: run, Final: true, Error: err.Error()})
	}
	var seq int64
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event := <-events:
			seq++
			sev := session.Event{SessionID: sess.ID, RunID: runID, Type: event.Type, Sequence: seq, Data: map[string]any{"text": event.Text, "error": event.Error, "metadata": event.Metadata}, Time: time.Now().UTC()}
			run.Sequence = seq
			run.State = control.RunStreaming
			final := event.Final || event.Type == "completed" || event.Type == "cancelled" || event.Type == "failed"
			if final {
				switch event.Type {
				case "cancelled":
					run.State = control.RunCancelled
				case "failed":
					run.State = control.RunFailed
				default:
					run.State = control.RunCompleted
				}
				run.Error = event.Error
				run.CompletedAt = ptr(time.Now().UTC())
			}
			if err := stream.Send(&rpc.PromptStreamItem{Event: sev, Run: run, Final: final, Error: event.Error}); err != nil {
				return err
			}
			if final {
				return nil
			}
		}
	}
}

func (m *Manager) Cancel(ctx context.Context, req *control.CancelRequest) (*rpc.Empty, error) {
	sess, handle, err := m.sessionAgent(req.SessionID)
	if err != nil {
		return nil, err
	}
	return &rpc.Empty{}, handle.client.Cancel(ctx, sess.ID, req.RunID)
}

func (m *Manager) ListAgents(context.Context, *rpc.Empty) (*rpc.ListAgentsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]control.AgentInstance, 0, len(m.agents))
	for _, handle := range m.agents {
		out = append(out, handle.snapshot())
	}
	return &rpc.ListAgentsResponse{Agents: out}, nil
}

func (m *Manager) ListSessions(_ context.Context, _ *rpc.ListSessionsRequest) (*rpc.ListSessionsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]control.Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		out = append(out, sess)
	}
	return &rpc.ListSessionsResponse{Sessions: out}, nil
}

func (m *Manager) agentByID(agentID string) (*agentHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, handle := range m.agents {
		if handle.instance.ID == agentID {
			return handle, nil
		}
	}
	return nil, control.ErrNotFound
}

func (m *Manager) sessionAgent(sessionID string) (control.Session, *agentHandle, error) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return control.Session{}, nil, control.ErrNotFound
	}
	handle, err := m.agentByID(sess.AgentInstanceID)
	return sess, handle, err
}

func agentKey(plan control.ExecutionPlan) string { return plan.Agent.ID + ":" + plan.Bundle.Digest }

func (a *agentHandle) snapshot() control.AgentInstance {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.instance
}

func (a *agentHandle) subscribe(runID string) <-chan acp.StreamEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	ch := make(chan acp.StreamEvent, 64)
	a.streams[runID] = ch
	return ch
}

func (a *agentHandle) unsubscribe(runID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ch, ok := a.streams[runID]; ok {
		delete(a.streams, runID)
		close(ch)
	}
}

func (a *agentHandle) routeEvent(event acp.StreamEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ch, ok := a.streams[event.RunID]; ok {
		select {
		case ch <- event:
		default:
		}
	}
}

func (a *agentHandle) wait(logger *slog.Logger) {
	_ = a.cmd.Wait()
	a.mu.Lock()
	a.instance.State = control.AgentExited
	a.instance.UpdatedAt = time.Now().UTC()
	a.mu.Unlock()
	logger.Info("agent exited", "agent_instance_id", a.instance.ID)
}

func mergeEnv(base []string, layers ...map[string]string) []string {
	m := map[string]string{}
	for _, kv := range base {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	for _, layer := range layers {
		for k, v := range layer {
			m[k] = v
		}
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

type logWriter struct{ logger *slog.Logger }

func newLogWriter(logger *slog.Logger) io.Writer { return &logWriter{logger: logger} }

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Info("agent stderr", "line", string(p))
	return len(p), nil
}

func ptr[T any](v T) *T { return &v }

func SupervisorSocket(root string) string {
	path := filepath.Join(root, "supervisor.sock")
	if len(path) < 96 {
		return path
	}
	sum := sha1.Sum([]byte(root))
	return filepath.Join(os.TempDir(), fmt.Sprintf("agd-%x.sock", sum[:4]))
}

func EnsureWorkspace(root string) (string, error) {
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", fmt.Errorf("mkdir workspace: %w", err)
	}
	return workspace, nil
}
