package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"

	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
	"github.com/akordium-id/mergiate-core/gen/mergiate/v1/mergiatev1connect"
)

// PingServiceServer implements mergiatev1.PingServiceServer (gRPC) and mergiatev1connect.PingServiceHandler (ConnectRPC).
type PingServiceServer struct {
	mergiatev1.UnimplementedPingServiceServer
	version string
}

func NewPingServiceServer(version string) *PingServiceServer {
	if version == "" {
		version = "1.0.0"
	}
	return &PingServiceServer{version: version}
}

// ConnectRPC handler
func (s *PingServiceServer) Ping(
	ctx context.Context,
	req *connect.Request[mergiatev1.PingRequest],
) (*connect.Response[mergiatev1.PingResponse], error) {
	msg := req.Msg.GetMessage()
	if msg == "" {
		msg = "pong"
	}

	resp := &mergiatev1.PingResponse{
		Message:   msg,
		Version:   s.version,
		Timestamp: time.Now().Unix(),
	}

	return connect.NewResponse(resp), nil
}

// Pure gRPC handler
func (s *PingServiceServer) PingGRPC(
	ctx context.Context,
	req *mergiatev1.PingRequest,
) (*mergiatev1.PingResponse, error) {
	msg := req.GetMessage()
	if msg == "" {
		msg = "pong"
	}

	return &mergiatev1.PingResponse{
		Message:   msg,
		Version:   s.version,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Implement standard gRPC interface
var _ mergiatev1.PingServiceServer = (*grpcPingAdapter)(nil)

type grpcPingAdapter struct {
	mergiatev1.UnimplementedPingServiceServer
	server *PingServiceServer
}

func (g *grpcPingAdapter) Ping(ctx context.Context, req *mergiatev1.PingRequest) (*mergiatev1.PingResponse, error) {
	return g.server.PingGRPC(ctx, req)
}

func (s *PingServiceServer) GRPCServer() mergiatev1.PingServiceServer {
	return &grpcPingAdapter{server: s}
}

var _ mergiatev1connect.PingServiceHandler = (*PingServiceServer)(nil)
