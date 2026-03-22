# agentd

`agentd` is an agent daemon for running ACP agents in isolated sandboxes with durable orchestration, multi-agent supervision, and live session streaming.

`agentd` splits the product into a small control plane and execution plane:

- Control plane: HTTP API, Temporal workflows/activities, registry sync, scheduling, metadata stores, SSE fanout.
- Execution plane: node runner, sandbox container abstraction, in-container ACP supervisor, ACP subprocess management.

The system is built around a few core design choices:

- Durable orchestration and lifecycle state live in Temporal workflows.
- Metadata lives in Postgres when `DATABASE_URL` is set, otherwise an in-memory store is used for single-process development.
- Live session events go through Redis when `REDIS_URL` is set, otherwise an in-memory broker is used.
- Node runner control traffic uses gRPC with a compact JSON codec instead of generated protobufs.
- ACP agent traffic uses JSON-RPC over stdio behind `acp` interfaces.

## Repository Layout

- `acp/`: ACP protocol types and stdio JSON-RPC client.
- `api/`: northbound HTTP API and SSE handling.
- `bundle/`: distribution normalization and bundle resolution.
- `control/`: core entities, state machines, execution-plan compilation, config snapshots.
- `cmd/agentd-api/`: northbound HTTP API.
- `cmd/agentd-worker/`: Temporal worker with workflows and activities.
- `cmd/agentd-node/`: execution node runner.
- `cmd/acp-supervisor/`: standalone supervisor binary.
- `obs/`: logs and Prometheus metrics.
- `registry/`: ACP registry parsing and sync.
- `runtime/`: gRPC transport, node client, local runtime, supervisor integration.
- `scheduler/`: placement logic.
- `session/`: event broker and stream fanout.
- `store/`: metadata and artifact stores.
- `workflows/`: workflows, activities, worker registration, Temporal client wrapper.
- `examples/echo-agent/`: fake ACP agent used by tests and local development.
- `test/unit/`: black-box package unit tests and benchmarks.
- `test/e2e/`: HTTP and runtime integration tests.
- `deploy/`: Docker development support.

## Architecture Overview

`agentd` keeps the control state durable in Temporal, while execution stays on nodes and inside supervisors.

The core split is:

- Control plane: HTTP API, execution-plan compilation, Temporal workflows/activities, scheduler, registry sync, metadata stores, SSE fanout.
- Execution plane: node runner, sandbox/container abstraction, per-container supervisor, ACP agent subprocesses.

The important boundary is that northbound API payloads do not drive execution directly. The API resolves the agent, normalizes the bundle, applies policy and container profile, and compiles one `ExecutionPlan` before anything reaches the node.

The execution plane is currently a local sandbox abstraction backed by a node daemon and an in-process Unix-socket supervisor. The interfaces are already shaped for a real container backend later.

```mermaid
flowchart LR
    Client["Client"] --> API["agentd-api<br/>HTTP API + SSE"]
    API --> Plan["Compile ExecutionPlan<br/>request + agent + bundle + policy + profile"]
    Plan --> WF["Temporal Workflows<br/>ContainerWorkflow / SessionWorkflow"]
    WF --> Act["Temporal Activities"]

    Registry["Registry Store<br/>ACP catalog"] --> Plan
    Bundle["Bundle Resolver<br/>binary / npx / uvx"] --> Plan
    Config["Immutable Config Snapshot"] --> API
    Config --> Plan

    Act --> Meta["Metadata Store<br/>Postgres or in-memory"]
    Act --> Sched["Scheduler"]
    Sched --> Node["agentd-node<br/>gRPC"]
    Act --> Node

    Node --> Runner["Local Runtime / Sandbox Handle"]
    Runner --> Supervisor["acp-supervisor<br/>Unix socket gRPC"]
    Supervisor --> ACP["ACP stdio JSON-RPC"]
    ACP --> Agents["Multiple ACP agent subprocesses"]

    Supervisor --> Stream["Prompt stream events"]
    Stream --> Broker["Redis or in-memory broker"]
    Broker --> API
    API --> Client

    Act --> Artifacts["Artifact Store<br/>filesystem now, S3/OCI later"]
```

The main request flow is:

