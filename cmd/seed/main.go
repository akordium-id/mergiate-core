package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/akordium-id/mergiate-core/pkg/bootstrap"
	"github.com/akordium-id/mergiate-core/pkg/config"
	"github.com/akordium-id/mergiate-core/pkg/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// CLI flags with environment fallbacks
	emailFlag := flag.String("email", getEnvDefault("SEED_EMAIL", "admin@akordium.com"), "Owner user email")
	passwordFlag := flag.String("password", getEnvDefault("SEED_PASSWORD", "Secret123!"), "Owner user password")
	nameFlag := flag.String("name", getEnvDefault("SEED_NAME", "Owner Administrator"), "Owner user full name")
	tenantCodeFlag := flag.String("tenant", getEnvDefault("SEED_TENANT", "akordium"), "Tenant code (alias: -tenant-code)")
	tenantCodeAlt := flag.String("tenant-code", "", "Tenant code alternative flag")
	tenantNameFlag := flag.String("tenant-name", getEnvDefault("SEED_TENANT_NAME", "Akordium Lab"), "Tenant display name")
	orgNameFlag := flag.String("org", getEnvDefault("SEED_ORG", "Akordium Main Org"), "Organization display name (alias: -org-name)")
	orgNameAlt := flag.String("org-name", "", "Organization display name alternative flag")
	orgCodeFlag := flag.String("org-code", getEnvDefault("SEED_ORG_CODE", "main"), "Organization code")

	flag.Parse()

	tenantCode := *tenantCodeFlag
	if *tenantCodeAlt != "" {
		tenantCode = *tenantCodeAlt
	}

	orgName := *orgNameFlag
	if *orgNameAlt != "" {
		orgName = *orgNameAlt
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	ctx := context.Background()
	dbPool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer dbPool.Close()

	seedCfg := bootstrap.Config{
		Email:      *emailFlag,
		Password:   *passwordFlag,
		Name:       *nameFlag,
		TenantCode: tenantCode,
		TenantName: *tenantNameFlag,
		OrgCode:    *orgCodeFlag,
		OrgName:    orgName,
	}

	result, err := bootstrap.Run(ctx, dbPool, seedCfg)
	if err != nil {
		slog.Error("bootstrap failed", slog.Any("error", err))
		os.Exit(1)
	}

	fmt.Print(result.Summary())
	slog.Info("bootstrap finished successfully")
}

func getEnvDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
