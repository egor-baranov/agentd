package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	localruntime "agentd/runtime/local"
	"agentd/runtime/rpc"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "agentd-node")
	listenAddr := getenv("AGENTD_NODE_LISTEN", ":9091")
	nodeID := getenv("AGENTD_NODE_ID", "node-1")
	root := getenv("AGENTD_NODE_ROOT", "./var/node")
	if err := os.MkdirAll(root, 0o755); err != nil {
		logger.Error("mkdir root", "error", err)
		os.Exit(1)
	}
	runner := localruntime.NewRunner(nodeID, root, logger)
	server := grpc.NewServer()
	rpc.RegisterNodeServer(server, localruntime.NodeServer{Runner: runner})
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}
	go func() {
		logger.Info("node listening", "addr", listenAddr, "node_id", nodeID)
		if err := server.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	server.GracefulStop()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