1. A client calls the northbound API on [`cmd/agentd-api/main.go`](/cmd/agentd-api/main.go), typically `POST /v1/containers`, `POST /v1/sessions`, `POST /v1/sessions/{id}:prompt`, or `POST /v1/sessions/{id}:cancel`.
2. [`api/server.go`](/api/server.go) validates the request, resolves a registry or manual agent, resolves a normalized bundle, and compiles an [`ExecutionPlan`](/control/types.go).
3. The API uses the Temporal client in [`workflows/client.go`](/workflows/client.go) to start or update long-lived container and session workflows.
4. [`workflows/workflows.go`](/workflows/workflows.go) keeps the durable lifecycle state and delegates side effects to activities.
5. [`workflows/activities.go`](/workflows/activities.go) writes metadata, asks the scheduler for placement when needed, and calls the selected node over gRPC.
6. [`cmd/agentd-node/main.go`](/cmd/agentd-node/main.go) exposes the node service. [`runtime/local/runner.go`](/runtime/local/runner.go) provisions the sandbox handle and binds one supervisor per container.
7. [`runtime/supervisor/manager.go`](/runtime/supervisor/manager.go) installs or reuses agents, negotiates ACP with each subprocess, and manages multiple agents and sessions inside the same container boundary.
8. Prompt events stream from supervisor to node gRPC stream to the session broker, then to SSE clients. Temporal stores run and session state transitions, but not token-by-token stream history.

This keeps the hot path small:

- execution happens over gRPC and ACP stdio streams, not through database polling
- SSE fanout uses Redis or in-memory pubsub, with no SQL in the live stream path
- Temporal stores durable control decisions and recovery points, not every output chunk
- container and session transitions stay explicit and idempotent

## Core Lifecycle Model

State machines are explicit and idempotent.

- Container: `requested -> provisioning -> starting -> ready -> busy -> idle -> hibernating -> stopped -> deleting -> deleted`
- Agent instance: `installing -> installed -> starting -> ready -> auth_required -> running -> exited -> failed`
- Session: `creating -> active -> waiting_input -> streaming -> cancelling -> completed -> failed -> archived`
- Run: `queued -> dispatching -> streaming -> completed/cancelled/failed`

Temporal workflows:

- `RegistrySyncWorkflow`
- `BundleWorkflow`
- `ContainerWorkflow`
- `SessionWorkflow`
- `GCWorkflow`

Mutating commands use Workflow Updates. Token-by-token stream events are not written into Temporal history.

## Local Development

### Prerequisites

- Go 1.25+ for this module as checked into `go.mod`.
- Docker only for the backing services in `deploy/docker-compose.yml`.
- A running Temporal server.

### Start Backing Services

```bash
cd deploy
docker compose up -d
```

This starts:

- `temporal`: Temporalite on `127.0.0.1:7233`
- `postgres`: app metadata on `127.0.0.1:5432`
- `redis`: event fanout on `127.0.0.1:6379`

### Build the Example ACP Agent

```bash
go build -o ./var/echo-agent ./examples/echo-agent
```

### Run the Node

```bash
AGENTD_NODE_ID=node-1 \
AGENTD_NODE_LISTEN=:9091 \
AGENTD_NODE_ROOT=./var/node \
go run ./cmd/agentd-node
```

### Run the Worker

```bash
DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5432/agentd?sslmode=disable \
REDIS_URL=redis://127.0.0.1:6379/0 \
TEMPORAL_ADDR=127.0.0.1:7233 \
TEMPORAL_NAMESPACE=default \
TEMPORAL_TASK_QUEUE=agentd \
AGENTD_NODE_ENDPOINTS=node-1=127.0.0.1:9091 \
go run ./cmd/agentd-worker
```

### Run the API

```bash
DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5432/agentd?sslmode=disable \
REDIS_URL=redis://127.0.0.1:6379/0 \
TEMPORAL_ADDR=127.0.0.1:7233 \
TEMPORAL_NAMESPACE=default \
TEMPORAL_TASK_QUEUE=agentd \
AGENTD_NODE_ENDPOINTS=node-1=127.0.0.1:9091 \
AGENTD_REGISTRY_SOURCE=./examples/registry.json \
AGENTD_HTTP_ADDR=:8080 \
go run ./cmd/agentd-api
```

### Smoke Test

Create a container explicitly:

```bash
curl -s http://127.0.0.1:8080/v1/containers \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"profile":{"node_id":"node-1"},"capacity":4}'
```

Create a session and let the scheduler create/pick the container if needed. This example uses a manual binary agent that points at the built echo agent:

