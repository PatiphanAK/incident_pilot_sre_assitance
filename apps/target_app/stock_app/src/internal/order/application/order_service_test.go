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
	r.products[id] = &stockdomain.Product{ID: id, SKU: "SKU-" + id, Name: name, Quantity: quantity, Price: price}
}

func (r *fakeProductRepo) Create(ctx context.Context, p *stockdomain.Product) error {
	for _, existing := range r.products {
		if existing.SKU == p.SKU {
			return stockdomain.ErrSKUExists
		}
	}
	cp := *p
	r.products[p.ID] = &cp
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

func (r *fakeProductRepo) Update(ctx context.Context, id string, name, description *string, price float64) (*stockdomain.Product, error) {
	p, ok := r.products[id]
	if !ok {
		return nil, stockdomain.ErrProductNotFound
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
		return stockdomain.ErrProductNotFound
	}
	delete(r.products, id)
	return nil
}

func (r *fakeProductRepo) SetQuantity(ctx context.Context, id string, quantity int) error {
	p, ok := r.products[id]
	if !ok {
		return stockdomain.ErrProductNotFound
	}
	p.Quantity = quantity
	return nil
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

func (r *fakeOrderRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	o, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	o.Status = status
	return nil
}

func (r *fakeOrderRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.orders[id]; !ok {
		return domain.ErrOrderNotFound
	}
	delete(r.orders, id)
	return nil
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

func TestOrderService_UpdateOrderStatus_Transitions(t *testing.T) {
	cases := []struct {
		from string
		to   string
		ok   bool
	}{
		{domain.StatusPending, domain.StatusPaid, true},
		{domain.StatusPending, domain.StatusCancelled, true},
		{domain.StatusPaid, domain.StatusShipped, true},
		{domain.StatusPaid, domain.StatusCancelled, true},
		{domain.StatusShipped, domain.StatusPaid, false},     // terminal
		{domain.StatusShipped, domain.StatusCancelled, false}, // terminal
		{domain.StatusCancelled, domain.StatusPending, false}, // terminal
		{domain.StatusCancelled, domain.StatusPaid, false},    // terminal
		{domain.StatusPending, domain.StatusPending, false},   // same status is not a transition
		{domain.StatusPending, "REFUNDED", false},             // unknown status
	}
	for _, tc := range cases {
		t.Run(tc.from+"_to_"+tc.to, func(t *testing.T) {
			stockRepo := newFakeProductRepo()
			orderRepo := newFakeOrderRepo()
			svc := newService(stockRepo, orderRepo)
			order := &domain.Order{
				ID:     "o1",
				UserID: "u1",
				Status: tc.from,
				Items:  []domain.OrderItem{{ProductID: "p1", Quantity: 2, PriceAtPurchase: 5.0}},
			}
			orderRepo.orders[order.ID] = order

			got, err := svc.UpdateOrderStatus(context.Background(), "o1", tc.to)
			if tc.ok {
				if err != nil {
					t.Fatalf("UpdateOrderStatus() error = %v, want none", err)
				}
				if got.Status != tc.to {
					t.Errorf("Status = %q, want %q", got.Status, tc.to)
				}
				if orderRepo.orders["o1"].Status != tc.to {
					t.Errorf("persisted status = %q, want %q", orderRepo.orders["o1"].Status, tc.to)
				}
			} else {
				if !errors.Is(err, domain.ErrInvalidStatusTransition) {
					t.Fatalf("UpdateOrderStatus() error = %v, want ErrInvalidStatusTransition", err)
				}
				if orderRepo.orders["o1"].Status != tc.from {
					t.Errorf("persisted status = %q, want %q (unchanged)", orderRepo.orders["o1"].Status, tc.from)
				}
			}
		})
	}
}

func TestOrderService_UpdateOrderStatus_NotFound(t *testing.T) {
	stockRepo := newFakeProductRepo()
	orderRepo := newFakeOrderRepo()
	svc := newService(stockRepo, orderRepo)

	_, err := svc.UpdateOrderStatus(context.Background(), "missing", domain.StatusPaid)
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("UpdateOrderStatus() error = %v, want ErrOrderNotFound", err)
	}
}

func TestOrderService_UpdateOrderStatus_CancelReleasesStock(t *testing.T) {
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
	if p, _ := stockRepo.GetByID(context.Background(), "p1"); p.Quantity != 7 {
		t.Fatalf("precondition: stock quantity = %d, want 7", p.Quantity)
	}

	updated, err := svc.UpdateOrderStatus(context.Background(), order.ID, domain.StatusCancelled)
	if err != nil {
		t.Fatalf("UpdateOrderStatus(cancel) error = %v", err)
	}
	if updated.Status != domain.StatusCancelled {
		t.Errorf("Status = %q, want CANCELLED", updated.Status)
	}
	// Cancelling is the business operation that releases the reserved stock.
	if p, _ := stockRepo.GetByID(context.Background(), "p1"); p.Quantity != 10 {
		t.Errorf("stock quantity = %d, want 10 (released on cancel)", p.Quantity)
	}
}

func TestOrderService_DeleteOrder(t *testing.T) {
	stockRepo := newFakeProductRepo()
	stockRepo.seed("p1", "Widget", 10, 5.0)
	orderRepo := newFakeOrderRepo()
	svc := newService(stockRepo, orderRepo)

	order, err := svc.Place(context.Background(), domain.PlaceOrderInput{
		UserID: "u1",
		Items:  []domain.OrderItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if err := svc.DeleteOrder(context.Background(), order.ID); err != nil {
		t.Fatalf("DeleteOrder() error = %v", err)
	}
	if _, err := svc.GetOrder(context.Background(), order.ID); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("GetOrder() after delete error = %v, want ErrOrderNotFound", err)
	}
	// Deleting again is a not-found, not a silent success.
	if err := svc.DeleteOrder(context.Background(), order.ID); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("second DeleteOrder() error = %v, want ErrOrderNotFound", err)
	}
}
