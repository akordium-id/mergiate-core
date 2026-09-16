package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/mergiate-core/internal/core/delivery/http/middleware"
	domainproduct "github.com/akordium-id/mergiate-core/internal/core/domain/product"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/usecase/product"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

type ProductHandler struct {
	usecase product.Usecase
}

func NewProductHandler(usecase product.Usecase) *ProductHandler {
	return &ProductHandler{usecase: usecase}
}

func (h *ProductHandler) RegisterRoutes(r chi.Router) {
	// Units routes
	r.Route("/units", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:read"))
			r.Get("/", h.ListUnits)
			r.Get("/{id}", h.GetUnitByID)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:create"))
			r.Post("/", h.CreateUnit)
			r.Post("/conversions", h.CreateConversion)
			r.Post("/convert", h.ConvertQuantity)
		})
	})

	// Products routes
	r.Route("/products", func(r chi.Router) {
		r.Use(middleware.TenantRequired())

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:read"))
			r.Get("/", h.ListProducts)
			r.Get("/{id}", h.GetProductByID)
			r.Get("/{id}/variants", h.ListVariants)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:create"))
			r.Post("/", h.CreateProduct)
			r.Post("/{id}/variants", h.CreateVariant)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:update"))
			r.Put("/{id}", h.UpdateProduct)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission("product:delete"))
			r.Delete("/variants/{variant_id}", h.DeleteVariant)
		})
	})
}


// ----------------------------------------------------------------------------
// Unit Handlers
// ----------------------------------------------------------------------------

func (h *ProductHandler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	var cmd product.CreateUnitCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	res, err := h.usecase.CreateUnit(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

func (h *ProductHandler) ListUnits(w http.ResponseWriter, r *http.Request) {
	res, err := h.usecase.ListUnits(r.Context())
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) GetUnitByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	res, err := h.usecase.GetUnitByID(r.Context(), id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) CreateConversion(w http.ResponseWriter, r *http.Request) {
	var cmd product.CreateConversionCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	res, err := h.usecase.CreateConversion(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

func (h *ProductHandler) ConvertQuantity(w http.ResponseWriter, r *http.Request) {
	var cmd product.ConvertQuantityCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	converted, err := h.usecase.ConvertQuantity(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"amount":           cmd.Amount,
		"from_unit_id":     cmd.FromUnitID,
		"to_unit_id":       cmd.ToUnitID,
		"converted_amount": converted,
	})
}

// ----------------------------------------------------------------------------
// Product Handlers
// ----------------------------------------------------------------------------

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var cmd product.CreateProductCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}

	res, err := h.usecase.CreateProduct(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page := int32(1)
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}

	pageSize := int32(20)
	if ps, err := strconv.Atoi(query.Get("page_size")); err == nil && ps > 0 {
		pageSize = int32(ps)
	}

	var prodType *domainproduct.Type
	if t := query.Get("type"); t != "" {
		pt := domainproduct.Type(t)
		prodType = &pt
	}

	var status *domainproduct.Status
	if s := query.Get("status"); s != "" {
		st := domainproduct.Status(s)
		status = &st
	}

	res, err := h.usecase.ListProducts(r.Context(), page, pageSize, prodType, status)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	res, err := h.usecase.GetProductByID(r.Context(), id)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid UUID format")
		return
	}

	var cmd product.UpdateProductCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.ID = id

	res, err := h.usecase.UpdateProduct(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	productID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid Product UUID format")
		return
	}

	var cmd product.CreateVariantCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_BODY", "Malformed request body")
		return
	}
	cmd.ProductID = productID

	res, err := h.usecase.CreateVariant(r.Context(), cmd)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

func (h *ProductHandler) ListVariants(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	productID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid Product UUID format")
		return
	}

	res, err := h.usecase.ListVariants(r.Context(), productID)
	if err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *ProductHandler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "variant_id")
	variantID, err := shared.ParseID(idStr)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "INVALID_ID", "Invalid Variant UUID format")
		return
	}

	if err := h.usecase.DeleteVariant(r.Context(), variantID); err != nil {
		response.FromDomainError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "variant deleted successfully",
	})
}
