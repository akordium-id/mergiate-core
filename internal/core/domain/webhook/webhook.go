// Package webhook contains the domain entities, value objects, and repository
// interface for the Webhook Endpoint feature.
//
// Design note: webhooks are scoped per-tenant. Each endpoint has a secret used
// to compute an HMAC-SHA256 signature over the delivery payload, allowing
// receiving systems to verify authenticity (similar to GitHub/Stripe webhooks).
package webhook

import (
	"context"
	"errors"
	"time"

	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// Domain errors
var (
	ErrEndpointNotFound   = errors.New("webhook endpoint not found")
	ErrEndpointInactive   = errors.New("webhook endpoint is not active")
	ErrInvalidURL         = errors.New("webhook URL must be a valid https:// or http:// URL")
	ErrSecretTooShort     = errors.New("webhook secret must be at least 16 characters")
)

// DeliveryStatus represents the outcome of a single delivery attempt.
type DeliveryStatus string

const (
	DeliveryStatusPending DeliveryStatus = "pending"
	DeliveryStatusSuccess DeliveryStatus = "success"
	DeliveryStatusFailed  DeliveryStatus = "failed"
)

// Endpoint is a registered HTTP callback that receives domain event payloads.
type Endpoint struct {
	ID        sdk.ID    `json:"id"`
	TenantID  sdk.ID    `json:"tenant_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"` // never serialized — signing only
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Delivery is an immutable record of one delivery attempt to a specific endpoint.
type Delivery struct {
	ID             sdk.ID         `json:"id"`
	EndpointID     sdk.ID         `json:"endpoint_id"`
	OutboxEventID  sdk.ID         `json:"outbox_event_id"`
	EventType      string         `json:"event_type"`
	Attempt        int            `json:"attempt"`
	Status         DeliveryStatus `json:"status"`
	ResponseCode   *int           `json:"response_code,omitempty"`
	ResponseBody   *string        `json:"response_body,omitempty"`
	ErrorMessage   *string        `json:"error_message,omitempty"`
	DeliveredAt    *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// Repository is the persistence contract for webhook aggregates.
type Repository interface {
	// Endpoint CRUD
	CreateEndpoint(ctx context.Context, ep *Endpoint) error
	GetEndpointByID(ctx context.Context, tenantID, id sdk.ID) (*Endpoint, error)
	ListEndpoints(ctx context.Context, tenantID sdk.ID) ([]Endpoint, error)
	SetEndpointActive(ctx context.Context, tenantID, id sdk.ID, active bool) error
	DeleteEndpoint(ctx context.Context, tenantID, id sdk.ID) error

	// ListActiveEndpointsByTenant is used by the dispatcher to fan-out events.
	ListActiveEndpointsByTenant(ctx context.Context, tenantID sdk.ID) ([]Endpoint, error)

	// Delivery logging
	CreateDelivery(ctx context.Context, d *Delivery) error
	ListDeliveriesByEndpoint(ctx context.Context, tenantID, endpointID sdk.ID, limit int) ([]Delivery, error)
}
