package rpc

import (
	"context"

	"google.golang.org/grpc"
)

func wrapUnary[Req any, Resp any, S any](fn func(context.Context, *Req, S) (*Resp, error)) grpc.MethodHandler {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(Req)
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return fn(ctx, req, srv.(S))
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: ""}
		handler := func(ctx context.Context, reqAny any) (any, error) {
			return fn(ctx, reqAny.(*Req), srv.(S))
		}
		return interceptor(ctx, req, info, handler)
	}
}
