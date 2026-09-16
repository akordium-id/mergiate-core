package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txContextKey struct{}

// WithTxContext injects a pgx.Tx into the context.
func WithTxContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext extracts a pgx.Tx from the context if present.
func TxFromContext(ctx context.Context) pgx.Tx {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}

// WithTx executes fn within a PostgreSQL transaction using the pool.
// If a transaction is already active in ctx (nested call), fn is called directly.
// Otherwise, a new transaction is begun via pool.BeginFunc, which automatically commits
// on nil return or rolls back on error/panic.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	if pool == nil {
		return fn(ctx)
	}
	if tx := TxFromContext(ctx); tx != nil {
		return fn(ctx)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(WithTxContext(ctx, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
