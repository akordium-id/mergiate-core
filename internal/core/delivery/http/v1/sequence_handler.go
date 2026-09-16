package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	sequsecase "github.com/akordium-id/mergiate-core/internal/core/usecase/sequence"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type SequenceHandler struct {
	usecase  sequsecase.Usecase
	tokenMgr auth.TokenManager
}

func NewSequenceHandler(usecase sequsecase.Usecase, tokenMgr auth.TokenManager) *SequenceHandler {
	return &SequenceHandler{
		usecase:  usecase,
		tokenMgr: tokenMgr,
	}
}

func (h *SequenceHandler) RegisterRoutes(r chi.Router) {
	r.Route("/sequences", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("sequence:manage"))

		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.GetByID)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)

		r.Post("/next", h.AcquireNext)
		r.Post("/preview", h.Preview)
	})
}


func (h *SequenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var cmd sequsecase.CreateSequenceCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID

	seq, err := h.usecase.CreateSequence(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, seq)
}

func (h *SequenceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid sequence ID format")
		return
	}

	seq, err := h.usecase.GetSequence(r.Context(), tenantID, id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, seq)
}

func (h *SequenceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	sequences, err := h.usecase.ListSequences(r.Context(), tenantID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, sequences)
}

func (h *SequenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid sequence ID format")
		return
	}

	var cmd sequsecase.UpdateSequenceCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID
	cmd.ID = id

	seq, err := h.usecase.UpdateSequence(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, seq)
}

func (h *SequenceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid sequence ID format")
		return
	}

	if err := h.usecase.DeleteSequence(r.Context(), tenantID, id); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "sequence deleted successfully"})
}

type acquireNextReq struct {
	EntityType  string            `json:"entity_type"`
	SubType     string            `json:"sub_type"`
	ExtraTokens map[string]string `json:"extra_tokens,omitempty"`
}

func (h *SequenceHandler) AcquireNext(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var req acquireNextReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	number, err := h.usecase.AcquireNextNumber(r.Context(), sequsecase.AcquireNextCommand{
		TenantID:    tenantID,
		EntityType:  req.EntityType,
		SubType:     req.SubType,
		ExtraTokens: req.ExtraTokens,
	})
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"number": number,
	})
}

func (h *SequenceHandler) Preview(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var req acquireNextReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	preview, err := h.usecase.PreviewNextNumber(r.Context(), sequsecase.PreviewCommand{
		TenantID:    tenantID,
		EntityType:  req.EntityType,
		SubType:     req.SubType,
		ExtraTokens: req.ExtraTokens,
	})
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"preview": preview,
	})
}
