package v1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	commusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/communication"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type CommunicationHandler struct {
	usecase  commusecase.Usecase
	tokenMgr auth.TokenManager
}

func NewCommunicationHandler(usecase commusecase.Usecase, tokenMgr auth.TokenManager) *CommunicationHandler {
	return &CommunicationHandler{
		usecase:  usecase,
		tokenMgr: tokenMgr,
	}
}

func (h *CommunicationHandler) RegisterRoutes(r chi.Router) {
	// Comments endpoints
	r.Route("/comments", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("comment:read"))
			r.Get("/{entityType}/{entityId}", h.ListEntityComments)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("comment:create"))
			r.Post("/", h.CreateComment)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("comment:delete"))
			r.Delete("/{id}", h.DeleteComment)
		})
	})

	// Notifications endpoints
	r.Route("/notifications", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("notification:read"))

		r.Get("/", h.ListNotifications)
		r.Post("/{id}/read", h.MarkRead)
		r.Post("/read-all", h.MarkAllRead)
	})

	// Unified Activity Timeline endpoint
	r.Route("/activities", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("comment:read"))

		r.Get("/{entityType}/{entityId}", h.GetActivityTimeline)
	})
}


func (h *CommunicationHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var cmd commusecase.CreateCommentCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID

	// Extract author from JWT if available and author_id is empty
	if cmd.AuthorID == shared.NilID() {
		if claims, ok := shared.GetAuthClaims(r.Context()); ok {
			cmd.AuthorID = claims.UserID
		}
	}
	if cmd.AuthorID == shared.NilID() {
		response.Err(w, http.StatusBadRequest, "AUTHOR_REQUIRED", "author_id is required or user must be authenticated")
		return
	}

	comment, err := h.usecase.CreateComment(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, comment)
}

func (h *CommunicationHandler) ListEntityComments(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	entityType := strings.TrimSpace(chi.URLParam(r, "entityType"))
	entityID, err := shared.ParseID(chi.URLParam(r, "entityId"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ENTITY_ID", "Invalid entity UUID format")
		return
	}

	comments, err := h.usecase.ListEntityComments(r.Context(), tenantID, entityType, entityID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, comments)
}

func (h *CommunicationHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid comment ID format")
		return
	}

	var actorID shared.ID
	if claims, ok := shared.GetAuthClaims(r.Context()); ok {
		actorID = claims.UserID
	}

	if err := h.usecase.DeleteComment(r.Context(), tenantID, id, actorID); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "comment deleted successfully",
	})
}

func (h *CommunicationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var userID shared.ID
	if claims, ok := shared.GetAuthClaims(r.Context()); ok {
		userID = claims.UserID
	} else if uStr := r.URL.Query().Get("user_id"); uStr != "" {
		userID, _ = shared.ParseID(uStr)
	}

	if userID == shared.NilID() {
		response.Err(w, http.StatusBadRequest, "USER_REQUIRED", "User ID or authentication context is required")
		return
	}

	unreadOnly, _ := strconv.ParseBool(r.URL.Query().Get("unread_only"))
	limit := int32(20)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = int32(l)
		}
	}
	offset := int32(0)
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	items, unreadCount, err := h.usecase.ListNotifications(r.Context(), tenantID, userID, unreadOnly, limit, offset)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"items":        items,
		"unread_count": unreadCount,
	})
}

func (h *CommunicationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid notification ID format")
		return
	}

	var userID shared.ID
	if claims, ok := shared.GetAuthClaims(r.Context()); ok {
		userID = claims.UserID
	} else if uStr := r.URL.Query().Get("user_id"); uStr != "" {
		userID, _ = shared.ParseID(uStr)
	}

	if err := h.usecase.MarkNotificationRead(r.Context(), tenantID, userID, id); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "notification marked as read",
	})
}

func (h *CommunicationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var userID shared.ID
	if claims, ok := shared.GetAuthClaims(r.Context()); ok {
		userID = claims.UserID
	} else if uStr := r.URL.Query().Get("user_id"); uStr != "" {
		userID, _ = shared.ParseID(uStr)
	}

	if err := h.usecase.MarkAllNotificationsRead(r.Context(), tenantID, userID); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "all notifications marked as read",
	})
}

func (h *CommunicationHandler) GetActivityTimeline(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	entityType := strings.TrimSpace(chi.URLParam(r, "entityType"))
	entityID, err := shared.ParseID(chi.URLParam(r, "entityId"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ENTITY_ID", "Invalid entity UUID format")
		return
	}

	activities, err := h.usecase.GetUnifiedTimeline(r.Context(), tenantID, entityType, entityID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, activities)
}
