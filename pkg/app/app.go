// Package app provides the public assembly API for mergiate-core.
// An external distro binary (e.g. mergiate-erp) can embed core + its own modules in one process:
//
//	opts := app.Options{
//	    Config:        cfg,
//	    Modules:       []module.Module{myERP},
//	    RunMigrations: true,
//	}
//	a, err := app.New(ctx, opts)
//	if err != nil { log.Fatal(err) }
//	if err := a.Start(ctx); err != nil { log.Fatal(err) }
package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	deliveryhttp "github.com/akordium-id/mergiate-core/internal/core/delivery/http"
	v1 "github.com/akordium-id/mergiate-core/internal/core/delivery/http/v1"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres"
	attachmentusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/attachment"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/audit"
	commusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/communication"
	customfieldusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/customfield"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/document"
	identityusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/identity"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/organization"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/party"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/product"
	sequenceusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/sequence"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/tenant"
	webhookuc "github.com/akordium-id/mergiate-core/internal/core/usecase/webhook"
	"github.com/akordium-id/mergiate-core/internal/core/worker"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/config"
	"github.com/akordium-id/mergiate-core/pkg/database"
	"github.com/akordium-id/mergiate-core/pkg/eventbus"
	"github.com/akordium-id/mergiate-core/pkg/migrations"
	"github.com/akordium-id/mergiate-core/pkg/module"
	"github.com/akordium-id/mergiate-core/pkg/ratelimit"
	"github.com/akordium-id/mergiate-core/pkg/storage/local"
)

// Options configures the App assembly.
type Options struct {
	// Config is the application configuration. Required.
	Config *config.Config
	// Logger to use. If nil, slog.Default() is used.
	Logger *slog.Logger
	// Modules are external module implementations registered before router build.
	Modules []module.Module
	// RunMigrations, if true, runs pkg/migrations.Up on app start before serving.
	RunMigrations bool
	// ExtraMigrationFS are module-owned migration sources applied after core's.
	// Expected to start at version 000100 to never collide with core (000001-…).
	ExtraMigrationFS []fs.FS
}

// App is the assembled core application instance.
type App struct {
	cfg              *config.Config
	logger           *slog.Logger
	db               *pgxpool.Pool
	tokenMgr         auth.TokenManager
	moduleHost       module.Host
	moduleRegistry   *module.Registry
	router           chi.Router
	outboxWorker     *worker.OutboxWorker
	webhookDispatcher *worker.WebhookDispatcher
	rateLimiter      ratelimit.Limiter
	workerCancel     context.CancelFunc
}

