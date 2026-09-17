// Package webhookuc provides the application-layer use cases for managing
// webhook endpoints and querying delivery logs.
package webhookuc

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/akordium-id/mergiate-core/internal/core/domain/webhook"
	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// Usecase orchestrates webhook endpoint registration and delivery log queries.
type Usecase struct {
	repo webhook.Repository
}

// New constructs a Usecase.
func New(repo webhook.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// RegisterEndpointInput is the DTO for creating a new webhook endpoint.
type RegisterEndpointInput struct {
	TenantID sdk.ID
	Name     string
	URL      string
	Secret   string
}

// RegisterEndpoint validates the input and persists a new webhook endpoint.
func (u *Usecase) RegisterEndpoint(ctx context.Context, in RegisterEndpointInput) (*webhook.Endpoint, error) {
	if err := validateEndpointInput(in.Name, in.URL, in.Secret); err != nil {
		return nil, err
	}

	id, err := sdk.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	now := time.Now().UTC()
	ep := &webhook.Endpoint{
		ID:        id,
		TenantID:  in.TenantID,
		Name:      strings.TrimSpace(in.Name),
		URL:       strings.TrimSpace(in.URL),
		Secret:    in.Secret,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := u.repo.CreateEndpoint(ctx, ep); err != nil {
		return nil, fmt.Errorf("create webhook endpoint: %w", err)
	}
	// Mask secret in the returned struct — callers should store the secret themselves.
	ep.Secret = ""
	return ep, nil
}

// ListEndpoints returns all webhook endpoints for a tenant.
func (u *Usecase) ListEndpoints(ctx context.Context, tenantID sdk.ID) ([]webhook.Endpoint, error) {
	eps, err := u.repo.ListEndpoints(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	// Mask secrets.
	for i := range eps {
		eps[i].Secret = ""
	}
	return eps, nil
}

// DisableEndpoint deactivates a webhook endpoint (soft-disable).
func (u *Usecase) DisableEndpoint(ctx context.Context, tenantID, id sdk.ID) error {
	return u.repo.SetEndpointActive(ctx, tenantID, id, false)
}

// EnableEndpoint re-activates a previously disabled endpoint.
func (u *Usecase) EnableEndpoint(ctx context.Context, tenantID, id sdk.ID) error {
	return u.repo.SetEndpointActive(ctx, tenantID, id, true)
}

// DeleteEndpoint permanently removes a webhook endpoint and its deliveries
// (via DB cascade).
func (u *Usecase) DeleteEndpoint(ctx context.Context, tenantID, id sdk.ID) error {
	return u.repo.DeleteEndpoint(ctx, tenantID, id)
}

// ListDeliveries returns recent delivery attempts for a specific endpoint.
func (u *Usecase) ListDeliveries(ctx context.Context, tenantID, endpointID sdk.ID, limit int) ([]webhook.Delivery, error) {
	return u.repo.ListDeliveriesByEndpoint(ctx, tenantID, endpointID, limit)
}

// ---------------------------------------------------------------------------
// Internal validation
// ---------------------------------------------------------------------------

func validateEndpointInput(name, rawURL, secret string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", sdk.ErrInvalidInput)
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return webhook.ErrInvalidURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return webhook.ErrInvalidURL
	}
	if len(secret) < 16 {
		return webhook.ErrSecretTooShort
	}
	return nil
}
