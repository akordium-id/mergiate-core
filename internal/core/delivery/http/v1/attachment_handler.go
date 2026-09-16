package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	attusecase "github.com/akordium-id/mergiate-core/internal/core/usecase/attachment"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type AttachmentHandler struct {
	usecase  attusecase.Usecase
	tokenMgr auth.TokenManager
}

func NewAttachmentHandler(usecase attusecase.Usecase, tokenMgr auth.TokenManager) *AttachmentHandler {
	return &AttachmentHandler{
		usecase:  usecase,
		tokenMgr: tokenMgr,
	}
}

func (h *AttachmentHandler) RegisterRoutes(r chi.Router) {
	// Files endpoints
	r.Route("/files", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("file:upload"))
			r.Post("/upload", h.Upload)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("file:read"))
			r.Get("/{id}", h.GetMetadata)
			r.Get("/{id}/download", h.Download)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("file:delete"))
			r.Delete("/{id}", h.DeleteFile)
		})
	})

	// Polymorphic entity attachments endpoints
	r.Route("/attachments", func(r chi.Router) {
		r.Use(middleware.TenantRequired())
		r.Use(middleware.RequirePermission("attachment:manage"))

		r.Post("/", h.Attach)
		r.Get("/{entityType}/{entityId}", h.ListByEntity)
		r.Delete("/{id}", h.Detach)
	})
}


func (h *AttachmentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	// 32MB in-memory buffer before spilling to temp files
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_MULTIPART", "Failed to parse multipart form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Err(w, http.StatusBadRequest, "FILE_REQUIRED", "Form field 'file' is required")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		mimeType = http.DetectContentType(buf[:n])
		// Seek back to beginning
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}
	}

	isPublic := false
	if pubVal := r.FormValue("is_public"); pubVal != "" {
		isPublic, _ = strconv.ParseBool(pubVal)
	}

	var uploaderID *shared.ID
	if claims, ok := shared.GetAuthClaims(r.Context()); ok {
		uploaderID = &claims.UserID
	}

	result, err := h.usecase.UploadFile(r.Context(), attusecase.UploadFileCommand{
		TenantID:   tenantID,
		Filename:   header.Filename,
		MimeType:   mimeType,
		Reader:     file,
		SizeBytes:  header.Size,
		UploadedBy: uploaderID,
		IsPublic:   isPublic,
	})
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

func (h *AttachmentHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid file ID format")
		return
	}

	file, err := h.usecase.GetFileMetadata(r.Context(), tenantID, id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, file)
}

func (h *AttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid file ID format")
		return
	}

	file, rc, err := h.usecase.GetFile(r.Context(), tenantID, id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}
	defer rc.Close()

	disposition := "inline"
	if r.URL.Query().Get("disposition") == "attachment" {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, file.Filename))

	_, _ = io.Copy(w, rc)
}

func (h *AttachmentHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid file ID format")
		return
	}

	if err := h.usecase.DeleteFile(r.Context(), tenantID, id); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "file deleted successfully",
	})
}

func (h *AttachmentHandler) Attach(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	var cmd attusecase.AttachFileCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.TenantID = tenantID

	att, err := h.usecase.AttachFile(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, att)
}

func (h *AttachmentHandler) ListByEntity(w http.ResponseWriter, r *http.Request) {
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

	attachments, err := h.usecase.ListEntityAttachments(r.Context(), tenantID, entityType, entityID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, attachments)
}

func (h *AttachmentHandler) Detach(w http.ResponseWriter, r *http.Request) {
	tenantID, err := shared.RequireTenantID(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid attachment ID format")
		return
	}

	if err := h.usecase.DetachFile(r.Context(), tenantID, id); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "attachment unlinked successfully",
	})
}
