package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/domain/webhook"
	webhookuc "github.com/akordium-id/mergiate-core/internal/core/usecase/webhook"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

// WebhookHandler manages webhook endpoint registration and delivery log queries.
type WebhookHandler struct {
	usecase *webhookuc.Usecase
}

// NewWebhookHandler constructs the handler.
func NewWebhookHandler(uc *webhookuc.Usecase) *WebhookHandler {
	return &WebhookHandler{usecase: uc}
}

// RegisterRoutes mounts webhook routes under /webhooks.
func (h *WebhookHandler) RegisterRoutes(r chi.Router) {
	r.Route("/webhooks", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequireAnyPermission("webhook:manage", "webhook:view"))

		r.Get("/", h.ListEndpoints)

		// Write operations require the manage permission.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("webhook:manage"))
			r.Post("/", h.RegisterEndpoint)
			r.Delete("/{id}", h.DeleteEndpoint)
			r.Post("/{id}/enable", h.EnableEndpoint)
			r.Post("/{id}/disable", h.DisableEndpoint)
		})

		r.Get("/{id}/deliveries", h.ListDeliveries)
	})
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

type registerEndpointRequest struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

// RegisterEndpoint POST /api/v1/webhooks
func (h *WebhookHandler) RegisterEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}

	var req registerEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON")
		return
	}

	ep, err := h.usecase.RegisterEndpoint(r.Context(), webhookuc.RegisterEndpointInput{
		TenantID: tenantID,
		Name:     req.Name,
		URL:      req.URL,
		Secret:   req.Secret,
	})
	if err != nil {
		switch {
		case errors.Is(err, webhook.ErrInvalidURL):
			response.Err(w, http.StatusBadRequest, "INVALID_URL", err.Error())
		case errors.Is(err, webhook.ErrSecretTooShort):
			response.Err(w, http.StatusBadRequest, "SECRET_TOO_SHORT", err.Error())
		default:
			response.Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to register webhook endpoint")
		}
		return
	}

	response.JSON(w, http.StatusCreated, ep)
}

// ListEndpoints GET /api/v1/webhooks
func (h *WebhookHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}

	eps, err := h.usecase.ListEndpoints(r.Context(), tenantID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list webhook endpoints")
		return
	}

	response.JSON(w, http.StatusOK, eps)
}

// DeleteEndpoint DELETE /api/v1/webhooks/{id}
func (h *WebhookHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid webhook endpoint ID")
		return
	}

	if err := h.usecase.DeleteEndpoint(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			response.Err(w, http.StatusNotFound, "NOT_FOUND", "Webhook endpoint not found")
			return
		}
		response.Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete webhook endpoint")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EnableEndpoint POST /api/v1/webhooks/{id}/enable
func (h *WebhookHandler) EnableEndpoint(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

// DisableEndpoint POST /api/v1/webhooks/{id}/disable
func (h *WebhookHandler) DisableEndpoint(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *WebhookHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid webhook endpoint ID")
		return
	}

	if active {
		err = h.usecase.EnableEndpoint(r.Context(), tenantID, id)
	} else {
		err = h.usecase.DisableEndpoint(r.Context(), tenantID, id)
	}
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			response.Err(w, http.StatusNotFound, "NOT_FOUND", "Webhook endpoint not found")
			return
		}
		response.Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update webhook endpoint")
		return
	}

	status := "disabled"
	if active {
		status = "enabled"
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": status})
}

// ListDeliveries GET /api/v1/webhooks/{id}/deliveries?limit=50
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", err.Error())
		return
	}

	endpointID, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid webhook endpoint ID")
		return
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	deliveries, err := h.usecase.ListDeliveries(r.Context(), tenantID, endpointID, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list webhook deliveries")
		return
	}

	response.JSON(w, http.StatusOK, deliveries)
}
