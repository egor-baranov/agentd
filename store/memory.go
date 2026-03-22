package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"agentd/control"
)

type MemoryStore struct {
	mu         sync.RWMutex
	registry   map[string]control.AgentCatalogEntry
	bundles    map[string]control.Bundle
	containers map[string]control.Container
	agents     map[string]control.AgentInstance
	sessions   map[string]control.Session
	runs       map[string]control.Run
	audits     []control.AuditRecord
	artifacts  map[string]artifactBlob
}

type artifactBlob struct {
	contentType string
	body        []byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		registry:   map[string]control.AgentCatalogEntry{},
		bundles:    map[string]control.Bundle{},
		containers: map[string]control.Container{},
		agents:     map[string]control.AgentInstance{},
		sessions:   map[string]control.Session{},
		runs:       map[string]control.Run{},
		artifacts:  map[string]artifactBlob{},
	}
}

func (s *MemoryStore) UpsertCatalog(_ context.Context, entries []control.AgentCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range entries {
		s.registry[entry.ID] = entry
	}
	return nil
}

func (s *MemoryStore) ListCatalog(_ context.Context) ([]control.AgentCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.AgentCatalogEntry, 0, len(s.registry))
	for _, entry := range s.registry {
		out = append(out, entry)
	}
	return out, nil
}

func (s *MemoryStore) GetCatalogEntry(_ context.Context, id string) (control.AgentCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.registry[id]
	if !ok {
		return control.AgentCatalogEntry{}, control.ErrNotFound
	}
	return entry, nil
}

func (s *MemoryStore) PutBundle(_ context.Context, bundle control.Bundle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[bundle.Digest] = bundle
	return nil
}

func (s *MemoryStore) GetBundle(_ context.Context, digest string) (control.Bundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bundle, ok := s.bundles[digest]
	if !ok {
		return control.Bundle{}, control.ErrNotFound
	}
	return bundle, nil
}

func (s *MemoryStore) ListBundles(_ context.Context) ([]control.Bundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.Bundle, 0, len(s.bundles))
	for _, bundle := range s.bundles {
		out = append(out, bundle)
	}
	return out, nil
}

func (s *MemoryStore) UpsertContainer(_ context.Context, container control.Container) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[container.ID] = container
	return nil
}

func (s *MemoryStore) GetContainer(_ context.Context, id string) (control.Container, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	container, ok := s.containers[id]
	if !ok {
		return control.Container{}, control.ErrNotFound
	}
	return container, nil
}

func (s *MemoryStore) ListContainers(_ context.Context) ([]control.Container, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.Container, 0, len(s.containers))
	for _, container := range s.containers {
		out = append(out, container)
	}
	return out, nil
}

func (s *MemoryStore) UpsertAgentInstance(_ context.Context, agent control.AgentInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agent.ID] = agent
	return nil
}

func (s *MemoryStore) GetAgentInstance(_ context.Context, id string) (control.AgentInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[id]
	if !ok {
		return control.AgentInstance{}, control.ErrNotFound
	}
	return agent, nil
}

func (s *MemoryStore) ListAgentInstances(_ context.Context, containerID string) ([]control.AgentInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.AgentInstance, 0, len(s.agents))
	for _, agent := range s.agents {
		if containerID == "" || agent.ContainerID == containerID {
			out = append(out, agent)
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertSession(_ context.Context, session control.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *MemoryStore) GetSession(_ context.Context, id string) (control.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return control.Session{}, control.ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) ListSessions(_ context.Context, containerID string) ([]control.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if containerID == "" || session.ContainerID == containerID {
			out = append(out, session)
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertRun(_ context.Context, run control.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetRun(_ context.Context, id string) (control.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return control.Run{}, control.ErrNotFound
	}
	return run, nil
}

func (s *MemoryStore) ListRuns(_ context.Context, sessionID string) ([]control.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if sessionID == "" || run.SessionID == sessionID {
			out = append(out, run)
		}
	}
	return out, nil
}

func (s *MemoryStore) AppendAudit(_ context.Context, record control.AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, record)
	return nil
}

func (s *MemoryStore) ListAudit(_ context.Context, entityType, entityID string) ([]control.AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]control.AuditRecord, 0, len(s.audits))
	for _, record := range s.audits {
		if (entityType == "" || record.EntityType == entityType) && (entityID == "" || record.EntityID == entityID) {
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *MemoryStore) Put(_ context.Context, key, contentType string, body io.Reader) error {
	buf, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts[key] = artifactBlob{contentType: contentType, body: buf}
	return nil
}

func (s *MemoryStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.artifacts[key]
	if !ok {
		return nil, "", control.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(blob.body)), blob.contentType, nil
}
