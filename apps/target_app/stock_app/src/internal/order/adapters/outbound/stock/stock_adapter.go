// Package stockadapter adapts the stock context's service to the order
// context's StockPort. It exists so the order context depends only on its own
// domain (order.domain.StockPort) and never imports the stock context directly:
// the order domain's errors are distinct from the stock context's, so this
// adapter translates the stock errors into the order domain's equivalents.
package stockadapter

import (
	"context"
	"errors"

	orderdomain "stock_app/src/internal/order/domain"
	stockapplication "stock_app/src/internal/stock/application"
	stockdomain "stock_app/src/internal/stock/domain"
)

// StockAdapter implements orderdomain.StockPort on top of the stock service.
type StockAdapter struct {
	stock *stockapplication.StockService
}

func NewAdapter(s *stockapplication.StockService) *StockAdapter {
	return &StockAdapter{stock: s}
}

// Compile-time check that the adapter satisfies the port.
var _ orderdomain.StockPort = (*StockAdapter)(nil)

func (a *StockAdapter) Reserve(ctx context.Context, productID string, quantity int) (float64, error) {
	price, err := a.stock.Reserve(ctx, productID, quantity)
	if err != nil {
		return 0, translate(err)
	}
	return price, nil
}

func (a *StockAdapter) Release(ctx context.Context, productID string, quantity int) error {
	return a.stock.Release(ctx, productID, quantity)
}

// translate maps stock-context errors to the order domain's equivalents, so the
// order context (and its HTTP handler) can react to them without importing the
// stock context.
func translate(err error) error {
	switch {
	case errors.Is(err, stockdomain.ErrInsufficientStock):
		return orderdomain.ErrInsufficientStock
	case errors.Is(err, stockdomain.ErrProductNotFound):
		return orderdomain.ErrProductNotFound
	default:
		return err
	}
}
