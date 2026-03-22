package main

import (
	"context"
	"os"

	"agentd/control"
	"agentd/obs"
	"agentd/registry"
	runtimeclient "agentd/runtime"
	"agentd/session"
	"agentd/store"
	"agentd/workflows"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	ctx := context.Background()
	logger := obs.NewLogger("agentd-worker")
	meta, err := store.Open(ctx, os.Getenv("DATABASE_URL"), getenv("AGENTD_ARTIFACT_DIR", "./var/artifacts"))
	if err != nil {
		logger.Error("open store", "error", err)
		os.Exit(1)
	}
	defer meta.Close()
	openedBroker, err := session.Open(ctx, os.Getenv("REDIS_URL"))
	if err != nil {
		logger.Error("open broker", "error", err)
		os.Exit(1)
	}
	defer openedBroker.Close()
	nodes, err := runtimeclient.ParseNodeEndpoints(os.Getenv("AGENTD_NODE_ENDPOINTS"))
	if err != nil {
		logger.Error("parse nodes", "error", err)
		os.Exit(1)
	}
	nodeClients := map[string]control.NodeClient{}
	for id, addr := range nodes {
		client, err := runtimeclient.DialNode(ctx, addr)
		if err != nil {
			logger.Error("dial node", "node_id", id, "error", err)
			os.Exit(1)
		}
		defer client.Close()
		nodeClients[id] = client
	}
	tc, err := temporalclient.Dial(temporalclient.Options{HostPort: getenv("TEMPORAL_ADDR", "127.0.0.1:7233"), Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
	if err != nil {
		logger.Error("dial temporal", "error", err)
		os.Exit(1)
	}
	defer tc.Close()
	w := worker.New(tc, getenv("TEMPORAL_TASK_QUEUE", workflows.TaskQueueDefault), worker.Options{})
	activities := &workflows.Activities{
		RegistrySyncer: registry.Syncer{Store: meta.Registry},
		RegistryStore:  meta.Registry,
		BundleStore:    meta.Bundles,
		Containers:     meta.Containers,
		Sessions:       meta.Sessions,
		Artifacts:      meta.Artifacts,
		Broker:         openedBroker.Broker,
		Nodes:          nodeClients,
	}
	workflows.Register(w, activities)
	logger.Info("worker starting", "task_queue", getenv("TEMPORAL_TASK_QUEUE", workflows.TaskQueueDefault))
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("worker run", "error", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