// New assembles all core repositories, usecases, handlers, and modules.
// It does NOT start serving; call Start to begin serving HTTP traffic.
func New(ctx context.Context, opts Options) (*App, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	cfg := opts.Config

	// DB
	dbPool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}

	// Optional: run migrations before wiring
	if opts.RunMigrations {
		if err := migrations.Up(dbPool, opts.ExtraMigrationFS...); err != nil {
			dbPool.Close()
			return nil, fmt.Errorf("migrations: %w", err)
		}
	}

	// Storage
	storageDriver, err := local.NewDriver(cfg.StoragePath)
	if err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("storage driver: %w", err)
	}

	// Auth
	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, cfg.AppName)

	// Event Bus + Outbox Worker
	bus := eventbus.NewInMemoryBus()
	outboxRepo := postgres.NewOutboxRepository(dbPool)
	outboxWorker := worker.NewOutboxWorker(outboxRepo, bus, worker.DefaultConfig())

	// Repositories
	tenantRepo := postgres.NewTenantRepository(dbPool)
	orgRepo := postgres.NewOrganizationRepository(dbPool)
	partyRepo := postgres.NewPartyRepository(dbPool)
	contactRepo := postgres.NewAddressContactRepository(dbPool)
	unitRepo := postgres.NewUnitRepository(dbPool)
	productRepo := postgres.NewProductRepository(dbPool)
	docRepo := postgres.NewDocumentRepository(dbPool)
	auditRepo := postgres.NewAuditRepository(dbPool)
	identityRepo := postgres.NewIdentityRepository(dbPool)
	customFieldRepo := postgres.NewCustomFieldRepository(dbPool)
	sequenceRepo := postgres.NewSequenceRepository(dbPool)
	attachmentRepo := postgres.NewAttachmentRepository(dbPool)
	commRepo := postgres.NewCommunicationRepository(dbPool)
	serviceAccountRepo := postgres.NewServiceAccountRepository(dbPool)
	webhookRepo := postgres.NewWebhookRepository(dbPool)

	// Usecases
	tenantUsecase := tenant.NewUsecase(tenantRepo)
	orgUsecase := organization.NewUsecase(orgRepo)
	partyUsecase := party.NewUsecase(partyRepo, contactRepo)
	productUsecase := product.NewUsecase(unitRepo, productRepo)
	sequenceUsecase := sequenceusecase.NewUsecase(sequenceRepo)
	docUsecase := document.NewUsecase(docRepo, auditRepo, outboxRepo, sequenceUsecase)
	auditUsecase := audit.NewUsecase(auditRepo)
	identityUsecase := identityusecase.NewUsecase(identityRepo, tenantRepo, tokenMgr, cfg.JWTExpiry)
	customFieldUsecase := customfieldusecase.NewUsecase(customFieldRepo)
	attachmentUsecase := attachmentusecase.NewUsecase(attachmentRepo, storageDriver)
	commUsecase := commusecase.NewUsecase(commRepo, auditRepo, attachmentRepo, outboxRepo)
	serviceAccountUsecase := identityusecase.NewServiceAccountUsecase(serviceAccountRepo)
	webhookUsecase := webhookuc.New(webhookRepo)

	// Handlers
	tenantHandler := v1.NewTenantHandler(tenantUsecase, tokenMgr)
	orgHandler := v1.NewOrganizationHandler(orgUsecase)
	partyHandler := v1.NewPartyHandler(partyUsecase)
	productHandler := v1.NewProductHandler(productUsecase)
	docHandler := v1.NewDocumentHandler(docUsecase)
	auditHandler := v1.NewAuditHandler(auditUsecase)
	authHandler := v1.NewAuthHandler(identityUsecase, tokenMgr)
	identityHandler := v1.NewIdentityHandler(identityUsecase, tokenMgr)
	customFieldHandler := v1.NewCustomFieldHandler(customFieldUsecase, tokenMgr)
	sequenceHandler := v1.NewSequenceHandler(sequenceUsecase, tokenMgr)
	attachmentHandler := v1.NewAttachmentHandler(attachmentUsecase, tokenMgr)
	commHandler := v1.NewCommunicationHandler(commUsecase, tokenMgr)
	serviceAccountHandler := v1.NewServiceAccountHandler(serviceAccountUsecase, tokenMgr)
	webhookHandler := v1.NewWebhookHandler(webhookUsecase)

	// Rate limiter
	var limiter ratelimit.Limiter
	if cfg.RateLimitEnabled {
		limiter = ratelimit.NewInMemory(ratelimit.InMemoryConfig{
			RequestsPerWindow: cfg.RateLimitPerMinute,
			Window:            60 * time.Second,
		})
		logger.Info("rate limiting enabled",
			slog.Int("requests_per_minute", cfg.RateLimitPerMinute),
		)
	}

	// Webhook dispatcher (subscribes to event bus, POSTs to registered endpoints).
	webhookDispatcher := worker.NewWebhookDispatcher(webhookRepo, bus, logger)

	// Module registry
	moduleHost := module.NewHost(dbPool, bus, storageDriver, tokenMgr, outboxRepo, logger)
	moduleRegistry := module.NewRegistry(moduleHost, logger)

	for _, m := range opts.Modules {
		if err := moduleRegistry.Register(m); err != nil {
			dbPool.Close()
			return nil, fmt.Errorf("module register %s: %w", m.Manifest().Name, err)
		}
	}

	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	if err := moduleRegistry.InitAll(initCtx); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("module init: %w", err)
	}
	if err := moduleRegistry.RegisterPermissions(initCtx); err != nil {
		logger.Warn("failed to register module permissions", slog.Any("error", err))
	}
	moduleRegistry.BindSubscriptions()

	// Router
	handlers := deliveryhttp.Handlers{
		TenantHandler:         tenantHandler,
		OrganizationHandler:   orgHandler,
		PartyHandler:          partyHandler,
		ProductHandler:        productHandler,
		DocumentHandler:       docHandler,
		AuditHandler:          auditHandler,
		AuthHandler:           authHandler,
		IdentityHandler:       identityHandler,
		CustomFieldHandler:    customFieldHandler,
		SequenceHandler:       sequenceHandler,
		AttachmentHandler:     attachmentHandler,
		CommunicationHandler:  commHandler,
		ServiceAccountHandler: serviceAccountHandler,
		WebhookHandler:        webhookHandler,
		ModuleRegistry:        moduleRegistry,
		TokenManager:          tokenMgr,
		ApiKeyValidator:       serviceAccountUsecase,
		Limiter:               limiter,
		CORSAllowedOrigins:    cfg.CORSAllowedOrigins,
	}

	router := deliveryhttp.NewRouter(dbPool, handlers)

	return &App{
		cfg:               cfg,
		logger:            logger,
		db:                dbPool,
		tokenMgr:          tokenMgr,
		moduleHost:        moduleHost,
		moduleRegistry:    moduleRegistry,
		router:            router.(chi.Router),
		outboxWorker:      outboxWorker,
		webhookDispatcher: webhookDispatcher,
		rateLimiter:       limiter,
	}, nil
}

