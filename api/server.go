package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agentd/control"
	"agentd/observer"
	"agentd/session"
	"agentd/store"
	"agentd/workflows"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	Logger     *slog.Logger
	Metrics    *observer.Metrics
	Temporal   workflows.Client
	Registry   store.RegistryStore
	Bundles    store.BundleStore
	Containers store.ContainerStore
	Sessions   store.SessionStore
	Scheduler  control.Scheduler
	Broker     session.Broker
}

type CreateSessionRequest struct {
	AgentID          string                     `json:"agent_id,omitempty"`
	ManualAgent      *control.AgentCatalogEntry `json:"manual_agent,omitempty"`
	ContainerID      string                     `json:"container_id,omitempty"`
	Platform         string                     `json:"platform,omitempty"`
	ContainerProfile control.ContainerProfile   `json:"container_profile,omitempty"`
	Session          control.SessionSpec        `json:"session"`
	LLMProfile       *control.LLMProfile        `json:"llm_profile,omitempty"`
	Policy           control.ExecutionPolicy    `json:"policy,omitempty"`
}

type InstallAgentRequest struct {
	AgentID     string                     `json:"agent_id,omitempty"`
	ManualAgent *control.AgentCatalogEntry `json:"manual_agent,omitempty"`
	Platform    string                     `json:"platform,omitempty"`
	WorkingDir  string                     `json:"working_dir,omitempty"`
	Env         map[string]string          `json:"env,omitempty"`
	LLMProfile  *control.LLMProfile        `json:"llm_profile,omitempty"`
	Policy      control.ExecutionPolicy    `json:"policy,omitempty"`
}

type PromptRequest struct {
	RunID    string         `json:"run_id,omitempty"`
	Prompt   string         `json:"prompt"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CancelRequest struct {
	RunID string `json:"run_id"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/registry/agents", s.wrap("registry_list", s.handleListRegistryAgents))
	mux.HandleFunc("GET /v1/registry/agents/{id}", s.wrap("registry_get", s.handleGetRegistryAgent))
	mux.HandleFunc("POST /v1/containers", s.wrap("container_create", s.handleCreateContainer))
	mux.HandleFunc("GET /v1/containers/{id}", s.wrap("container_get", s.handleGetContainer))
	mux.HandleFunc("POST /v1/containers/", s.wrap("container_actions", s.handleContainerAction))
	mux.HandleFunc("DELETE /v1/containers/{id}", s.wrap("container_delete", s.handleDeleteContainer))
	mux.HandleFunc("POST /v1/containers/{id}/agents", s.wrap("agent_install", s.handleInstallAgent))
	mux.HandleFunc("POST /v1/sessions", s.wrap("session_create", s.handleCreateSession))
	mux.HandleFunc("GET /v1/sessions/{id}", s.wrap("session_get", s.handleGetSession))
	mux.HandleFunc("POST /v1/sessions/", s.wrap("session_actions", s.handleSessionAction))
	mux.HandleFunc("GET /v1/sessions/{id}/events", s.wrap("session_events", s.handleSessionEvents))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.Metrics.Registry, promhttp.HandlerOpts{}))
	return mux
}

func (s *Server) handleListRegistryAgents(w http.ResponseWriter, r *http.Request) error {
	agents, err := s.Registry.ListCatalog(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (s *Server) handleGetRegistryAgent(w http.ResponseWriter, r *http.Request) error {
	agent, err := s.Registry.GetCatalogEntry(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, agent)
}

func (s *Server) handleCreateContainer(w http.ResponseWriter, r *http.Request) error {
	var req control.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest(err)
	}
	container, err := s.Temporal.StartContainer(r.Context(), req)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, container)
}

