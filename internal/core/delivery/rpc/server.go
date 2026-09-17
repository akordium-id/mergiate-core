package rpc

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
	"github.com/akordium-id/mergiate-core/gen/mergiate/v1/mergiatev1connect"
	"github.com/akordium-id/mergiate-core/internal/core/delivery/rpc/interceptors"
	"github.com/akordium-id/mergiate-core/internal/core/domain/identity"
	partyuc "github.com/akordium-id/mergiate-core/internal/core/usecase/party"
	"github.com/akordium-id/mergiate-core/pkg/auth"
)

// Server holds both ConnectRPC handlers and pure gRPC server configuration.
type Server struct {
	authHandler  *interceptors.AuthHandler
	pingServer   *PingServiceServer
	partyServer  *PartyServiceServer
	healthServer *health.Server
}

// Config provides dependencies for setting up RPC services.
type Config struct {
	TokenManager    auth.TokenManager
	ApiKeyValidator identity.APIKeyValidator
	PartyUsecase    partyuc.Usecase
	AppVersion      string
}

// NewServer initializes RPC services and interceptors.
func NewServer(cfg Config) *Server {
	authHandler := interceptors.NewAuthHandler(cfg.TokenManager, cfg.ApiKeyValidator)
	pingServer := NewPingServiceServer(cfg.AppVersion)
	partyServer := NewPartyServiceServer(cfg.PartyUsecase)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	return &Server{
		authHandler:  authHandler,
		pingServer:   pingServer,
		partyServer:  partyServer,
		healthServer: healthServer,
	}
}

// MountConnectRPC mounts ConnectRPC handlers directly onto a Chi router.
// This allows ConnectRPC (and gRPC over HTTP) to be served seamlessly on the same port as HTTP REST.
func (s *Server) MountConnectRPC(r chi.Router) {
	connectInterceptor := connect.WithInterceptors(s.authHandler.ConnectInterceptor())

	// 1. Health checker for ConnectRPC / gRPC-health
	checker := grpchealth.NewStaticChecker()
	r.Mount(grpchealth.NewHandler(checker))

	// 2. Ping Service
	pingPath, pingHandler := mergiatev1connect.NewPingServiceHandler(s.pingServer, connectInterceptor)
	r.Mount(pingPath, pingHandler)

	// 3. Party Service
	partyPath, partyHandler := mergiatev1connect.NewPartyServiceHandler(s.partyServer, connectInterceptor)
	r.Mount(partyPath, partyHandler)
}

// BuildGRPCServer builds a standalone standard gRPC server.
func (s *Server) BuildGRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		s.authHandler.GRPCUnaryInterceptor(),
	}

	serverOpts := append([]grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
	}, opts...)

	grpcServer := grpc.NewServer(serverOpts...)

	// Register gRPC Health service
	grpc_health_v1.RegisterHealthServer(grpcServer, s.healthServer)

	// Register business services
	mergiatev1.RegisterPingServiceServer(grpcServer, s.pingServer.GRPCServer())
	mergiatev1.RegisterPartyServiceServer(grpcServer, s.partyServer.GRPCServer())

	return grpcServer
}

// Handler returns an http.Handler that can be used independently (e.g. in tests).
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	s.MountConnectRPC(r)
	return r
}
