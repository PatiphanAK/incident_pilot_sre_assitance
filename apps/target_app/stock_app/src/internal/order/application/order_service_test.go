package application

import (
	"context"
	"errors"
	"testing"

	"stock_app/src/internal/order/adapters/outbound/stock" // package stockadapter
	"stock_app/src/internal/order/domain"
	stockapp "stock_app/src/internal/stock/application"
	stockdomain "stock_app/src/internal/stock/domain"
)

// fakeProductRepo is an in-memory implementation of stock.ProductRepository.
type fakeProductRepo struct {
	products map[string]*stockdomain.Product
}

func newFakeProductRepo() *fakeProductRepo {
	return &fakeProductRepo{products: make(map[string]*stockdomain.Product)}
}

func (r *fakeProductRepo) seed(id, name string, quantity int, price float64) {
	r.products[id] = &stockdomain.Product{ID: id, Name: name, Quantity: quantity, Price: price}
}

func (r *fakeProductRepo) Create(ctx context.Context, p *stockdomain.Product) error {
	r.products[p.ID] = p
	return nil
}

func (r *fakeProductRepo) GetByID(ctx context.Context, id string) (*stockdomain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, stockdomain.ErrProductNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *fakeProductRepo) List(ctx context.Context) ([]stockdomain.Product, error) {
	out := make([]stockdomain.Product, 0)
	for _, p := range r.products {
		out = append(out, *p)
	}
	return out, nil
}

func (r *fakeProductRepo) DecrementQuantity(ctx context.Context, id string, by int) (*stockdomain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, stockdomain.ErrProductNotFound
	}
	if p.Quantity < by {
		return nil, stockdomain.ErrInsufficientStock
	}
	p.Quantity -= by
	cp := *p
	return &cp, nil
}

func (r *fakeProductRepo) IncrementQuantity(ctx context.Context, id string, by int) (*stockdomain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, stockdomain.ErrProductNotFound
	}
	p.Quantity += by
	cp := *p
	return &cp, nil
}

// fakeOrderRepo is an in-memory implementation of order.OrderRepository.
type fakeOrderRepo struct {
	orders     map[string]*domain.Order
	failCreate bool
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{orders: make(map[string]*domain.Order)}
}

func (r *fakeOrderRepo) Create(ctx context.Context, o *domain.Order) error {
	if r.failCreate {
		return errors.New("order create failed")
	}
	r.orders[o.ID] = o
	return nil
}

func (r *fakeOrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (r *fakeOrderRepo) List(ctx context.Context) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	for _, o := range r.orders {
		out = append(out, *o)
	}
	return out, nil
}

func newService(stockRepo *fakeProductRepo, orderRepo *fakeOrderRepo) *OrderService {
	// The real StockService (behind the StockPort adapter, which also translates
	// errors) is wired as the order's StockPort — this tests the in-process
	// reserve->order flow, not a stub.
	return NewOrderService(orderRepo, stockadapter.NewAdapter(stockapp.NewStockService(stockRepo)))
}

func TestOrderService_Place_HappyPath(t *testing.T) {
	stockRepo := newFakeProductRepo()
	stockRepo.seed("p1", "Widget", 10, 5.0)
	orderRepo := newFakeOrderRepo()
	svc := newService(stockRepo, orderRepo)

	order, err := svc.Place(context.Background(), domain.PlaceOrderInput{
		UserID: "u1",
		Items:  []domain.OrderItemInput{{ProductID: "p1", Quantity: 3}},
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if order.TotalPrice != 15.0 {
		t.Errorf("TotalPrice = %v, want 15.0", order.TotalPrice)
	}
	if len(order.Items) != 1 || order.Items[0].PriceAtPurchase != 5.0 {
		t.Errorf("unexpected items: %+v", order.Items)
	}
	if order.Status != domain.StatusPending {
		t.Errorf("Status = %q, want PENDING", order.Status)
	}
	// Stock was reserved in-process.
	if p, _ := stockRepo.GetByID(context.Background(), "p1"); p.Quantity != 7 {
		t.Errorf("stock quantity = %d, want 7", p.Quantity)
	}
	// The order was persisted.
	if _, ok := orderRepo.orders[order.ID]; !ok {
		t.Errorf("order %s not persisted", order.ID)
	}
}

func TestOrderService_Place_InsufficientStock(t *testing.T) {
	stockRepo := newFakeProductRepo()
	stockRepo.seed("p1", "Widget", 2, 5.0)
	orderRepo := newFakeOrderRepo()
	svc := newService(stockRepo, orderRepo)

	_, err := svc.Place(context.Background(), domain.PlaceOrderInput{
		UserID: "u1",
		Items:  []domain.OrderItemInput{{ProductID: "p1", Quantity: 5}},
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("Place() error = %v, want ErrInsufficientStock", err)
	}
	// Stock untouched, and no order created.
	if p, _ := stockRepo.GetByID(context.Background(), "p1"); p.Quantity != 2 {
		t.Errorf("stock quantity = %d, want 2 (unchanged)", p.Quantity)
	}
	if len(orderRepo.orders) != 0 {
		t.Errorf("expected no orders, got %d", len(orderRepo.orders))
	}
}

func TestOrderService_Place_ProductNotFound(t *testing.T) {
	stockRepo := newFakeProductRepo()
	orderRepo := newFakeOrderRepo()
	svc := newService(stockRepo, orderRepo)

	_, err := svc.Place(context.Background(), domain.PlaceOrderInput{
		UserID: "u1",
		Items:  []domain.OrderItemInput{{ProductID: "missing", Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("Place() error = %v, want ErrProductNotFound", err)
	}
}

func TestOrderService_Place_CompensatesOnCreateFailure(t *testing.T) {
	stockRepo := newFakeProductRepo()
	stockRepo.seed("p1", "Widget", 10, 5.0)
	orderRepo := newFakeOrderRepo()
	orderRepo.failCreate = true
	svc := newService(stockRepo, orderRepo)

	_, err := svc.Place(context.Background(), domain.PlaceOrderInput{
		UserID: "u1",
		Items:  []domain.OrderItemInput{{ProductID: "p1", Quantity: 3}},
	})
	if err == nil {
		t.Fatalf("Place() error = nil, want an error")
	}
	// The order creation failed, so the reserved stock must be released (compensated).
	if p, _ := stockRepo.GetByID(context.Background(), "p1"); p.Quantity != 10 {
		t.Errorf("stock quantity = %d, want 10 (compensated)", p.Quantity)
	}
	if len(orderRepo.orders) != 0 {
		t.Errorf("expected no persisted orders, got %d", len(orderRepo.orders))
	}
}
