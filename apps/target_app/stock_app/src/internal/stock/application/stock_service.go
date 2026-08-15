// Package application holds the stock use cases. It depends only on the stock
// domain (its ports), never on a concrete adapter or on another context.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"stock_app/src/internal/stock/domain"
)

// StockService is the stock use cases. Its Reserve method is what the order
// context calls in-process; structurally it satisfies the order domain's
// StockPort.
type StockService struct {
	repo domain.ProductRepository
}

func NewStockService(repo domain.ProductRepository) *StockService {
	return &StockService{repo: repo}
}

// CreateProduct creates a product with the given name, initial quantity, and price.
func (s *StockService) CreateProduct(ctx context.Context, name string, quantity int, price float64) (*domain.Product, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if quantity < 0 {
		return nil, errors.New("quantity must be non-negative")
	}
	now := time.Now().UTC()
	p := &domain.Product{
		ID:        uuid.NewString(),
		Name:      name,
		Quantity:  quantity,
		Price:     price,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListProducts returns all products, newest first.
func (s *StockService) ListProducts(ctx context.Context) ([]domain.Product, error) {
	return s.repo.List(ctx)
}

// GetProduct returns a product by id.
func (s *StockService) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	return s.repo.GetByID(ctx, id)
}

// Reserve decrements `quantity` units of the product and returns its current unit
// price. This is the in-process call the order context uses to reserve stock; it
// is the method that satisfies the order domain's StockPort.
func (s *StockService) Reserve(ctx context.Context, productID string, quantity int) (float64, error) {
	if quantity <= 0 {
		return 0, errors.New("quantity must be positive")
	}
	p, err := s.repo.DecrementQuantity(ctx, productID, quantity)
	if err != nil {
		return 0, err
	}
	return p.Price, nil
}

// Release re-increments `quantity` units of the product. It is the compensation
// for a reservation that must be undone (e.g. the order could not be created after
// the stock was reserved). Part of the order domain's StockPort.
func (s *StockService) Release(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	_, err := s.repo.IncrementQuantity(ctx, productID, quantity)
	return err
}
