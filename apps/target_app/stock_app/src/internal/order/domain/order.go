// Package domain is the order bounded context's domain: the order and order-item
// entities, the outbound OrderRepository port, the StockPort (the in-process
// dependency on the stock context), and the errors. It has no dependency on
// infrastructure or on the concrete stock context.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductNotFound   = errors.New("product not found")
)

// Order status values.
const (
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusShipped   = "SHIPPED"
	StatusCancelled = "CANCELLED"
)

// OrderItem is one line of an order. PriceAtPurchase snapshots the unit price at
// the time of purchase (the stock service may reprice a product later, which must
// not rewrite an existing order's history).
type OrderItem struct {
	ID              string  `json:"id,omitempty"`
	OrderID         string  `json:"order_id,omitempty"`
	ProductID       string  `json:"product_id"`
	Quantity        int     `json:"quantity"`
	PriceAtPurchase float64 `json:"price_at_purchase"`
}

// Order is a customer's order: id, owner, status, total, and its items.
type Order struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	Status     string      `json:"status"`
	TotalPrice float64     `json:"total_price"`
	CreatedAt  time.Time   `json:"created_at"`
	Items      []OrderItem `json:"items"`
}

// OrderItemInput is one requested line when placing an order.
type OrderItemInput struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// PlaceOrderInput is the request to place an order.
type PlaceOrderInput struct {
	UserID string           `json:"user_id"`
	Items  []OrderItemInput `json:"items"`
}

// OrderRepository is the outbound port for order persistence (order_db).
type OrderRepository interface {
	// Create inserts the order and its items together (one transaction, same database).
	Create(ctx context.Context, o *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context) ([]Order, error)
}

// StockPort is the in-process dependency on the stock context. It is satisfied by
// the stock context's StockService (see the router wiring). Because stock lives in
// its own database, reserving stock and creating the order are NOT one
// transaction — see OrderService.Place.
type StockPort interface {
	// Reserve decrements `quantity` units of the product and returns its current
	// unit price. It returns ErrProductNotFound or ErrInsufficientStock.
	Reserve(ctx context.Context, productID string, quantity int) (price float64, err error)
	// Release re-increments `quantity` units of the product. It is the compensation
	// for a reservation that must be undone (the order could not be created after
	// the stock was reserved).
	Release(ctx context.Context, productID string, quantity int) error
}
