package store

import (
	"context"
	"encoding/json"
	"fmt"

	"agentd/control"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	const schema = `
create table if not exists registry_agents (
  id text primary key,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create table if not exists bundles (
  digest text primary key,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create table if not exists containers (
  id text primary key,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create table if not exists agent_instances (
  id text primary key,
  container_id text not null,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create index if not exists idx_agent_instances_container_id on agent_instances(container_id);
create table if not exists sessions (
  id text primary key,
  container_id text not null,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create index if not exists idx_sessions_container_id on sessions(container_id);
create table if not exists runs (
  id text primary key,
  session_id text not null,
  body jsonb not null,
  updated_at timestamptz not null default now()
);
create index if not exists idx_runs_session_id on runs(session_id);
create table if not exists audits (
  id text primary key,
  entity_type text not null,
  entity_id text not null,
  action text not null,
  body jsonb not null,
  created_at timestamptz not null default now()
);
create index if not exists idx_audits_entity on audits(entity_type, entity_id, created_at desc);
`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

func (s *PostgresStore) UpsertCatalog(ctx context.Context, entries []control.AgentCatalogEntry) error {
	for _, entry := range entries {
		if err := upsertJSON(ctx, s.pool, "registry_agents", entry.ID, map[string]any{"id": entry.ID}, entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) ListCatalog(ctx context.Context) ([]control.AgentCatalogEntry, error) {
	return listJSON[control.AgentCatalogEntry](ctx, s.pool, "select body from registry_agents order by id")
}

func (s *PostgresStore) GetCatalogEntry(ctx context.Context, id string) (control.AgentCatalogEntry, error) {
	return getJSON[control.AgentCatalogEntry](ctx, s.pool, "select body from registry_agents where id=$1", id)
}

func (s *PostgresStore) PutBundle(ctx context.Context, bundle control.Bundle) error {
	return upsertJSON(ctx, s.pool, "bundles", bundle.Digest, map[string]any{"digest": bundle.Digest}, bundle)
}

func (s *PostgresStore) GetBundle(ctx context.Context, digest string) (control.Bundle, error) {
	return getJSON[control.Bundle](ctx, s.pool, "select body from bundles where digest=$1", digest)
}

func (s *PostgresStore) ListBundles(ctx context.Context) ([]control.Bundle, error) {
	return listJSON[control.Bundle](ctx, s.pool, "select body from bundles order by digest")
}

func (s *PostgresStore) UpsertContainer(ctx context.Context, container control.Container) error {
	return upsertJSON(ctx, s.pool, "containers", container.ID, map[string]any{"id": container.ID}, container)
}

func (s *PostgresStore) GetContainer(ctx context.Context, id string) (control.Container, error) {
	return getJSON[control.Container](ctx, s.pool, "select body from containers where id=$1", id)
}

func (s *PostgresStore) ListContainers(ctx context.Context) ([]control.Container, error) {
	return listJSON[control.Container](ctx, s.pool, "select body from containers order by updated_at desc")
}

func (s *PostgresStore) UpsertAgentInstance(ctx context.Context, agent control.AgentInstance) error {
	body, err := json.Marshal(agent)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
insert into agent_instances (id, container_id, body, updated_at)
values ($1,$2,$3,now())
on conflict (id) do update set container_id=excluded.container_id, body=excluded.body, updated_at=now()`, agent.ID, agent.ContainerID, body)
	return err
}

func (s *PostgresStore) GetAgentInstance(ctx context.Context, id string) (control.AgentInstance, error) {
	return getJSON[control.AgentInstance](ctx, s.pool, "select body from agent_instances where id=$1", id)
}

func (s *PostgresStore) ListAgentInstances(ctx context.Context, containerID string) ([]control.AgentInstance, error) {
	if containerID == "" {
		return listJSON[control.AgentInstance](ctx, s.pool, "select body from agent_instances order by updated_at desc")
	}
	return listJSON[control.AgentInstance](ctx, s.pool, "select body from agent_instances where container_id=$1 order by updated_at desc", containerID)
}

func (s *PostgresStore) UpsertSession(ctx context.Context, session control.Session) error {
	body, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
insert into sessions (id, container_id, body, updated_at)
values ($1,$2,$3,now())
on conflict (id) do update set container_id=excluded.container_id, body=excluded.body, updated_at=now()`, session.ID, session.ContainerID, body)
	return err
}

func (s *PostgresStore) GetSession(ctx context.Context, id string) (control.Session, error) {
	return getJSON[control.Session](ctx, s.pool, "select body from sessions where id=$1", id)
}

func (s *PostgresStore) ListSessions(ctx context.Context, containerID string) ([]control.Session, error) {
	if containerID == "" {
		return listJSON[control.Session](ctx, s.pool, "select body from sessions order by updated_at desc")
	}
	return listJSON[control.Session](ctx, s.pool, "select body from sessions where container_id=$1 order by updated_at desc", containerID)
}

func (s *PostgresStore) UpsertRun(ctx context.Context, run control.Run) error {
	body, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
insert into runs (id, session_id, body, updated_at)
values ($1,$2,$3,now())
on conflict (id) do update set session_id=excluded.session_id, body=excluded.body, updated_at=now()`, run.ID, run.SessionID, body)
	return err
}

func (s *PostgresStore) GetRun(ctx context.Context, id string) (control.Run, error) {
	return getJSON[control.Run](ctx, s.pool, "select body from runs where id=$1", id)
}

func (s *PostgresStore) ListRuns(ctx context.Context, sessionID string) ([]control.Run, error) {
	if sessionID == "" {
		return listJSON[control.Run](ctx, s.pool, "select body from runs order by updated_at desc")
	}
	return listJSON[control.Run](ctx, s.pool, "select body from runs where session_id=$1 order by updated_at desc", sessionID)
}

func (s *PostgresStore) AppendAudit(ctx context.Context, record control.AuditRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `insert into audits (id, entity_type, entity_id, action, body, created_at) values ($1,$2,$3,$4,$5,$6)`, record.ID, record.EntityType, record.EntityID, record.Action, body, record.CreatedAt)
	return err
}

func (s *PostgresStore) ListAudit(ctx context.Context, entityType, entityID string) ([]control.AuditRecord, error) {
	if entityType == "" && entityID == "" {
		return listJSON[control.AuditRecord](ctx, s.pool, "select body from audits order by created_at desc")
	}
	return listJSON[control.AuditRecord](ctx, s.pool, "select body from audits where entity_type=$1 and entity_id=$2 order by created_at desc", entityType, entityID)
}

func upsertJSON(ctx context.Context, pool *pgxpool.Pool, table, key string, _ map[string]any, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`insert into %s (%s, body, updated_at) values ($1,$2,now()) on conflict (%s) do update set body=excluded.body, updated_at=now()`, table, primaryKeyColumn(table), primaryKeyColumn(table))
	_, err = pool.Exec(ctx, query, key, body)
	return err
}

func primaryKeyColumn(table string) string {
	switch table {
	case "bundles":
		return "digest"
	default:
		return "id"
	}
}

func getJSON[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (T, error) {
	var raw []byte
	if err := pool.QueryRow(ctx, query, args...).Scan(&raw); err != nil {
		return *new(T), fmt.Errorf("query row: %w", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return *new(T), fmt.Errorf("decode row: %w", err)
	}
	return out, nil
}

func listJSON[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) ([]T, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