// Router returns the assembled Chi router (/healthz, /readyz, /api/v1 core + module routes).
func (a *App) Router() chi.Router { return a.router }

// Host returns the module Host for advanced embedding scenarios.
func (a *App) Host() module.Host { return a.moduleHost }

// DB returns the PostgreSQL connection pool.
func (a *App) DB() *pgxpool.Pool { return a.db }

// Tokens returns the JWT token manager.
func (a *App) Tokens() auth.TokenManager { return a.tokenMgr }

// Logger returns the application logger.
func (a *App) Logger() *slog.Logger { return a.logger }

// Start begins the outbox worker, webhook dispatcher, and HTTP server.
// It blocks until the context is cancelled or the server encounters a fatal error.
func (a *App) Start(ctx context.Context) error {
	workerCtx, workerCancel := context.WithCancel(ctx)
	a.workerCancel = workerCancel
	go a.outboxWorker.Start(workerCtx)

	// Start webhook dispatcher — subscribes to event bus synchronously then returns.
	a.webhookDispatcher.Start(workerCtx)

	addr := fmt.Sprintf(":%s", a.cfg.AppPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      a.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on context cancellation.
	shutdownErrChan := make(chan error, 1)
	go func() {
		<-ctx.Done()
		a.logger.Info("context cancelled, shutting down")
		workerCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := a.moduleRegistry.ShutdownAll(shutdownCtx); err != nil {
			a.logger.Error("error during module shutdown", slog.Any("error", err))
		}
		if a.rateLimiter != nil {
			a.rateLimiter.Close()
		}
		shutdownErrChan <- server.Shutdown(shutdownCtx)
	}()

	a.logger.Info("server listening", slog.String("addr", addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}

	return <-shutdownErrChan
}

// Shutdown performs a graceful shutdown without blocking on HTTP server lifecycle.
// Useful when the caller manages the http.Server lifecycle externally.
func (a *App) Shutdown(ctx context.Context) error {
	if a.workerCancel != nil {
		a.workerCancel()
	}
	if a.rateLimiter != nil {
		a.rateLimiter.Close()
	}
	return a.moduleRegistry.ShutdownAll(ctx)
}
