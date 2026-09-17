package interceptors

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/akordium-id/mergiate-core/internal/core/domain/identity"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/pkg/auth"
)

// AuthHandler authenticates requests from ConnectRPC or gRPC metadata.
type AuthHandler struct {
	tokenMgr    auth.TokenManager
	kv          identity.APIKeyValidator
	publicProcs map[string]bool
}

// NewAuthHandler creates an authenticator for RPC requests.
func NewAuthHandler(tokenMgr auth.TokenManager, kv identity.APIKeyValidator, publicProcs ...string) *AuthHandler {
	pub := make(map[string]bool)
	// Default public RPC procedures
	pub["/mergiate.v1.PingService/Ping"] = true
	pub["/grpc.health.v1.Health/Check"] = true
	pub["/grpc.health.v1.Health/Watch"] = true
	for _, p := range publicProcs {
		pub[p] = true
	}

	return &AuthHandler{
		tokenMgr:    tokenMgr,
		kv:          kv,
		publicProcs: pub,
	}
}

// Authenticate verifies credentials from a token / API key string and tenant context.
func (a *AuthHandler) Authenticate(ctx context.Context, authVal string, apiKeyVal string) (context.Context, error) {
	// 1. Direct API Key header / metadata
	if apiKeyVal != "" && a.kv != nil {
		claims, err := a.kv.ValidateAPIKey(ctx, apiKeyVal, "")
		if err != nil {
			return nil, err
		}
		ctx = shared.WithAuthClaims(ctx, claims)
		if claims.TenantID != shared.NilID() {
			ctx = shared.WithTenantID(ctx, claims.TenantID)
		}
		return ctx, nil
	}

	// 2. Authorization header / metadata
	if authVal == "" {
		return nil, errors.New("missing authorization credentials")
	}

	parts := strings.SplitN(authVal, " ", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed authorization token")
	}

	scheme := parts[0]
	tokenStr := strings.TrimSpace(parts[1])

	// 2a. M2M API key in bearer scheme
	if (strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "ApiKey")) &&
		strings.HasPrefix(tokenStr, identity.APIKeyPrefix) && a.kv != nil {
		claims, err := a.kv.ValidateAPIKey(ctx, tokenStr, "")
		if err != nil {
			return nil, err
		}
		ctx = shared.WithAuthClaims(ctx, claims)
		if claims.TenantID != shared.NilID() {
			ctx = shared.WithTenantID(ctx, claims.TenantID)
		}
		return ctx, nil
	}

	// 2b. JWT token
	if strings.EqualFold(scheme, "Bearer") {
		claims, err := a.tokenMgr.ValidateToken(tokenStr)
		if err != nil {
			return nil, err
		}

		ctx = shared.WithAuthClaims(ctx, &shared.AuthClaims{
			UserID:      claims.UserID,
			TenantID:    claims.TenantID,
			Email:       claims.Email,
			Name:        claims.Name,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
			ActorType:   shared.ActorTypeUser,
		})
		if claims.TenantID != shared.NilID() {
			ctx = shared.WithTenantID(ctx, claims.TenantID)
		}
		return ctx, nil
	}

	return nil, errors.New("unsupported authorization scheme")
}

// ConnectInterceptor returns a connect.UnaryInterceptorFunc for ConnectRPC.
func (a *AuthHandler) ConnectInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			proc := req.Spec().Procedure
			if a.publicProcs[proc] {
				return next(ctx, req)
			}

			authVal := req.Header().Get("Authorization")
			apiKeyVal := req.Header().Get("X-API-Key")

			authCtx, err := a.Authenticate(ctx, authVal, apiKeyVal)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			return next(authCtx, req)
		}
	}
}

// GRPCUnaryInterceptor returns a standard grpc.UnaryServerInterceptor.
func (a *AuthHandler) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if a.publicProcs[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		var authVal, apiKeyVal string
		if vals := md.Get("authorization"); len(vals) > 0 {
			authVal = vals[0]
		}
		if vals := md.Get("x-api-key"); len(vals) > 0 {
			apiKeyVal = vals[0]
		}

		authCtx, err := a.Authenticate(ctx, authVal, apiKeyVal)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		return handler(authCtx, req)
	}
}
