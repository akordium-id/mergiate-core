package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/webhook"
	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// WebhookRepository implements webhook.Repository using PostgreSQL.
type WebhookRepository struct {
	db *pgxpool.Pool
}

// NewWebhookRepository constructs the postgres webhook repository.
func NewWebhookRepository(db *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// ---------------------------------------------------------------------------
// Endpoint CRUD
// ---------------------------------------------------------------------------

func (r *WebhookRepository) CreateEndpoint(ctx context.Context, ep *webhook.Endpoint) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, tenant_id, name, url, secret, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sdk.ToPgUUID(ep.ID),
		sdk.ToPgUUID(ep.TenantID),
		ep.Name,
		ep.URL,
		ep.Secret,
		ep.IsActive,
		ep.CreatedAt,
		ep.UpdatedAt,
	)
	return err
}

func (r *WebhookRepository) GetEndpointByID(ctx context.Context, tenantID, id sdk.ID) (*webhook.Endpoint, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, name, url, secret, is_active, created_at, updated_at
		FROM webhook_endpoints
		WHERE id = $1 AND tenant_id = $2`,
		sdk.ToPgUUID(id),
		sdk.ToPgUUID(tenantID),
	)
	return scanEndpoint(row)
}

func (r *WebhookRepository) ListEndpoints(ctx context.Context, tenantID sdk.ID) ([]webhook.Endpoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, name, url, secret, is_active, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1
		ORDER BY created_at DESC`,
		sdk.ToPgUUID(tenantID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectEndpoints(rows)
}

func (r *WebhookRepository) SetEndpointActive(ctx context.Context, tenantID, id sdk.ID, active bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE webhook_endpoints SET is_active = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4`,
		active, time.Now().UTC(),
		sdk.ToPgUUID(id), sdk.ToPgUUID(tenantID),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return webhook.ErrEndpointNotFound
	}
	return nil
}

func (r *WebhookRepository) DeleteEndpoint(ctx context.Context, tenantID, id sdk.ID) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2`,
		sdk.ToPgUUID(id), sdk.ToPgUUID(tenantID),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return webhook.ErrEndpointNotFound
	}
	return nil
}

func (r *WebhookRepository) ListActiveEndpointsByTenant(ctx context.Context, tenantID sdk.ID) ([]webhook.Endpoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, name, url, secret, is_active, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1 AND is_active = TRUE
		ORDER BY created_at`,
		sdk.ToPgUUID(tenantID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectEndpoints(rows)
}

// ---------------------------------------------------------------------------
// Delivery logging
// ---------------------------------------------------------------------------

func (r *WebhookRepository) CreateDelivery(ctx context.Context, d *webhook.Delivery) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhook_deliveries
		    (id, endpoint_id, outbox_event_id, event_type, attempt, status,
		     response_code, response_body, error_message, delivered_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		sdk.ToPgUUID(d.ID),
		sdk.ToPgUUID(d.EndpointID),
		sdk.ToPgUUID(d.OutboxEventID),
		d.EventType,
		d.Attempt,
		string(d.Status),
		d.ResponseCode,
		d.ResponseBody,
		d.ErrorMessage,
		d.DeliveredAt,
		d.CreatedAt,
	)
	return err
}

func (r *WebhookRepository) ListDeliveriesByEndpoint(ctx context.Context, tenantID, endpointID sdk.ID, limit int) ([]webhook.Delivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT d.id, d.endpoint_id, d.outbox_event_id, d.event_type, d.attempt, d.status,
		       d.response_code, d.response_body, d.error_message, d.delivered_at, d.created_at
		FROM webhook_deliveries d
		JOIN webhook_endpoints e ON e.id = d.endpoint_id
		WHERE d.endpoint_id = $1 AND e.tenant_id = $2
		ORDER BY d.created_at DESC
		LIMIT $3`,
		sdk.ToPgUUID(endpointID), sdk.ToPgUUID(tenantID), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectDeliveries(rows)
}

// ---------------------------------------------------------------------------
// Internal scan helpers
// ---------------------------------------------------------------------------

type scannable interface {
	Scan(dest ...any) error
}

func scanEndpoint(row scannable) (*webhook.Endpoint, error) {
	var ep webhook.Endpoint
	var pgID, pgTenantID [16]byte
	err := row.Scan(
		&pgID, &pgTenantID,
		&ep.Name, &ep.URL, &ep.Secret, &ep.IsActive,
		&ep.CreatedAt, &ep.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, webhook.ErrEndpointNotFound
		}
		return nil, err
	}
	ep.ID = sdk.ID(pgID)
	ep.TenantID = sdk.ID(pgTenantID)
	return &ep, nil
}

func collectEndpoints(rows pgx.Rows) ([]webhook.Endpoint, error) {
	var endpoints []webhook.Endpoint
	for rows.Next() {
		var ep webhook.Endpoint
		var pgID, pgTenantID [16]byte
		if err := rows.Scan(
			&pgID, &pgTenantID,
			&ep.Name, &ep.URL, &ep.Secret, &ep.IsActive,
			&ep.CreatedAt, &ep.UpdatedAt,
		); err != nil {
			return nil, err
		}
		ep.ID = sdk.ID(pgID)
		ep.TenantID = sdk.ID(pgTenantID)
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}

func collectDeliveries(rows pgx.Rows) ([]webhook.Delivery, error) {
	var deliveries []webhook.Delivery
	for rows.Next() {
		var d webhook.Delivery
		var pgID, pgEpID, pgEvtID [16]byte
		if err := rows.Scan(
			&pgID, &pgEpID, &pgEvtID,
			&d.EventType, &d.Attempt, &d.Status,
			&d.ResponseCode, &d.ResponseBody, &d.ErrorMessage,
			&d.DeliveredAt, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		d.ID = sdk.ID(pgID)
		d.EndpointID = sdk.ID(pgEpID)
		d.OutboxEventID = sdk.ID(pgEvtID)
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}
