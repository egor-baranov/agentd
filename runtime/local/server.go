package local

import (
	"context"

	"agentd/control"
	"agentd/runtime/rpc"
	"agentd/session"
)

type NodeServer struct{ Runner *Runner }

func (s NodeServer) ProvisionContainer(ctx context.Context, req *control.CreateContainerRequest) (*rpc.ContainerResponse, error) {
	container, err := s.Runner.ProvisionContainer(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &rpc.ContainerResponse{Container: container}, nil
}

func (s NodeServer) GetContainer(ctx context.Context, req *rpc.IDRequest) (*rpc.ContainerSnapshotResponse, error) {
	snapshot, err := s.Runner.GetContainer(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &rpc.ContainerSnapshotResponse{Snapshot: snapshot}, nil
}

func (s NodeServer) HibernateContainer(ctx context.Context, req *rpc.IDRequest) (*rpc.ContainerResponse, error) {
	container, err := s.Runner.HibernateContainer(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &rpc.ContainerResponse{Container: container}, nil
}

func (s NodeServer) DeleteContainer(ctx context.Context, req *rpc.IDRequest) (*rpc.Empty, error) {
	return &rpc.Empty{}, s.Runner.DeleteContainer(ctx, req.ID)
}

func (s NodeServer) EnsureAgent(ctx context.Context, req *control.InstallAgentRequest) (*rpc.AgentResponse, error) {
	agent, err := s.Runner.EnsureAgent(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &rpc.AgentResponse{Agent: agent}, nil
}

func (s NodeServer) CreateSession(ctx context.Context, req *control.CreateSessionRequest) (*rpc.SessionResponse, error) {
	session, err := s.Runner.CreateSession(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &rpc.SessionResponse{Session: session}, nil
}

func (s NodeServer) LoadSession(ctx context.Context, req *control.LoadSessionRequest) (*rpc.SessionResponse, error) {
	session, err := s.Runner.LoadSession(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &rpc.SessionResponse{Session: session}, nil
}

func (s NodeServer) Prompt(req *control.PromptRequest, stream rpc.Node_PromptServer) error {
	run, err := s.Runner.Prompt(stream.Context(), *req, func(event session.Event) error {
		return stream.Send(&rpc.PromptStreamItem{Event: event, Final: false})
	})
	if err != nil {
		return err
	}
	return stream.Send(&rpc.PromptStreamItem{Run: run, Final: true})
}

func (s NodeServer) Cancel(ctx context.Context, req *control.CancelRequest) (*rpc.Empty, error) {
	return &rpc.Empty{}, s.Runner.Cancel(ctx, *req)
}
