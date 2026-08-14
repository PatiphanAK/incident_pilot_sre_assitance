package handlers

import (
	"errors"
	netmail "net/mail"

	"github.com/gofiber/fiber/v3"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/user/application"
	"stock_app/src/internal/user/domain"
)

type Handler struct {
	service *application.UserService
	tokens  *auth.TokenService
}

func NewHandler(service *application.UserService, tokens *auth.TokenService) *Handler {
	return &Handler{service: service, tokens: tokens}
}

type userRequest struct {
	Username string `json:"username" form:"username"`
	Email    string `json:"email" form:"email"`
}

type registerRequest struct {
	Username string `json:"username" form:"username"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

type loginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// RegisterRoutes mounts the auth routes (public) and the user CRUD routes
// (behind the bearer-token middleware) on the given router.
func RegisterRoutes(router fiber.Router, h *Handler) {
	authRoutes := router.Group("/auth")
	authRoutes.Post("/register", h.Register)
	authRoutes.Post("/login", h.Login)
	authRoutes.Get("/me", auth.RequireAuth(h.tokens), h.Me)

	users := router.Group("/users", auth.RequireAuth(h.tokens))
	users.Post("/", h.Create)
	users.Get("/", h.List)
	users.Get("/:id", h.Get)
	users.Put("/:id", h.Update)
	users.Delete("/:id", h.Delete)
}

// Register handles POST /api/auth/register
func (h *Handler) Register(c fiber.Ctx) error {
	var req registerRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username is required"})
	}
	if _, err := netmail.ParseAddress(req.Email); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is invalid"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
	}
	user, err := h.service.Register(c.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

// Login handles POST /api/auth/login and returns a signed bearer token.
func (h *Handler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	user, err := h.service.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return h.mapError(c, err)
	}
	token, err := h.tokens.Sign(user.ID, user.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
	return c.JSON(fiber.Map{"token": token, "user": user})
}

// Me handles GET /api/auth/me for the user identified by the bearer token.
func (h *Handler) Me(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	user, err := h.service.GetUser(c.Context(), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(user)
}

// Create handles POST /api/users
func (h *Handler) Create(c fiber.Ctx) error {
	req, err := h.bind(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	user, err := h.service.CreateUser(c.Context(), req.Username, req.Email)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

// Get handles GET /api/users/:id
func (h *Handler) Get(c fiber.Ctx) error {
	user, err := h.service.GetUser(c.Context(), c.Params("id"))
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(user)
}

// List handles GET /api/users
func (h *Handler) List(c fiber.Ctx) error {
	users, err := h.service.ListUsers(c.Context())
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(users)
}

// Update handles PUT /api/users/:id
func (h *Handler) Update(c fiber.Ctx) error {
	req, err := h.bind(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	user, err := h.service.UpdateUser(c.Context(), c.Params("id"), req.Username, req.Email)
	if err != nil {
		return h.mapError(c, err)
	}
	return c.JSON(user)
}

// Delete handles DELETE /api/users/:id
func (h *Handler) Delete(c fiber.Ctx) error {
	if err := h.service.DeleteUser(c.Context(), c.Params("id")); err != nil {
		return h.mapError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) bind(c fiber.Ctx) (userRequest, error) {
	var req userRequest
	if err := c.Bind().Body(&req); err != nil {
		return req, errors.New("invalid request body")
	}
	if req.Username == "" {
		return req, errors.New("username is required")
	}
	if _, err := netmail.ParseAddress(req.Email); err != nil {
		return req, errors.New("email is invalid")
	}
	return req, nil
}

func (h *Handler) mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrEmailTaken):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidCredentials):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
