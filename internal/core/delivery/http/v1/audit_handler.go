package v1

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	domainaudit "github.com/akordium-id/mergiate-core/internal/core/domain/audit"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/audit"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type AuditHandler struct {
	usecase audit.Usecase
}

func NewAuditHandler(usecase audit.Usecase) *AuditHandler {
	return &AuditHandler{usecase: usecase}
}

func (h *AuditHandler) RegisterRoutes(r chi.Router) {
	r.Route("/audit-logs", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("audit:read"))

		r.Get("/", h.List)
	})
}


func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page := int32(1)
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}

	pageSize := int32(20)
	if ps, err := strconv.Atoi(query.Get("page_size")); err == nil && ps > 0 {
		pageSize = int32(ps)
	}

	var entityType *string
	if et := query.Get("entity_type"); et != "" {
		entityType = &et
	}

	var entityID *shared.ID
	if eidStr := query.Get("entity_id"); eidStr != "" {
		if eid, err := shared.ParseID(eidStr); err == nil {
			entityID = &eid
		}
	}

	var actorID *shared.ID
	if aidStr := query.Get("actor_id"); aidStr != "" {
		if aid, err := shared.ParseID(aidStr); err == nil {
			actorID = &aid
		}
	}

	var action *domainaudit.Action
	if actStr := query.Get("action"); actStr != "" {
		a := domainaudit.Action(actStr)
		action = &a
	}

	res, err := h.usecase.List(r.Context(), page, pageSize, entityType, entityID, actorID, action)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}
