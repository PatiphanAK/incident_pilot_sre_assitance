package handlers

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/stock/application"
	"stock_app/src/internal/stock/domain"
)

type Handler struct {
	service *application.StockService
	tokens  *auth.TokenService
}

func NewHandler(service *application.StockService, tokens *auth.TokenService) *Handler {
	return &Handler{service: service, tokens: tokens}
}

type createProductRequest struct {
	SKU         string  `json:"sku" form:"sku"`
	Name        string  `json:"name" form:"name"`
	Description string  `json:"description" form:"description"`
	Quantity    int     `json:"quantity" form:"quantity"`
	Price       float64 `json:"price" form:"price"`
}

// updateProductRequest is the PATCH body. name/description are optional (the
// current value is kept when absent); price is required — the update always sets
// it, so a client that does not change it must send the current value back.
type updateProductRequest struct {
	Name        *string  `json:"name" form:"name"`
	Description *string  `json:"description" form:"description"`
	Price       *float64 `json:"price" form:"price"`
}

type setQuantityRequest struct {
	Quantity *int `json:"quantity" form:"quantity"`
}

type reserveRequest struct {
	Quantity int `json:"quantity" form:"quantity"`
}

// RegisterRoutes mounts the stock routes (all behind the bearer-token middleware)
// on the given router.
func RegisterRoutes(router fiber.Router, h *Handler) {
	products := router.Group("/stock/products", auth.RequireAuth(h.tokens))
	products.Post("/", h.Create)
	products.Get("/", h.List)
	products.Get("/:id", h.Get)
	products.Post("/:id/reserve", h.Reserve)
	products.Patch("/:id", h.Update)
	products.Delete("/:id", h.Delete)
	products.Patch("/:id/inventory", h.SetInventory)
}

// Create handles POST /api/v1/stock/products
func (h *Handler) Create(c fiber.Ctx) error {
	var req createProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.SKU == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "sku is required"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	p, err := h.service.CreateProduct(c.Context(), req.SKU, req.Name, req.Description, req.Quantity, req.Price)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(p)
}

// Get handles GET /api/v1/stock/products/:id
func (h *Handler) Get(c fiber.Ctx) error {
	p, err := h.service.GetProduct(c.Context(), c.Params("id"))
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(p)
}

// List handles GET /api/v1/stock/products
func (h *Handler) List(c fiber.Ctx) error {
	products, err := h.service.ListProducts(c.Context())
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(products)
}

// Reserve handles POST /api/v1/stock/products/:id/reserve — decrements a product's
// quantity (mainly a manual/testing entry point; the order context reserves
// in-process instead).
func (h *Handler) Reserve(c fiber.Ctx) error {
	var req reserveRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Quantity <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "quantity must be positive"})
	}
	price, err := h.service.Reserve(c.Context(), c.Params("id"), req.Quantity)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(fiber.Map{"product_id": c.Params("id"), "unit_price": price})
}

// Update handles PATCH /api/v1/stock/products/:id — partial update of
// name/description plus a required new price.
func (h *Handler) Update(c fiber.Ctx) error {
	var req updateProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Price == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "price is required"})
	}
	p, err := h.service.UpdateProduct(c.Context(), c.Params("id"), req.Name, req.Description, *req.Price)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(p)
}

// Delete handles DELETE /api/v1/stock/products/:id
func (h *Handler) Delete(c fiber.Ctx) error {
	if err := h.service.DeleteProduct(c.Context(), c.Params("id")); err != nil {
		return h.mapError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// SetInventory handles PATCH /api/v1/stock/products/:id/inventory — sets the
// product's quantity to the given absolute value.
func (h *Handler) SetInventory(c fiber.Ctx) error {
	var req setQuantityRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Quantity == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "quantity is required"})
	}
	if err := h.service.SetQuantity(c.Context(), c.Params("id"), *req.Quantity); err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(fiber.Map{"product_id": c.Params("id"), "quantity": *req.Quantity})
}

func (h *Handler) mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientStock):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrSKUExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	default:
		slog.Error(
			"unhandled application error",
			"error", err,
			"path", c.Path(),
			"method", c.Method(),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
