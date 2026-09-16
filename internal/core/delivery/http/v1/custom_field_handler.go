package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/customfield"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	cfusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/customfield"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type CustomFieldHandler struct {
	usecase  cfusecase.Usecase
	tokenMgr auth.TokenManager
}

func NewCustomFieldHandler(usecase cfusecase.Usecase, tokenMgr auth.TokenManager) *CustomFieldHandler {
	return &CustomFieldHandler{
		usecase:  usecase,
		tokenMgr: tokenMgr,
	}
}

func (h *CustomFieldHandler) RegisterRoutes(r chi.Router) {
	r.Route("/custom-fields", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("custom_field:manage"))

		// Definitions management
		r.Route("/definitions", func(r chi.Router) {
			r.Get("/", h.ListDefinitions)
			r.Get("/{id}", h.GetDefinition)
			r.Post("/", h.CreateDefinition)
			r.Put("/{id}", h.UpdateDefinition)
			r.Delete("/{id}", h.DeleteDefinition)
		})

		// Entity values
		r.Route("/values", func(r chi.Router) {
			r.Get("/{entityType}/{entityId}", h.GetEntityValues)
			r.Put("/{entityType}/{entityId}", h.SetEntityValues)
		})
	})
}


// ----------------------------------------------------------------------------
// Definitions Handlers
// ----------------------------------------------------------------------------

func (h *CustomFieldHandler) CreateDefinition(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var cmd cfusecase.CreateDefinitionCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID

	def, err := h.usecase.CreateDefinition(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, def)
}

func (h *CustomFieldHandler) GetDefinition(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid definition ID format")
		return
	}

	def, err := h.usecase.GetDefinition(r.Context(), tenantID, id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, def)
}

func (h *CustomFieldHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	entityTypeStr := r.URL.Query().Get("entity_type")
	if entityTypeStr == "" {
		response.Err(w, http.StatusBadRequest, "MISSING_ENTITY_TYPE", "Query parameter 'entity_type' is required")
		return
	}

	defs, err := h.usecase.ListDefinitions(r.Context(), tenantID, customfield.EntityType(entityTypeStr))
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, defs)
}

func (h *CustomFieldHandler) UpdateDefinition(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid definition ID format")
		return
	}

	var cmd cfusecase.UpdateDefinitionCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID
	cmd.ID = id

	def, err := h.usecase.UpdateDefinition(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, def)
}

func (h *CustomFieldHandler) DeleteDefinition(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid definition ID format")
		return
	}

	if err := h.usecase.DeleteDefinition(r.Context(), tenantID, id); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "definition deleted successfully"})
}

// ----------------------------------------------------------------------------
// Entity Values Handlers
// ----------------------------------------------------------------------------

func (h *CustomFieldHandler) GetEntityValues(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	entityType := customfield.EntityType(chi.URLParam(r, "entityType"))
	entityID, err := shared.ParseID(chi.URLParam(r, "entityId"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid entity ID format")
		return
	}

	vals, err := h.usecase.GetEntityValues(r.Context(), tenantID, entityType, entityID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, vals)
}

type setValuesReq struct {
	Values map[string]any `json:"values"`
}

func (h *CustomFieldHandler) SetEntityValues(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	entityType := customfield.EntityType(chi.URLParam(r, "entityType"))
	entityID, err := shared.ParseID(chi.URLParam(r, "entityId"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid entity ID format")
		return
	}

	var req setValuesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	vals, err := h.usecase.SetEntityValues(r.Context(), cfusecase.SetEntityValuesCommand{
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		Values:     req.Values,
	})
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, vals)
}
