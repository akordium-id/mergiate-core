package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/pkg/config"
)

// NewPostgresPool initializes a pgx connection pool with health checking.
func NewPostgresPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database url: %w", err)
	}

	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MinConns = cfg.DBMinConns
	poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdle
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLife

	// PgBouncer transaction pooling does not support named prepared statements.
	// When DBPgBouncer is enabled, disable statement cache and use unnamed prepared statements.
	if cfg.DBPgBouncer {
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
		poolConfig.ConnConfig.StatementCacheCapacity = 0
		slog.Info("pgbouncer mode enabled: prepared statement caching disabled")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	slog.Info("connected to postgresql",
		slog.String("db_url_masked", poolConfig.ConnConfig.Host),
		slog.Int("max_conns", int(poolConfig.MaxConns)),
	)

	return pool, nil
}
