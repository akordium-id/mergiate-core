package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/audit"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres/sqlc"
	"github.com/akordium-id/mergiate-core/pkg/database"
)

type auditRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewAuditRepository creates a new PostgreSQL audit log repository.
func NewAuditRepository(pool *pgxpool.Pool) audit.Repository {
	return &auditRepository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *auditRepository) q(ctx context.Context) *sqlc.Queries {
	if tx := database.TxFromContext(ctx); tx != nil {
		return r.queries.WithTx(tx)
	}
	return r.queries
}

func (r *auditRepository) Create(ctx context.Context, a *audit.AuditLog) error {
	changesJSON, _ := json.Marshal(a.Changes)
	metaJSON, _ := json.Marshal(a.Metadata)

	var actorID pgtype.UUID
	if a.ActorID != nil && *a.ActorID != shared.NilID() {
		actorID = shared.ToPgUUID(*a.ActorID)
	}

	params := sqlc.CreateAuditLogParams{
		ID:         shared.ToPgUUID(a.ID),
		TenantID:   shared.ToPgUUID(a.TenantID),
		ActorID:    actorID,
		ActorType:  string(a.ActorType),
		Action:     string(a.Action),
		EntityType: a.EntityType,
		EntityID:   shared.ToPgUUID(a.EntityID),
		Changes:    changesJSON,
		Metadata:   metaJSON,
		CreatedAt:  pgtype.Timestamptz{Time: a.CreatedAt, Valid: true},
	}

	row, err := r.q(ctx).CreateAuditLog(ctx, params)
	if err != nil {
		return err
	}

	a.ID = shared.FromPgUUID(row.ID)
	a.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *auditRepository) List(ctx context.Context, tenantID shared.ID, filter audit.Filter) ([]audit.AuditLog, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := max(filter.Offset, 0)

	var entityID pgtype.UUID
	if filter.EntityID != nil && *filter.EntityID != shared.NilID() {
		entityID = shared.ToPgUUID(*filter.EntityID)
	}

	var actorID pgtype.UUID
	if filter.ActorID != nil && *filter.ActorID != shared.NilID() {
		actorID = shared.ToPgUUID(*filter.ActorID)
	}

	var actionStr *string
	if filter.Action != nil {
		s := string(*filter.Action)
		actionStr = &s
	}

	total, err := r.q(ctx).CountAuditLogs(ctx, sqlc.CountAuditLogsParams{
		TenantID:   shared.ToPgUUID(tenantID),
		EntityType: filter.EntityType,
		EntityID:   entityID,
		ActorID:    actorID,
		Action:     actionStr,
	})
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.q(ctx).ListAuditLogs(ctx, sqlc.ListAuditLogsParams{
		TenantID:   shared.ToPgUUID(tenantID),
		Limit:      limit,
		Offset:     offset,
		EntityType: filter.EntityType,
		EntityID:   entityID,
		ActorID:    actorID,
		Action:     actionStr,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]audit.AuditLog, len(rows))
	for i, row := range rows {
		var aid *shared.ID
		if row.ActorID.Valid {
			id := shared.FromPgUUID(row.ActorID)
			aid = &id
		}

		var changes map[string]any
		if len(row.Changes) > 0 {
			_ = json.Unmarshal(row.Changes, &changes)
		}

		var meta map[string]any
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &meta)
		}

		items[i] = audit.AuditLog{
			ID:         shared.FromPgUUID(row.ID),
			TenantID:   shared.FromPgUUID(row.TenantID),
			ActorID:    aid,
			ActorType:  audit.ActorType(row.ActorType),
			Action:     audit.Action(row.Action),
			EntityType: row.EntityType,
			EntityID:   shared.FromPgUUID(row.EntityID),
			Changes:    changes,
			Metadata:   meta,
			CreatedAt:  row.CreatedAt.Time,
		}
	}

	return items, total, nil
}
