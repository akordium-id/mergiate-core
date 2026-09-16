package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	"github.com/akordium-id/mergiate-core/internal/core/worker"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/config"
	"github.com/akordium-id/mergiate-core/pkg/database"
	"github.com/akordium-id/mergiate-core/pkg/eventbus"
	"github.com/akordium-id/mergiate-core/pkg/module"
	"github.com/akordium-id/mergiate-core/pkg/storage/local"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// A4: JWT secret hygiene — refuse to start in production with insecure secret.
	if err := cfg.ValidateJWTSecret(); err != nil {
		slog.Error("SECURITY: refusing to start — insecure JWT configuration", slog.Any("error", err))
		os.Exit(1)
	}
	if config.IsInsecureJWTSecret(cfg.JWTSecret) {
		slog.Warn("SECURITY WARNING: using insecure default JWT secret — set JWT_SECRET in production")
	}

	slog.Info("starting application",
		slog.String("app", cfg.AppName),
		slog.String("env", cfg.AppEnv),
		slog.String("port", cfg.AppPort),
	)

	// Database Connection Pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbPool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		slog.Warn("database connection failed (will run in degraded mode if offline)", slog.Any("error", err))
	} else {
		defer dbPool.Close()
	}

	// Layer Wiring (Clean Architecture)
	tenantRepo := postgres.NewTenantRepository(dbPool)
	orgRepo := postgres.NewOrganizationRepository(dbPool)
	partyRepo := postgres.NewPartyRepository(dbPool)
	contactRepo := postgres.NewAddressContactRepository(dbPool)
	unitRepo := postgres.NewUnitRepository(dbPool)
	productRepo := postgres.NewProductRepository(dbPool)
	docRepo := postgres.NewDocumentRepository(dbPool)
	auditRepo := postgres.NewAuditRepository(dbPool)
	outboxRepo := postgres.NewOutboxRepository(dbPool)
	identityRepo := postgres.NewIdentityRepository(dbPool)
	customFieldRepo := postgres.NewCustomFieldRepository(dbPool)
	sequenceRepo := postgres.NewSequenceRepository(dbPool)
	attachmentRepo := postgres.NewAttachmentRepository(dbPool)
	commRepo := postgres.NewCommunicationRepository(dbPool)
	serviceAccountRepo := postgres.NewServiceAccountRepository(dbPool)

	// Pluggable Storage Driver
	storageDriver, err := local.NewDriver(cfg.StoragePath)
	if err != nil {
		slog.Error("failed to initialize storage driver", slog.Any("error", err))
		os.Exit(1)
	}

	// Auth & Security Token Manager
	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, cfg.AppName)

	// Event Bus & Background Outbox Worker
	bus := eventbus.NewInMemoryBus()
	outboxWorker := worker.NewOutboxWorker(outboxRepo, bus, worker.DefaultConfig())

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go outboxWorker.Start(workerCtx)

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

	// Module SPI & Plugin Engine
	moduleHost := module.NewHost(dbPool, bus, storageDriver, tokenMgr, outboxRepo, logger)
	moduleRegistry := module.NewRegistry(moduleHost, logger)

	// Note: External modules are registered to moduleRegistry before InitAll
	if err := moduleRegistry.InitAll(context.Background()); err != nil {
		slog.Error("failed to initialize modules", slog.Any("error", err))
		os.Exit(1)
	}
	if err := moduleRegistry.RegisterPermissions(context.Background()); err != nil {
		slog.Warn("failed to register module permissions", slog.Any("error", err))
	}
	moduleRegistry.BindSubscriptions()

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
		ModuleRegistry:        moduleRegistry,
		TokenManager:          tokenMgr,
		ApiKeyValidator:       serviceAccountUsecase,
		CORSAllowedOrigins:    cfg.CORSAllowedOrigins,
	}

	router := deliveryhttp.NewRouter(dbPool, handlers)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server shutdown channel
	shutdownErrChan := make(chan error, 1)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit

		slog.Info("received shutdown signal", slog.String("signal", sig.String()))
		workerCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := moduleRegistry.ShutdownAll(shutdownCtx); err != nil {
			slog.Error("error during module shutdown", slog.Any("error", err))
		}

		shutdownErrChan <- server.Shutdown(shutdownCtx)
	}()

	slog.Info("server listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server fatal error", slog.Any("error", err))
		os.Exit(1)
	}

	if err := <-shutdownErrChan; err != nil {
		slog.Error("error during server shutdown", slog.Any("error", err))
	}

	slog.Info("server exited cleanly")
}
