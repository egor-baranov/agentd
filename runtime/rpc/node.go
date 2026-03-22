package rpc

import (
	"context"

	"agentd/control"
	"google.golang.org/grpc"
)

const nodeServiceName = "agentd.runtime.Node"

type NodeServer interface {
	ProvisionContainer(context.Context, *control.CreateContainerRequest) (*ContainerResponse, error)
	GetContainer(context.Context, *IDRequest) (*ContainerSnapshotResponse, error)
	HibernateContainer(context.Context, *IDRequest) (*ContainerResponse, error)
	DeleteContainer(context.Context, *IDRequest) (*Empty, error)
	EnsureAgent(context.Context, *control.InstallAgentRequest) (*AgentResponse, error)
	CreateSession(context.Context, *control.CreateSessionRequest) (*SessionResponse, error)
	LoadSession(context.Context, *control.LoadSessionRequest) (*SessionResponse, error)
	Prompt(*control.PromptRequest, Node_PromptServer) error
	Cancel(context.Context, *control.CancelRequest) (*Empty, error)
}

type Node_PromptServer interface {
	Send(*PromptStreamItem) error
	grpc.ServerStream
}

type Node_PromptClient interface {
	Recv() (*PromptStreamItem, error)
	grpc.ClientStream
}

type NodeClient struct{ cc *grpc.ClientConn }

func NewNodeClient(cc *grpc.ClientConn) *NodeClient { return &NodeClient{cc: cc} }

func (c *NodeClient) ProvisionContainer(ctx context.Context, req *control.CreateContainerRequest) (*ContainerResponse, error) {
	out := new(ContainerResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/ProvisionContainer", req, out)
}

func (c *NodeClient) GetContainer(ctx context.Context, req *IDRequest) (*ContainerSnapshotResponse, error) {
	out := new(ContainerSnapshotResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/GetContainer", req, out)
}

func (c *NodeClient) HibernateContainer(ctx context.Context, req *IDRequest) (*ContainerResponse, error) {
	out := new(ContainerResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/HibernateContainer", req, out)
}

func (c *NodeClient) DeleteContainer(ctx context.Context, req *IDRequest) (*Empty, error) {
	out := new(Empty)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/DeleteContainer", req, out)
}

func (c *NodeClient) EnsureAgent(ctx context.Context, req *control.InstallAgentRequest) (*AgentResponse, error) {
	out := new(AgentResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/EnsureAgent", req, out)
}

func (c *NodeClient) CreateSession(ctx context.Context, req *control.CreateSessionRequest) (*SessionResponse, error) {
	out := new(SessionResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/CreateSession", req, out)
}

func (c *NodeClient) LoadSession(ctx context.Context, req *control.LoadSessionRequest) (*SessionResponse, error) {
	out := new(SessionResponse)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/LoadSession", req, out)
}

func (c *NodeClient) Prompt(ctx context.Context, req *control.PromptRequest) (Node_PromptClient, error) {
	stream, err := c.cc.NewStream(ctx, &nodeServiceDesc.Streams[0], "/"+nodeServiceName+"/Prompt")
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	return &nodePromptClient{ClientStream: stream}, nil
}

func (c *NodeClient) Cancel(ctx context.Context, req *control.CancelRequest) (*Empty, error) {
	out := new(Empty)
	return out, c.cc.Invoke(ctx, "/"+nodeServiceName+"/Cancel", req, out)
}

type nodePromptClient struct{ grpc.ClientStream }

func (c *nodePromptClient) Recv() (*PromptStreamItem, error) {
	item := new(PromptStreamItem)
	if err := c.ClientStream.RecvMsg(item); err != nil {
		return nil, err
	}
	return item, nil
}

func RegisterNodeServer(s *grpc.Server, srv NodeServer) { s.RegisterService(&nodeServiceDesc, srv) }

var nodeServiceDesc = grpc.ServiceDesc{
	ServiceName: nodeServiceName,
	HandlerType: (*NodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "ProvisionContainer", Handler: wrapUnary(func(ctx context.Context, req *control.CreateContainerRequest, srv NodeServer) (*ContainerResponse, error) {
			return srv.ProvisionContainer(ctx, req)
		})},
		{MethodName: "GetContainer", Handler: wrapUnary(func(ctx context.Context, req *IDRequest, srv NodeServer) (*ContainerSnapshotResponse, error) {
			return srv.GetContainer(ctx, req)
		})},
		{MethodName: "HibernateContainer", Handler: wrapUnary(func(ctx context.Context, req *IDRequest, srv NodeServer) (*ContainerResponse, error) {
			return srv.HibernateContainer(ctx, req)
		})},
		{MethodName: "DeleteContainer", Handler: wrapUnary(func(ctx context.Context, req *IDRequest, srv NodeServer) (*Empty, error) {
			return srv.DeleteContainer(ctx, req)
		})},
		{MethodName: "EnsureAgent", Handler: wrapUnary(func(ctx context.Context, req *control.InstallAgentRequest, srv NodeServer) (*AgentResponse, error) {
			return srv.EnsureAgent(ctx, req)
		})},
		{MethodName: "CreateSession", Handler: wrapUnary(func(ctx context.Context, req *control.CreateSessionRequest, srv NodeServer) (*SessionResponse, error) {
			return srv.CreateSession(ctx, req)
		})},
		{MethodName: "LoadSession", Handler: wrapUnary(func(ctx context.Context, req *control.LoadSessionRequest, srv NodeServer) (*SessionResponse, error) {
			return srv.LoadSession(ctx, req)
		})},
		{MethodName: "Cancel", Handler: wrapUnary(func(ctx context.Context, req *control.CancelRequest, srv NodeServer) (*Empty, error) {
			return srv.Cancel(ctx, req)
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
			return srv.(NodeServer).Prompt(req, &nodePromptServer{stream})
		},
	}},
}

type nodePromptServer struct{ grpc.ServerStream }

func (s *nodePromptServer) Send(item *PromptStreamItem) error { return s.ServerStream.SendMsg(item) }
