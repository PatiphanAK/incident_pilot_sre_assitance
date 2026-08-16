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
	ErrSKUExists         = errors.New("sku already exists")
)

// Product is a stockable item: an id, sku, name, optional description, quantity
// on hand, and unit price. Quantity lives in its own inventory table (one row per
// product); the repository joins it in. Price is a float64 in the domain; the
// outbound repository stores it as DECIMAL in stock_db and converts at the
// boundary.
type Product struct {
	ID          string    `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    int       `json:"quantity"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProductRepository is the outbound port for product persistence (stock_db).
type ProductRepository interface {
	// Create inserts the product and its inventory row (initial quantity) together,
	// in one transaction. It returns ErrSKUExists when the sku is already taken.
	Create(ctx context.Context, p *Product) error
	GetByID(ctx context.Context, id string) (*Product, error)
	List(ctx context.Context) ([]Product, error)
	// Update sets name/description (each only when non-nil) and price on the
	// product and returns the updated product. It returns ErrProductNotFound when
	// the product does not exist.
	Update(ctx context.Context, id string, name, description *string, price float64) (*Product, error)
	// Delete removes the product; its inventory row goes with it (DB-level cascade).
	// It returns ErrProductNotFound when the product does not exist.
	Delete(ctx context.Context, id string) error
	// SetQuantity sets the product's quantity to the given absolute value (upserting
	// the inventory row). It returns ErrProductNotFound when the product does not
	// exist.
	SetQuantity(ctx context.Context, id string, quantity int) error
	// DecrementQuantity reduces a product's quantity by `by` and returns the updated
	// product. It returns ErrProductNotFound when the product does not exist and
	// ErrInsufficientStock when `by` exceeds the current quantity.
	DecrementQuantity(ctx context.Context, id string, by int) (*Product, error)
	// IncrementQuantity increases a product's quantity by `by` and returns the updated
	// product. It returns ErrProductNotFound when the product does not exist. It is
	// the compensation for a reservation that must be undone.
	IncrementQuantity(ctx context.Context, id string, by int) (*Product, error)
}