```bash
curl -s http://127.0.0.1:8080/v1/sessions \
  -X POST \
  -H 'content-type: application/json' \
  -d @- <<'JSON'
{
  "manual_agent": {
    "id": "echo-local",
    "version": "1.0.0",
    "source": "manual",
    "protocol": { "auth_modes": ["agent"] },
    "distribution": {
      "type": "binary",
      "binary": {
        "url": "https://example.invalid/echo-agent",
        "executable": "./var/echo-agent"
      }
    }
  },
  "container_profile": {
    "node_id": "node-1"
  },
  "session": {
    "working_dir": "."
  }
}
JSON
```

Prompt the session:

```bash
curl -s http://127.0.0.1:8080/v1/sessions/<session-id>:prompt \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"prompt":"hello"}'
```

Tail SSE events:

```bash
curl -N http://127.0.0.1:8080/v1/sessions/<session-id>/events
```

Cancel a run:

```bash
curl -s http://127.0.0.1:8080/v1/sessions/<session-id>:cancel \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"run_id":"<run-id>"}'
```

## Docker Dev Mode

`deploy/docker-compose.yml` is intentionally small and only brings up the backing services. Run the Go binaries locally, or build your own container images around the four commands.

For a full multi-process dev loop use:

1. `docker compose up -d` in `deploy/`
2. `go run ./cmd/agentd-node`
3. `go run ./cmd/agentd-worker`
4. `go run ./cmd/agentd-api`

If you skip Postgres or Redis, the in-memory fallbacks still work, but only inside a single process. Separate API and worker processes should use both services.

## Helm

A Helm chart lives in [`deploy/charts/agentd/`](/deploy/charts/agentd).

The repo also includes a GHCR image workflow in [`.github/workflows/container.yml`](/.github/workflows/container.yml). It builds the multi-binary `agentd` image from [`Dockerfile`](/Dockerfile) and publishes:

- `ghcr.io/<owner>/agentd:sha-<commit>`
- `ghcr.io/<owner>/agentd:latest` on the default branch

Render it locally:

```bash
helm template agentd ./deploy/charts/agentd \
  --set image.repository=ghcr.io/your-org/agentd \
  --set image.tag=latest \
  --set database.url='postgres://postgres:postgres@postgres:5432/agentd?sslmode=disable' \
  --set redis.url='redis://redis:6379/0' \
  --set temporal.address='temporal-frontend:7233'
```

Install it:

```bash
helm upgrade --install agentd ./deploy/charts/agentd \
  --namespace agentd \
  --create-namespace \
  --set image.repository=ghcr.io/your-org/agentd \
  --set image.tag=latest \
  --set database.url='postgres://postgres:postgres@postgres:5432/agentd?sslmode=disable' \
  --set redis.url='redis://redis:6379/0' \
  --set temporal.address='temporal-frontend:7233'
```

The chart is intentionally small:

- `api` and `worker` default to enabled
- `node` defaults to a single in-cluster node runner exposed by a Service
- `node.endpointsOverride` lets you point the control plane at external or separately deployed node runners

For a real multi-pod deployment, set both `database.url` and `redis.url`. Leaving either unset falls back to in-memory stores inside each pod, which is only useful for local experimentation.

## Tests

```bash
go test ./...
```

Covered areas:

- registry parsing
- bundle resolution
- execution-plan compilation
- config snapshot reload concurrency
- SSE and HTTP smoke tests in `test/e2e/`
- local node/supervisor runtime with a real ACP subprocess in `test/e2e/`
- Temporal workflow update lifecycle tests in `test/unit/`
- prompt dispatch benchmark

## Adding a New Distribution Type

1. Extend [`control/types.go`](/control/types.go) with the new `DistributionType` payload.
2. Update [`registry/parser.go`](/registry/parser.go) validation.
3. Add resolution logic in [`bundle/resolver.go`](/bundle/resolver.go).
4. If the new type needs materialization or install-time behavior, extend the node/supervisor path in [`runtime/`](/runtime).
5. Add unit tests in [`test/unit/bundle/`](/test/unit/bundle) and an end-to-end runtime test in [`test/e2e/`](/test/e2e) if it changes subprocess startup.

The intended rule is: registry payloads stay declarative, bundle resolution normalizes them into a content-addressed bundle, and execution only consumes the normalized bundle.
