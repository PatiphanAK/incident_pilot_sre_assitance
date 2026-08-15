package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/order/application"
	"stock_app/src/internal/order/domain"
)

type Handler struct {
	service *application.OrderService
	tokens  *auth.TokenService
}

func NewHandler(service *application.OrderService, tokens *auth.TokenService) *Handler {
	return &Handler{service: service, tokens: tokens}
}

type placeOrderRequest struct {
	UserID string                `json:"user_id" form:"user_id"`
	Items  []domain.OrderItemInput `json:"items"`
}

// RegisterRoutes mounts the order routes (behind the bearer-token middleware) on
// the given router.
func RegisterRoutes(router fiber.Router, h *Handler) {
	orders := router.Group("/orders", auth.RequireAuth(h.tokens))
	orders.Post("/", h.Place)
	orders.Get("/", h.List)
	orders.Get("/:id", h.Get)
}

// Place handles POST /api/orders — reserves stock for every line (in-process) and
// creates the order.
func (h *Handler) Place(c fiber.Ctx) error {
	var req placeOrderRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	// If no user_id in the body, use the authenticated user from the bearer token.
	if req.UserID == "" {
		if uid, ok := c.Locals("userID").(string); ok {
			req.UserID = uid
		}
	}
	if req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}
	order, err := h.service.Place(c.Context(), domain.PlaceOrderInput{UserID: req.UserID, Items: req.Items})
	if err != nil {
		return h.mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(order)
}

// Get handles GET /api/orders/:id
func (h *Handler) Get(c fiber.Ctx) error {
	order, err := h.service.GetOrder(c.Context(), c.Params("id"))
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(order)
}

// List handles GET /api/orders
func (h *Handler) List(c fiber.Ctx) error {
	orders, err := h.service.ListOrders(c.Context())
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(orders)
}

func (h *Handler) mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientStock):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrProductNotFound):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "product not found"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
