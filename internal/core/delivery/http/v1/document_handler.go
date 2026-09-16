package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	domaindoc "github.com/akordium-id/mergiate-core/internal/core/domain/document"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/document"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type DocumentHandler struct {
	usecase document.Usecase
}

func NewDocumentHandler(usecase document.Usecase) *DocumentHandler {
	return &DocumentHandler{usecase: usecase}
}

func (h *DocumentHandler) RegisterRoutes(r chi.Router) {
	r.Route("/documents", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("document:read"))
			r.Get("/", h.List)
			r.Get("/{id}", h.GetByID)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("document:create"))
			r.Post("/", h.Create)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("document:transition"))
			r.Post("/{id}/transition", h.Transition)
		})
	})
}


func (h *DocumentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var cmd document.CreateDocumentCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	res, err := h.usecase.CreateDocument(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page := int32(1)
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}

	pageSize := int32(20)
	if ps, err := strconv.Atoi(query.Get("page_size")); err == nil && ps > 0 {
		pageSize = int32(ps)
	}

	var orgID *shared.ID
	if oidStr := query.Get("organization_id"); oidStr != "" {
		if oid, err := shared.ParseID(oidStr); err == nil {
			orgID = &oid
		}
	}

	var docType *domaindoc.DocumentType
	if dtStr := query.Get("type"); dtStr != "" {
		dt := domaindoc.DocumentType(dtStr)
		docType = &dt
	}

	var status *domaindoc.Status
	if stStr := query.Get("status"); stStr != "" {
		st := domaindoc.Status(stStr)
		status = &st
	}

	res, err := h.usecase.ListDocuments(r.Context(), page, pageSize, orgID, docType, status)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *DocumentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	res, err := h.usecase.GetDocumentByID(r.Context(), id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

type transitionRequest struct {
	TargetStatus domaindoc.Status `json:"target_status"`
	Reason       string           `json:"reason,omitempty"`
	ActorID      *shared.ID       `json:"actor_id,omitempty"`
}

func (h *DocumentHandler) Transition(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	cmd := document.TransitionDocumentCommand{
		DocumentID:   id,
		TargetStatus: req.TargetStatus,
		Reason:       req.Reason,
		ActorID:      req.ActorID,
	}

	res, err := h.usecase.TransitionDocument(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}
