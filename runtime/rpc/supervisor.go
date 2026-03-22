package rpc

import (
	"context"

	"agentd/control"
	"google.golang.org/grpc"
)

const supervisorServiceName = "agentd.runtime.Supervisor"

type SupervisorServer interface {
	EnsureAgent(context.Context, *control.InstallAgentRequest) (*AgentResponse, error)
	CreateSession(context.Context, *control.CreateSessionRequest) (*SessionResponse, error)
	LoadSession(context.Context, *control.LoadSessionRequest) (*SessionResponse, error)
	Prompt(*control.PromptRequest, Supervisor_PromptServer) error
	Cancel(context.Context, *control.CancelRequest) (*Empty, error)
	ListAgents(context.Context, *Empty) (*ListAgentsResponse, error)
	ListSessions(context.Context, *ListSessionsRequest) (*ListSessionsResponse, error)
}

type Supervisor_PromptServer interface {
	Send(*PromptStreamItem) error
	grpc.ServerStream
}

type Supervisor_PromptClient interface {
	Recv() (*PromptStreamItem, error)
	grpc.ClientStream
}

type SupervisorClient struct {
	cc *grpc.ClientConn
}

func NewSupervisorClient(cc *grpc.ClientConn) *SupervisorClient { return &SupervisorClient{cc: cc} }

func (c *SupervisorClient) EnsureAgent(ctx context.Context, req *control.InstallAgentRequest) (*AgentResponse, error) {
	out := new(AgentResponse)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/EnsureAgent", req, out)
}

func (c *SupervisorClient) CreateSession(ctx context.Context, req *control.CreateSessionRequest) (*SessionResponse, error) {
	out := new(SessionResponse)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/CreateSession", req, out)
}

func (c *SupervisorClient) LoadSession(ctx context.Context, req *control.LoadSessionRequest) (*SessionResponse, error) {
	out := new(SessionResponse)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/LoadSession", req, out)
}

func (c *SupervisorClient) Prompt(ctx context.Context, req *control.PromptRequest) (Supervisor_PromptClient, error) {
	stream, err := c.cc.NewStream(ctx, &supervisorServiceDesc.Streams[0], "/"+supervisorServiceName+"/Prompt")
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	return &supervisorPromptClient{ClientStream: stream}, nil
}

func (c *SupervisorClient) Cancel(ctx context.Context, req *control.CancelRequest) (*Empty, error) {
	out := new(Empty)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/Cancel", req, out)
}

func (c *SupervisorClient) ListAgents(ctx context.Context, req *Empty) (*ListAgentsResponse, error) {
	out := new(ListAgentsResponse)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/ListAgents", req, out)
}

func (c *SupervisorClient) ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error) {
	out := new(ListSessionsResponse)
	return out, c.cc.Invoke(ctx, "/"+supervisorServiceName+"/ListSessions", req, out)
}

type supervisorPromptClient struct{ grpc.ClientStream }

func (c *supervisorPromptClient) Recv() (*PromptStreamItem, error) {
	item := new(PromptStreamItem)
	if err := c.ClientStream.RecvMsg(item); err != nil {
		return nil, err
	}
	return item, nil
}

func RegisterSupervisorServer(s *grpc.Server, srv SupervisorServer) {
	s.RegisterService(&supervisorServiceDesc, srv)
}

var supervisorServiceDesc = grpc.ServiceDesc{
	ServiceName: supervisorServiceName,
	HandlerType: (*SupervisorServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "EnsureAgent", Handler: wrapUnary(func(ctx context.Context, req *control.InstallAgentRequest, srv SupervisorServer) (*AgentResponse, error) {
			return srv.EnsureAgent(ctx, req)
		})},
		{MethodName: "CreateSession", Handler: wrapUnary(func(ctx context.Context, req *control.CreateSessionRequest, srv SupervisorServer) (*SessionResponse, error) {
			return srv.CreateSession(ctx, req)
		})},
		{MethodName: "LoadSession", Handler: wrapUnary(func(ctx context.Context, req *control.LoadSessionRequest, srv SupervisorServer) (*SessionResponse, error) {
			return srv.LoadSession(ctx, req)
		})},
		{MethodName: "Cancel", Handler: wrapUnary(func(ctx context.Context, req *control.CancelRequest, srv SupervisorServer) (*Empty, error) {
			return srv.Cancel(ctx, req)
		})},
		{MethodName: "ListAgents", Handler: wrapUnary(func(ctx context.Context, req *Empty, srv SupervisorServer) (*ListAgentsResponse, error) {
			return srv.ListAgents(ctx, req)
		})},
		{MethodName: "ListSessions", Handler: wrapUnary(func(ctx context.Context, req *ListSessionsRequest, srv SupervisorServer) (*ListSessionsResponse, error) {
			return srv.ListSessions(ctx, req)
		})},
	},
	Streams: []grpc.StreamDesc{{
		StreamName:    "Prompt",
		ServerStreams: true,
		Handler: func(srv any, stream grpc.ServerStream) error {
			req := new(control.PromptRequest)
			if err := stream.RecvMsg(req); err != nil {
				return err
			}
			return srv.(SupervisorServer).Prompt(req, &supervisorPromptServer{stream})
		},
	}},
}

type supervisorPromptServer struct{ grpc.ServerStream }

func (s *supervisorPromptServer) Send(item *PromptStreamItem) error {
	return s.ServerStream.SendMsg(item)
}
