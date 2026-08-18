package application

import (
	"context"
	"errors"
	"testing"

	"stock_app/src/internal/stock/domain"
)

// fakeProductRepo is an in-memory implementation of domain.ProductRepository.
type fakeProductRepo struct {
	products map[string]*domain.Product
}

func newFakeProductRepo() *fakeProductRepo {
	return &fakeProductRepo{products: make(map[string]*domain.Product)}
}

func (r *fakeProductRepo) seed(id, sku, name, description string, quantity int, price float64) {
	r.products[id] = &domain.Product{ID: id, SKU: sku, Name: name, Description: description, Quantity: quantity, Price: price}
}

func (r *fakeProductRepo) Create(ctx context.Context, p *domain.Product) error {
	for _, existing := range r.products {
		if existing.SKU == p.SKU {
			return domain.ErrSKUExists
		}
	}
	cp := *p
	r.products[p.ID] = &cp
	return nil
}

func (r *fakeProductRepo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *fakeProductRepo) List(ctx context.Context) ([]domain.Product, error) {
	out := make([]domain.Product, 0)
	for _, p := range r.products {
		out = append(out, *p)
	}
	return out, nil
}

func (r *fakeProductRepo) Update(ctx context.Context, id string, name, description *string, price float64) (*domain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	if name != nil {
		p.Name = *name
	}
	if description != nil {
		p.Description = *description
	}
	p.Price = price
	cp := *p
	return &cp, nil
}

func (r *fakeProductRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.products[id]; !ok {
		return domain.ErrProductNotFound
	}
	delete(r.products, id)
	return nil
}

func (r *fakeProductRepo) SetQuantity(ctx context.Context, id string, quantity int) error {
	p, ok := r.products[id]
	if !ok {
		return domain.ErrProductNotFound
	}
	p.Quantity = quantity
	return nil
}

func (r *fakeProductRepo) DecrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	if p.Quantity < by {
		return nil, domain.ErrInsufficientStock
	}
	p.Quantity -= by
	cp := *p
	return &cp, nil
}

func (r *fakeProductRepo) IncrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	p.Quantity += by
	cp := *p
	return &cp, nil
}

func TestStockService_CreateProduct_Valid(t *testing.T) {
	repo := newFakeProductRepo()
	svc := NewStockService(repo)

	p, err := svc.CreateProduct(context.Background(), "SKU-1", "Widget", "A useful widget", 10, 5.5)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	if p.ID == "" || p.SKU != "SKU-1" || p.Name != "Widget" || p.Description != "A useful widget" {
		t.Errorf("unexpected product: %+v", p)
	}
	if p.Quantity != 10 || p.Price != 5.5 {
		t.Errorf("Quantity/Price = %d/%v, want 10/5.5", p.Quantity, p.Price)
	}
	if stored, ok := repo.products[p.ID]; !ok || stored.SKU != "SKU-1" {
		t.Errorf("product not stored as expected: %+v", stored)
	}
}

func TestStockService_CreateProduct_Validation(t *testing.T) {
	svc := NewStockService(newFakeProductRepo())
	cases := []struct {
		name     string
		sku      string
		product  string
		quantity int
		price    float64
	}{
		{"missing sku", "", "Widget", 0, 0},
		{"missing name", "SKU-1", "", 0, 0},
		{"negative quantity", "SKU-1", "Widget", -1, 0},
		{"negative price", "SKU-1", "Widget", 0, -0.01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateProduct(context.Background(), tc.sku, tc.product, "", tc.quantity, tc.price)
			if err == nil {
				t.Fatalf("CreateProduct() error = nil, want an error")
			}
		})
	}
}

func TestStockService_CreateProduct_DuplicateSKU(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "", 5, 5.0)
	svc := NewStockService(repo)

	_, err := svc.CreateProduct(context.Background(), "SKU-1", "Other", "", 1, 1.0)
	if !errors.Is(err, domain.ErrSKUExists) {
		t.Fatalf("CreateProduct() error = %v, want ErrSKUExists", err)
	}
}

