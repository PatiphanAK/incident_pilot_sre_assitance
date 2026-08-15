package handlers

import (
	"errors"

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
	Name     string  `json:"name" form:"name"`
	Quantity int     `json:"quantity" form:"quantity"`
	Price    float64 `json:"price" form:"price"`
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
}

// Create handles POST /api/v1/stock/products
func (h *Handler) Create(c fiber.Ctx) error {
	var req createProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	p, err := h.service.CreateProduct(c.Context(), req.Name, req.Quantity, req.Price)
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

func (h *Handler) mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientStock):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
