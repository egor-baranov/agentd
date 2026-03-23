package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"agentd/api"
	"agentd/observer"
	"agentd/registry"
	runtimeclient "agentd/runtime"
	"agentd/scheduler"
	"agentd/session"
	"agentd/store"
	"agentd/workflows"
	temporalclient "go.temporal.io/sdk/client"
)

func main() {
	ctx := context.Background()
	logger := observer.NewLogger("agentd-api")
	metrics := observer.NewMetrics()
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
	if source := os.Getenv("AGENTD_REGISTRY_SOURCE"); source != "" {
		if _, err := (registry.Syncer{Store: meta.Registry}).Sync(ctx, source); err != nil {
			logger.Error("sync registry", "error", err)
		}
	}
	tc, err := temporalclient.Dial(temporalclient.Options{HostPort: getenv("TEMPORAL_ADDR", "127.0.0.1:7233"), Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
	if err != nil {
		logger.Error("dial temporal", "error", err)
		os.Exit(1)
	}
	defer tc.Close()
	server := &api.Server{
		Logger:     logger,
		Metrics:    metrics,
		Temporal:   workflows.Client{Raw: tc, TaskQueue: getenv("TEMPORAL_TASK_QUEUE", workflows.TaskQueueDefault)},
		Registry:   meta.Registry,
		Bundles:    meta.Bundles,
		Containers: meta.Containers,
		Sessions:   meta.Sessions,
		Scheduler:  &scheduler.RoundRobin{Containers: meta.Containers, Nodes: runtimeclient.NodeIDs(nodes)},
		Broker:     openedBroker.Broker,
	}
	httpServer := &http.Server{Addr: getenv("AGENTD_HTTP_ADDR", ":8080"), Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
	logger.Info("api listening", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
