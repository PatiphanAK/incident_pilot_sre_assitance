// Package domain is the stock bounded context's domain: the product entity, the
// outbound ProductRepository port, and the use cases' dependencies. It has no
// dependency on any infrastructure or on any other context.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// Product is a stockable item: an id, name, quantity on hand, and unit price.
// Price is a float64 in the domain; the outbound repository stores it as DECIMAL
// in stock_db and converts at the boundary.
type Product struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProductRepository is the outbound port for product persistence (stock_db).
type ProductRepository interface {
	Create(ctx context.Context, p *Product) error
	GetByID(ctx context.Context, id string) (*Product, error)
	List(ctx context.Context) ([]Product, error)
	// DecrementQuantity reduces a product's quantity by `by` and returns the updated
	// product. It returns ErrProductNotFound when the product does not exist and
	// ErrInsufficientStock when `by` exceeds the current quantity.
	DecrementQuantity(ctx context.Context, id string, by int) (*Product, error)
	// IncrementQuantity increases a product's quantity by `by` and returns the updated
	// product. It returns ErrProductNotFound when the product does not exist. It is
	// the compensation for a reservation that must be undone.
	IncrementQuantity(ctx context.Context, id string, by int) (*Product, error)
}