func TestStockService_UpdateProduct_Partial(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "Old description", 5, 5.0)
	svc := NewStockService(repo)

	newName := "Gadget"
	updated, err := svc.UpdateProduct(context.Background(), "p1", &newName, nil, 7.25)
	if err != nil {
		t.Fatalf("UpdateProduct() error = %v", err)
	}
	if updated.Name != "Gadget" {
		t.Errorf("Name = %q, want Gadget", updated.Name)
	}
	// nil description -> the current value is kept.
	if updated.Description != "Old description" {
		t.Errorf("Description = %q, want the old value kept", updated.Description)
	}
	if updated.Price != 7.25 {
		t.Errorf("Price = %v, want 7.25", updated.Price)
	}
}

func TestStockService_UpdateProduct_NegativePrice(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "", 5, 5.0)
	svc := NewStockService(repo)

	_, err := svc.UpdateProduct(context.Background(), "p1", nil, nil, -1)
	if err == nil {
		t.Fatalf("UpdateProduct() error = nil, want an error for a negative price")
	}
}

func TestStockService_UpdateProduct_NotFound(t *testing.T) {
	svc := NewStockService(newFakeProductRepo())
	_, err := svc.UpdateProduct(context.Background(), "missing", nil, nil, 1)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("UpdateProduct() error = %v, want ErrProductNotFound", err)
	}
}

func TestStockService_DeleteProduct(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "", 5, 5.0)
	svc := NewStockService(repo)

	if err := svc.DeleteProduct(context.Background(), "p1"); err != nil {
		t.Fatalf("DeleteProduct() error = %v", err)
	}
	if _, ok := repo.products["p1"]; ok {
		t.Errorf("product still present after delete")
	}
	if err := svc.DeleteProduct(context.Background(), "p1"); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("second DeleteProduct() error = %v, want ErrProductNotFound", err)
	}
}

func TestStockService_SetQuantity(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "", 5, 5.0)
	svc := NewStockService(repo)

	if err := svc.SetQuantity(context.Background(), "p1", 42); err != nil {
		t.Fatalf("SetQuantity() error = %v", err)
	}
	if p, _ := repo.GetByID(context.Background(), "p1"); p.Quantity != 42 {
		t.Errorf("quantity = %d, want 42", p.Quantity)
	}

	if err := svc.SetQuantity(context.Background(), "p1", -1); err == nil {
		t.Errorf("SetQuantity(-1) error = nil, want an error")
	}
	if err := svc.SetQuantity(context.Background(), "missing", 1); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("SetQuantity(missing) error = %v, want ErrProductNotFound", err)
	}
}

func TestStockService_ReserveAndRelease(t *testing.T) {
	repo := newFakeProductRepo()
	repo.seed("p1", "SKU-1", "Widget", "", 10, 5.0)
	svc := NewStockService(repo)

	price, err := svc.Reserve(context.Background(), "p1", 3)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if price != 5.0 {
		t.Errorf("price = %v, want 5.0", price)
	}
	if p, _ := repo.GetByID(context.Background(), "p1"); p.Quantity != 7 {
		t.Errorf("quantity after reserve = %d, want 7", p.Quantity)
	}

	// Reserve more than is on hand -> insufficient stock, quantity unchanged.
	if _, err := svc.Reserve(context.Background(), "p1", 8); !errors.Is(err, domain.ErrInsufficientStock) {
		t.Errorf("Reserve(8) error = %v, want ErrInsufficientStock", err)
	}

	if err := svc.Release(context.Background(), "p1", 3); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if p, _ := repo.GetByID(context.Background(), "p1"); p.Quantity != 10 {
		t.Errorf("quantity after release = %d, want 10", p.Quantity)
	}

	if _, err := svc.Reserve(context.Background(), "missing", 1); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("Reserve(missing) error = %v, want ErrProductNotFound", err)
	}
	if err := svc.Release(context.Background(), "missing", 1); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("Release(missing) error = %v, want ErrProductNotFound", err)
	}
	if _, err := svc.Reserve(context.Background(), "p1", 0); err == nil {
		t.Errorf("Reserve(0) error = nil, want an error")
	}
}
