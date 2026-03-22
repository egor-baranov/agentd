package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agentd/runtime/rpc"
	"agentd/runtime/supervisor"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "acp-supervisor")
	containerID := getenv("AGENTD_CONTAINER_ID", "dev-container")
	root := getenv("AGENTD_SUPERVISOR_ROOT", "./var/supervisor")
	socket := getenv("AGENTD_SUPERVISOR_SOCKET", supervisor.SupervisorSocket(root))
	if err := os.MkdirAll(root, 0o755); err != nil {
		logger.Error("mkdir root", "error", err)
		os.Exit(1)
	}
	workspace, err := supervisor.EnsureWorkspace(root)
	if err != nil {
		logger.Error("workspace", "error", err)
		os.Exit(1)
	}
	_ = os.Remove(socket)
	lis, err := net.Listen("unix", socket)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}
	server := grpc.NewServer()
	rpc.RegisterSupervisorServer(server, supervisor.NewManager(containerID, workspace, logger))
	go func() {
		logger.Info("supervisor listening", "socket", socket)
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