func (s *Server) handleGetContainer(w http.ResponseWriter, r *http.Request) error {
	container, err := s.Containers.GetContainer(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	agents, _ := s.Containers.ListAgentInstances(r.Context(), container.ID)
	sessions, _ := s.Sessions.ListSessions(r.Context(), container.ID)
	return writeJSON(w, http.StatusOK, control.ContainerSnapshot{Container: container, Agents: agents, Sessions: sessions})
}

func (s *Server) handleHibernateContainer(w http.ResponseWriter, r *http.Request) error {
	container, err := s.Temporal.HibernateContainer(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, container)
}

func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request) error {
	if strings.HasSuffix(r.URL.Path, ":hibernate") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/containers/"), ":hibernate")
		container, err := s.Temporal.HibernateContainer(r.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, container)
	}
	return httpError{status: http.StatusNotFound, msg: "not found"}
}

func (s *Server) handleDeleteContainer(w http.ResponseWriter, r *http.Request) error {
	if err := s.Temporal.DeleteContainer(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleInstallAgent(w http.ResponseWriter, r *http.Request) error {
	container, err := s.Containers.GetContainer(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	var req InstallAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest(err)
	}
	agent, bundle, err := s.resolveAgentBundle(r.Context(), req.AgentID, req.ManualAgent, req.Platform)
	if err != nil {
		return err
	}
	plan, err := control.CompileExecutionPlan(control.CompileInput{
		Agent:            agent,
		Bundle:           bundle,
		ContainerProfile: container.Profile,
		Session:          control.SessionSpec{WorkingDir: req.WorkingDir, Env: req.Env},
		LLMProfile:       req.LLMProfile,
		Placement:        control.Placement{ContainerID: container.ID, NodeID: container.NodeID},
		Policy:           normalizePolicy(req.Policy),
	})
	if err != nil {
		return err
	}
	installed, err := s.Temporal.InstallAgent(r.Context(), control.InstallAgentRequest{ContainerID: container.ID, Plan: plan})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, installed)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) error {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest(err)
	}
	agent, bundle, err := s.resolveAgentBundle(r.Context(), req.AgentID, req.ManualAgent, req.Platform)
	if err != nil {
		return err
	}
	placement := control.Placement{ContainerID: req.ContainerID}
	profile := req.ContainerProfile
	if req.ContainerID != "" {
		container, err := s.Containers.GetContainer(r.Context(), req.ContainerID)
		if err != nil {
			return err
		}
		placement.NodeID = container.NodeID
		profile = container.Profile
	}
	plan, err := control.CompileExecutionPlan(control.CompileInput{
		Agent:            agent,
		Bundle:           bundle,
		ContainerProfile: profile,
		Session:          req.Session,
		LLMProfile:       req.LLMProfile,
		Placement:        placement,
		Policy:           normalizePolicy(req.Policy),
	})
	if err != nil {
		return err
	}
	if plan.Placement.ContainerID == "" {
		placement, err = s.Scheduler.SelectPlacement(r.Context(), plan)
		if err != nil {
			return err
		}
		plan.Placement = placement
	}
	containerID := plan.Placement.ContainerID
	if containerID == "" {
		createReq := control.CreateContainerRequest{Profile: plan.ContainerProfile, Capacity: 4}
		createReq.Profile.NodeID = plan.Placement.NodeID
		container, err := s.Temporal.StartContainer(r.Context(), createReq)
		if err != nil {
			return err
		}
		containerID = container.ID
		plan.Placement.ContainerID = container.ID
		plan.Placement.NodeID = container.NodeID
	}
	agentInstance, err := s.Temporal.InstallAgent(r.Context(), control.InstallAgentRequest{ContainerID: containerID, Plan: plan})
	if err != nil {
		return err
	}
	sess, err := s.Temporal.CreateSession(r.Context(), control.CreateSessionRequest{ContainerID: containerID, AgentInstanceID: agentInstance.ID, Plan: plan})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.Sessions.GetSession(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	runs, _ := s.Sessions.ListRuns(r.Context(), sess.ID)
	return writeJSON(w, http.StatusOK, control.SessionDetail{Session: sess, Runs: runs})
}

func (s *Server) handlePromptSession(w http.ResponseWriter, r *http.Request) error {
	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest(err)
	}
	run, err := s.Temporal.Prompt(r.Context(), control.PromptRequest{SessionID: r.PathValue("id"), RunID: req.RunID, Input: control.SessionInput{Prompt: req.Prompt, Metadata: req.Metadata}})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleCancelSession(w http.ResponseWriter, r *http.Request) error {
	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest(err)
	}
	if err := s.Temporal.Cancel(r.Context(), control.CancelRequest{SessionID: r.PathValue("id"), RunID: req.RunID}); err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (s *Server) handleSessionAction(w http.ResponseWriter, r *http.Request) error {
	switch {
	case strings.HasSuffix(r.URL.Path, ":prompt"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), ":prompt")
		r.SetPathValue("id", id)
		return s.handlePromptSession(w, r)
	case strings.HasSuffix(r.URL.Path, ":cancel"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), ":cancel")
		r.SetPathValue("id", id)
		return s.handleCancelSession(w, r)
	default:
		return httpError{status: http.StatusNotFound, msg: "not found"}
	}
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ctx := r.Context()
	ch, cancel := s.Broker.Subscribe(ctx, r.PathValue("id"))
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			raw, _ := json.Marshal(event)
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, raw); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}

