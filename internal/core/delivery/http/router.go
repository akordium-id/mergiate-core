package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	v1 "github.com/akordium-id/mergiate-core/internal/core/delivery/http/v1"
	"github.com/akordium-id/mergiate-core/internal/core/domain/identity"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/module"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type Handlers struct {
	TenantHandler         *v1.TenantHandler
	OrganizationHandler   *v1.OrganizationHandler
	PartyHandler          *v1.PartyHandler
	ProductHandler        *v1.ProductHandler
	DocumentHandler       *v1.DocumentHandler
	AuditHandler          *v1.AuditHandler
	AuthHandler           *v1.AuthHandler
	IdentityHandler       *v1.IdentityHandler
	CustomFieldHandler    *v1.CustomFieldHandler
	SequenceHandler       *v1.SequenceHandler
	AttachmentHandler     *v1.AttachmentHandler
	CommunicationHandler  *v1.CommunicationHandler
	ServiceAccountHandler *v1.ServiceAccountHandler
	ModuleRegistry        *module.Registry
	TokenManager          auth.TokenManager
	ApiKeyValidator       identity.APIKeyValidator
	// CORSAllowedOrigins is the list of allowed origins for CORS.
	// Empty slice = no cross-origin access (same-origin / reverse-proxy deployments).
	// Wildcard "*" is NOT accepted when credentials are in use — pass explicit origins only.
	CORSAllowedOrigins []string
}

// NewRouter constructs the Chi router with middleware and routes.
func NewRouter(db *pgxpool.Pool, handlers Handlers) http.Handler {
	r := chi.NewRouter()

	// Global Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.RequestLogger())
	r.Use(chimiddleware.Recoverer)

	// CORS — never combine wildcard origins with AllowCredentials (invalid per spec).
	// Wildcard entries are filtered out to prevent misconfiguration.
	safeOrigins := filterWildcardOrigins(handlers.CORSAllowedOrigins)
	if len(safeOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   safeOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key", middleware.HeaderTenantID},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	} else {
		// No explicit origins: allow no cross-origin access.
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key", middleware.HeaderTenantID},
			AllowCredentials: false,
		}))
	}

	// Health Checks (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			response.Err(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database connection ping failed")
			return
		}

		response.JSON(w, http.StatusOK, map[string]string{
			"status":   "ready",
			"database": "connected",
		})
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes (register, login) — no auth required.
		// /auth/me and /auth/switch-tenant are protected inside AuthHandler.
		if handlers.AuthHandler != nil {
			handlers.AuthHandler.RegisterRoutes(r)
		}

		// All remaining routes require authentication.
		r.Group(func(r chi.Router) {
			if handlers.TokenManager != nil {
				r.Use(middleware.AuthRequired(handlers.TokenManager, handlers.ApiKeyValidator))
			}

			if handlers.IdentityHandler != nil {
				handlers.IdentityHandler.RegisterRoutes(r)
			}
			if handlers.CustomFieldHandler != nil {
				handlers.CustomFieldHandler.RegisterRoutes(r)
			}
			if handlers.SequenceHandler != nil {
				handlers.SequenceHandler.RegisterRoutes(r)
			}
			if handlers.AttachmentHandler != nil {
				handlers.AttachmentHandler.RegisterRoutes(r)
			}
			if handlers.CommunicationHandler != nil {
				handlers.CommunicationHandler.RegisterRoutes(r)
			}
			if handlers.ServiceAccountHandler != nil {
				handlers.ServiceAccountHandler.RegisterRoutes(r)
			}
			if handlers.ModuleRegistry != nil {
				// Module routes are also behind AuthRequired.
				handlers.ModuleRegistry.MountRoutes(r)
			}
			handlers.TenantHandler.RegisterRoutes(r)
			handlers.OrganizationHandler.RegisterRoutes(r)
			handlers.PartyHandler.RegisterRoutes(r)
			handlers.ProductHandler.RegisterRoutes(r)
			handlers.DocumentHandler.RegisterRoutes(r)
			handlers.AuditHandler.RegisterRoutes(r)
		})
	})

	return r
}

// filterWildcardOrigins removes wildcard "*" entries from the origins list to prevent
// combining AllowCredentials with a wildcard origin (invalid per CORS spec).
func filterWildcardOrigins(origins []string) []string {
	filtered := make([]string, 0, len(origins))
	for _, o := range origins {
		if strings.TrimSpace(o) != "*" {
			filtered = append(filtered, o)
		}
	}
	return filtered
}
