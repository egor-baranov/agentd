package runtime

import (
	"context"
	"fmt"

	"agentd/control"
	"agentd/runtime/rpc"
	"agentd/session"
	"google.golang.org/grpc"
)

type RemoteNodeClient struct {
	conn   *grpc.ClientConn
	client *rpc.NodeClient
}

func DialNode(ctx context.Context, target string) (*RemoteNodeClient, error) {
	conn, err := rpc.DialContext(ctx, target)
	if err != nil {
		return nil, err
	}
	return &RemoteNodeClient{conn: conn, client: rpc.NewNodeClient(conn)}, nil
}

func (c *RemoteNodeClient) Close() error { return c.conn.Close() }

func (c *RemoteNodeClient) ProvisionContainer(ctx context.Context, req control.CreateContainerRequest) (control.Container, error) {
	resp, err := c.client.ProvisionContainer(ctx, &req)
	if err != nil {
		return control.Container{}, err
	}
	return resp.Container, nil
}

func (c *RemoteNodeClient) GetContainer(ctx context.Context, containerID string) (control.ContainerSnapshot, error) {
	resp, err := c.client.GetContainer(ctx, &rpc.IDRequest{ID: containerID})
	if err != nil {
		return control.ContainerSnapshot{}, err
	}
	return resp.Snapshot, nil
}

func (c *RemoteNodeClient) HibernateContainer(ctx context.Context, containerID string) (control.Container, error) {
	resp, err := c.client.HibernateContainer(ctx, &rpc.IDRequest{ID: containerID})
	if err != nil {
		return control.Container{}, err
	}
	return resp.Container, nil
}

func (c *RemoteNodeClient) DeleteContainer(ctx context.Context, containerID string) error {
	_, err := c.client.DeleteContainer(ctx, &rpc.IDRequest{ID: containerID})
	return err
}

func (c *RemoteNodeClient) EnsureAgent(ctx context.Context, req control.InstallAgentRequest) (control.AgentInstance, error) {
	resp, err := c.client.EnsureAgent(ctx, &req)
	if err != nil {
		return control.AgentInstance{}, err
	}
	return resp.Agent, nil
}

func (c *RemoteNodeClient) CreateSession(ctx context.Context, req control.CreateSessionRequest) (control.Session, error) {
	resp, err := c.client.CreateSession(ctx, &req)
	if err != nil {
		return control.Session{}, err
	}
	return resp.Session, nil
}

func (c *RemoteNodeClient) LoadSession(ctx context.Context, req control.LoadSessionRequest) (control.Session, error) {
	resp, err := c.client.LoadSession(ctx, &req)
	if err != nil {
		return control.Session{}, err
	}
	return resp.Session, nil
}

func (c *RemoteNodeClient) Prompt(ctx context.Context, req control.PromptRequest, publish func(session.Event) error) (control.Run, error) {
	stream, err := c.client.Prompt(ctx, &req)
	if err != nil {
		return control.Run{}, err
	}
	var final control.Run
	for {
		item, err := stream.Recv()
		if err != nil {
			if final.ID != "" {
				return final, nil
			}
			return control.Run{}, err
		}
		if publish != nil && item.Event.Type != "" {
			if err := publish(item.Event); err != nil {
				return control.Run{}, err
			}
		}
		if item.Error != "" && item.Final && item.Run.ID == "" {
			return control.Run{}, fmt.Errorf("%s", item.Error)
		}
		if item.Run.ID != "" {
			final = item.Run
		}
		if item.Final {
			return final, nil
		}
	}
}

func (c *RemoteNodeClient) Cancel(ctx context.Context, req control.CancelRequest) error {
	_, err := c.client.Cancel(ctx, &req)
	return err
}