func (s *Server) resolveAgentBundle(ctx context.Context, agentID string, manual *control.AgentCatalogEntry, platform string) (control.AgentCatalogEntry, control.Bundle, error) {
	var agent control.AgentCatalogEntry
	var err error
	if manual != nil {
		agent = *manual
		agent.Source = control.AgentSourceManual
		if agent.ID == "" {
			agent.ID = control.NewID("agent")
		}
		if agent.Version == "" {
			agent.Version = "manual"
		}
		agent.UpdatedAt = time.Now().UTC()
	} else {
		agent, err = s.Registry.GetCatalogEntry(ctx, agentID)
		if err != nil {
			return control.AgentCatalogEntry{}, control.Bundle{}, err
		}
	}
	bundle, err := s.Temporal.ResolveBundle(ctx, workflows.BundleRequest{Agent: agent, Platform: platform})
	if err != nil {
		return control.AgentCatalogEntry{}, control.Bundle{}, err
	}
	return agent, bundle, nil
}

func (s *Server) wrap(route string, fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		code := http.StatusOK
		if err := fn(w, r); err != nil {
			code = writeError(w, err)
		}
		if s.Metrics != nil {
			s.Metrics.HTTPRequests.WithLabelValues(route, strconv.Itoa(code), r.Method).Inc()
			s.Metrics.HTTPDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(value)
}

func normalizePolicy(policy control.ExecutionPolicy) control.ExecutionPolicy {
	if !policy.HostedMode && !policy.AllowPrivateAgents && !policy.AllowTerminalAuth {
		return control.ExecutionPolicy{HostedMode: true, AllowTerminalAuth: false, AllowPrivateAgents: true}
	}
	return policy
}

type httpError struct {
	status int
	msg    string
}

func (e httpError) Error() string { return e.msg }

func badRequest(err error) error { return httpError{status: http.StatusBadRequest, msg: err.Error()} }

func writeError(w http.ResponseWriter, err error) int {
	status := http.StatusInternalServerError
	msg := err.Error()
	var httpErr httpError
	switch {
	case errors.As(err, &httpErr):
		status = httpErr.status
		msg = httpErr.msg
	case errors.Is(err, control.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, control.ErrNoCapacity), errors.Is(err, control.ErrUnsupportedAuth), errors.Is(err, control.ErrUnsupportedBundle):
		status = http.StatusBadRequest
	}
	_ = writeJSON(w, status, map[string]any{"error": map[string]any{"message": msg}})
	return status
}

func Shutdown(ctx context.Context, server *http.Server) error {
	return server.Shutdown(ctx)
}
