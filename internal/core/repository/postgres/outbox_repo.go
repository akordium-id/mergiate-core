package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/event"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres/sqlc"
	"github.com/akordium-id/mergiate-core/pkg/database"
)

type outboxRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewOutboxRepository creates a new PostgreSQL Outbox repository.
func NewOutboxRepository(pool *pgxpool.Pool) event.OutboxRepository {
	return &outboxRepository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *outboxRepository) q(ctx context.Context) *sqlc.Queries {
	if tx := database.TxFromContext(ctx); tx != nil {
		return r.queries.WithTx(tx)
	}
	return r.queries
}

func (r *outboxRepository) Create(ctx context.Context, evt *event.OutboxEvent) error {
	payloadJSON, _ := json.Marshal(evt.Payload)

	status := string(evt.Status)
	if status == "" {
		status = string(event.OutboxStatusPending)
	}

	params := sqlc.CreateOutboxEventParams{
		ID:            shared.ToPgUUID(evt.ID),
		TenantID:      shared.ToPgUUID(evt.TenantID),
		EventType:     evt.EventType,
		AggregateType: evt.AggregateType,
		AggregateID:   shared.ToPgUUID(evt.AggregateID),
		Payload:       payloadJSON,
		Status:        status,
		RetryCount:    evt.RetryCount,
		CreatedAt:     pgtype.Timestamptz{Time: evt.CreatedAt, Valid: true},
	}

	row, err := r.q(ctx).CreateOutboxEvent(ctx, params)
	if err != nil {
		return err
	}

	evt.ID = shared.FromPgUUID(row.ID)
	evt.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *outboxRepository) FetchPending(ctx context.Context, maxRetries, limit int32) ([]event.OutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if maxRetries <= 0 {
		maxRetries = 5
	}

	rows, err := r.q(ctx).FetchPendingOutboxEvents(ctx, sqlc.FetchPendingOutboxEventsParams{
		RetryCount: maxRetries,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}

	events := make([]event.OutboxEvent, len(rows))
	for i, row := range rows {
		var payload map[string]any
		if len(row.Payload) > 0 {
			_ = json.Unmarshal(row.Payload, &payload)
		}

		var pubAt *time.Time
		if row.PublishedAt.Valid {
			t := row.PublishedAt.Time
			pubAt = &t
		}

		events[i] = event.OutboxEvent{
			ID:            shared.FromPgUUID(row.ID),
			TenantID:      shared.FromPgUUID(row.TenantID),
			EventType:     row.EventType,
			AggregateType: row.AggregateType,
			AggregateID:   shared.FromPgUUID(row.AggregateID),
			Payload:       payload,
			Status:        event.OutboxStatus(row.Status),
			RetryCount:    row.RetryCount,
			ErrorMessage:  strFromPtr(row.ErrorMessage),
			CreatedAt:     row.CreatedAt.Time,
			PublishedAt:   pubAt,
		}
	}

	return events, nil
}

func (r *outboxRepository) MarkPublished(ctx context.Context, id shared.ID, publishedAt time.Time) error {
	return r.q(ctx).MarkOutboxEventPublished(ctx, sqlc.MarkOutboxEventPublishedParams{
		ID:          shared.ToPgUUID(id),
		PublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
	})
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id shared.ID, errMsg string) error {
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	return r.q(ctx).MarkOutboxEventFailed(ctx, sqlc.MarkOutboxEventFailedParams{
		ID:           shared.ToPgUUID(id),
		ErrorMessage: errPtr,
	})
}
