package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Grandeath/order-service/internal/order/domain"
)

var (
	testNow = time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	testAddr = domain.Address{
		Street:     "Marszałkowska 1",
		City:       "Warszawa",
		PostalCode: "00-001",
		Country:    "PL",
	}
)

func validItems() []domain.OrderItem {
	return []domain.OrderItem{
		{
			ProductID:   "p1",
			ProductName: "Product 1",
			Quantity:    2,
			UnitPrice:   decimal.NewFromFloat(9.99),
		},
		{
			ProductID:   "p2",
			ProductName: "Product 2",
			Quantity:    1,
			UnitPrice:   decimal.NewFromInt(20),
		},
	}
}

func TestNewOrder_HappyPath(t *testing.T) {
	o, err := domain.NewOrder("ord-1", "cust-1", "PLN", validItems(), testAddr, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != domain.OrderStatusNew {
		t.Errorf("status = %s, want NEW", o.Status)
	}
	want := decimal.NewFromFloat(9.99).Mul(decimal.NewFromInt(2)).Add(decimal.NewFromInt(20))
	if !o.TotalAmount.Equal(want) {
		t.Errorf("total = %s, want %s", o.TotalAmount, want)
	}
	if o.Currency != "PLN" {
		t.Errorf("currency = %s, want PLN", o.Currency)
	}
	if len(o.History) != 1 || o.History[0].To != domain.OrderStatusNew {
		t.Errorf("history not initialised correctly: %#v", o.History)
	}
}

func TestNewOrder_Validation(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		customer   string
		currency   string
		items      []domain.OrderItem
		addr       domain.Address
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:    "empty id",
			id:      "",
			customer: "c1",
			currency: "PLN",
			items:   validItems(),
			addr:    testAddr,
			wantAnyErr: true,
		},
		{
			name:     "no customer",
			id:       "ord-1",
			customer: "",
			currency: "PLN",
			items:    validItems(),
			addr:     testAddr,
			wantErr:  domain.ErrInvalidCustomer,
		},
		{
			name:     "bad currency",
			id:       "ord-1",
			customer: "c1",
			currency: "PL",
			items:    validItems(),
			addr:     testAddr,
			wantErr:  domain.ErrInvalidCurrency,
		},
		{
			name:     "no items",
			id:       "ord-1",
			customer: "c1",
			currency: "PLN",
			items:    nil,
			addr:     testAddr,
			wantErr:  domain.ErrEmptyItems,
		},
		{
			name:     "bad address",
			id:       "ord-1",
			customer: "c1",
			currency: "PLN",
			items:    validItems(),
			addr:     domain.Address{Street: "x"},
			wantErr:  domain.ErrInvalidAddress,
		},
		{
			name:     "zero quantity",
			id:       "ord-1",
			customer: "c1",
			currency: "PLN",
			items: []domain.OrderItem{{
				ProductID: "p", Quantity: 0, UnitPrice: decimal.NewFromInt(1),
			}},
			addr:    testAddr,
			wantErr: domain.ErrInvalidQuantity,
		},
		{
			name:     "negative price",
			id:       "ord-1",
			customer: "c1",
			currency: "PLN",
			items: []domain.OrderItem{{
				ProductID: "p", Quantity: 1, UnitPrice: decimal.NewFromInt(-1),
			}},
			addr:    testAddr,
			wantErr: domain.ErrInvalidUnitPrice,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := domain.NewOrder(c.id, c.customer, c.currency, c.items, c.addr, testNow)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("error = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestOrder_TransitionHappyPath(t *testing.T) {
	o, _ := domain.NewOrder("ord-1", "c1", "PLN", validItems(), testAddr, testNow)

	steps := []domain.OrderStatus{
		domain.OrderStatusValidated,
		domain.OrderStatusPaymentPending,
		domain.OrderStatusPaid,
		domain.OrderStatusProcessing,
		domain.OrderStatusShipped,
	}
	for i, to := range steps {
		if err := o.Transition(to, "", testNow.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatalf("transition to %s failed: %v", to, err)
		}
		if o.Status != to {
			t.Errorf("after step %d: status = %s, want %s", i, o.Status, to)
		}
	}
	if len(o.History) != len(steps)+1 {
		t.Errorf("history length = %d, want %d", len(o.History), len(steps)+1)
	}
}

func TestOrder_InvalidTransition(t *testing.T) {
	o, _ := domain.NewOrder("ord-1", "c1", "PLN", validItems(), testAddr, testNow)
	err := o.Transition(domain.OrderStatusPaid, "", testNow)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestOrder_CancelFromAnyNonTerminal(t *testing.T) {
	cases := []domain.OrderStatus{
		domain.OrderStatusNew,
		domain.OrderStatusValidated,
		domain.OrderStatusPaymentPending,
		domain.OrderStatusPaid,
		domain.OrderStatusProcessing,
	}
	for _, from := range cases {
		t.Run(string(from), func(t *testing.T) {
			o, _ := domain.NewOrder("ord-1", "c1", "PLN", validItems(), testAddr, testNow)
			o.Status = from
			if err := o.Cancel("test", testNow); err != nil {
				t.Fatalf("cancel from %s: %v", from, err)
			}
			if o.Status != domain.OrderStatusCancelled {
				t.Errorf("status = %s, want CANCELLED", o.Status)
			}
		})
	}
}

func TestOrder_CannotTransitionFromTerminal(t *testing.T) {
	for _, from := range []domain.OrderStatus{
		domain.OrderStatusShipped,
		domain.OrderStatusCancelled,
		domain.OrderStatusFailed,
	} {
		t.Run(string(from), func(t *testing.T) {
			o, _ := domain.NewOrder("ord-1", "c1", "PLN", validItems(), testAddr, testNow)
			o.Status = from
			err := o.Transition(domain.OrderStatusValidated, "", testNow)
			if !errors.Is(err, domain.ErrTerminalStatus) {
				t.Fatalf("err = %v, want ErrTerminalStatus", err)
			}
		})
	}
}

func TestOrder_InvalidStatus(t *testing.T) {
	o, _ := domain.NewOrder("ord-1", "c1", "PLN", validItems(), testAddr, testNow)
	err := o.Transition("WHATEVER", "", testNow)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("err = %v, want ErrInvalidStatus", err)
	}
}
