package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/party"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	partyuc "github.com/akordium-id/mergiate-core/internal/core/usecase/party"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type PartyHandler struct {
	usecase partyuc.Usecase
}

func NewPartyHandler(usecase partyuc.Usecase) *PartyHandler {
	return &PartyHandler{usecase: usecase}
}

func (h *PartyHandler) RegisterRoutes(r chi.Router) {
	r.Route("/parties", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		// Read operations
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("party:read"))
			r.Get("/", h.List)
			r.Get("/{id}", h.GetByID)
		})

		// Create
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("party:create"))
			r.Post("/", h.Create)
			r.Post("/{id}/roles", h.AddRole)
			r.Post("/{id}/addresses", h.AddAddress)
			r.Post("/{id}/contacts", h.AddContact)
		})

		// Update
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("party:update"))
			r.Put("/{id}", h.Update)
		})

		// Delete
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("party:delete"))
			r.Delete("/{id}/roles/{roleId}", h.RemoveRole)
		})
	})
}


func (h *PartyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var cmd partyuc.CreatePartyCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	result, err := h.usecase.CreateParty(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

func (h *PartyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	result, err := h.usecase.GetParty(r.Context(), id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *PartyHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	var partyType *party.Type
	if t := r.URL.Query().Get("type"); t != "" {
		pt := party.Type(t)
		partyType = &pt
	}

	var roleType *party.RoleType
	if role := r.URL.Query().Get("role"); role != "" {
		pr := party.RoleType(role)
		roleType = &pr
	}

	result, err := h.usecase.ListParties(r.Context(), int32(page), int32(pageSize), partyType, roleType)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result.Items, map[string]any{
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

func (h *PartyHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd partyuc.UpdatePartyCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.ID = id

	result, err := h.usecase.UpdateParty(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *PartyHandler) AddRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	partyID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd partyuc.AddRoleCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.PartyID = partyID

	result, err := h.usecase.AddRole(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

func (h *PartyHandler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := shared.ParseID(roleIDStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	if err := h.usecase.RemoveRole(r.Context(), roleID); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "role removed successfully",
	})
}

func (h *PartyHandler) AddAddress(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	partyID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd partyuc.AddAddressCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	result, err := h.usecase.AddAddress(r.Context(), partyID, cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

func (h *PartyHandler) AddContact(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	partyID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd partyuc.AddContactCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	result, err := h.usecase.AddContact(r.Context(), partyID, cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}
