package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/organization"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type OrganizationHandler struct {
	usecase organization.Usecase
}

func NewOrganizationHandler(usecase organization.Usecase) *OrganizationHandler {
	return &OrganizationHandler{usecase: usecase}
}

func (h *OrganizationHandler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		// Write operations
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("organization:manage"))
			r.Post("/", h.Create)
			r.Put("/{id}", h.Update)
		})

		// Read operations
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("organization:read"))
			r.Get("/", h.List)
			r.Get("/tree", h.GetTree)
			r.Get("/{id}", h.GetByID)
		})
	})
}


func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var cmd organization.CreateCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	result, err := h.usecase.Create(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.List(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *OrganizationHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.GetTree(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *OrganizationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	result, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd organization.UpdateCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.ID = id

	result, err := h.usecase.Update(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}
