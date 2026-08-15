// Package application holds the order use cases. It depends only on the order
// domain (its ports), never on a concrete adapter or on the concrete stock
// context — that dependency is expressed through the domain.StockPort.
package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"stock_app/src/internal/order/domain"
)

type OrderService struct {
	orders domain.OrderRepository
	stock  domain.StockPort
}

func NewOrderService(orders domain.OrderRepository, stock domain.StockPort) *OrderService {
	return &OrderService{orders: orders, stock: stock}
}

// Place reserves stock for every line (in-process, stock_db) and, if all lines
// reserve, creates the order (order_db).
//
// Because stock and order live in DIFFERENT databases, this is NOT atomic: if the
// order write fails after the reservations succeeded, the reserved stock is left
// decremented without an order. That is an eventual-consistency gap — the cost of
// the separate-databases choice. A production version would add a compensating
// action or an outbox / saga.
func (s *OrderService) Place(ctx context.Context, input domain.PlaceOrderInput) (*domain.Order, error) {
	if input.UserID == "" {
		return nil, errors.New("user_id is required")
	}
	if len(input.Items) == 0 {
		return nil, errors.New("order must have at least one item")
	}

	// 1. Reserve stock for each line first. If any line cannot be reserved, the
	// order is not created and the request fails.
	var items []domain.OrderItem
	var total float64
	for _, in := range input.Items {
		if in.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		price, err := s.stock.Reserve(ctx, in.ProductID, in.Quantity)
		if err != nil { // ErrProductNotFound or ErrInsufficientStock
			return nil, err
		}
		total += price * float64(in.Quantity)
		items = append(items, domain.OrderItem{
			ProductID:       in.ProductID,
			Quantity:        in.Quantity,
			PriceAtPurchase: price,
		})
	}

	// 2. Create the order (order_db). If this fails, compensate by releasing the
	// stock we reserved (different database — see the method comment).
	now := time.Now().UTC()
	order := &domain.Order{
		ID:         uuid.NewString(),
		UserID:     input.UserID,
		Status:     domain.StatusPending,
		TotalPrice: total,
		CreatedAt:  now,
		Items:      items,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		s.compensate(ctx, items)
		return nil, err
	}
	return order, nil
}

// compensate releases (re-increments) the stock reserved for the given items. It
// is best-effort: a release that itself fails is logged and left for a later
// reconciliation. This narrows — but does not fully close — the eventual-
// consistency gap of separate databases; a durable version would record the
// compensation in an outbox / saga log.
func (s *OrderService) compensate(ctx context.Context, items []domain.OrderItem) {
	for _, it := range items {
		if err := s.stock.Release(ctx, it.ProductID, it.Quantity); err != nil {
			slog.Warn("failed to release reserved stock",
				"product_id", it.ProductID, "quantity", it.Quantity, "error", err)
		}
	}
}

// ListOrders returns all orders, newest first.
func (s *OrderService) ListOrders(ctx context.Context) ([]domain.Order, error) {
	return s.orders.List(ctx)
}

// GetOrder returns an order (with its items) by id.
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	return s.orders.GetByID(ctx, id)
}
