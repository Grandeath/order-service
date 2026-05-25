package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

func (a Address) validate() error {
	if strings.TrimSpace(a.Street) == "" ||
		strings.TrimSpace(a.City) == "" ||
		strings.TrimSpace(a.PostalCode) == "" ||
		strings.TrimSpace(a.Country) == "" {
		return ErrInvalidAddress
	}
	return nil
}

type OrderItem struct {
	ProductID   string          `json:"productId"`
	ProductName string          `json:"productName"`
	Quantity    int             `json:"quantity"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
	TotalPrice  decimal.Decimal `json:"totalPrice"`
}

func (i *OrderItem) validate() error {
	if strings.TrimSpace(i.ProductID) == "" {
		return fmt.Errorf("item productId is required")
	}
	if i.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if i.UnitPrice.IsNegative() {
		return ErrInvalidUnitPrice
	}
	return nil
}

// recomputeTotal calculates TotalPrice from UnitPrice * Quantity.
func (i *OrderItem) recomputeTotal() {
	i.TotalPrice = i.UnitPrice.Mul(decimal.NewFromInt(int64(i.Quantity)))
}

type StatusChange struct {
	From      OrderStatus `json:"from"`
	To        OrderStatus `json:"to"`
	At        time.Time   `json:"at"`
	Reason    string      `json:"reason,omitempty"`
}

type Order struct {
	ID              string          `json:"id"`
	CustomerID      string          `json:"customerId"`
	Status          OrderStatus     `json:"status"`
	Items           []OrderItem     `json:"items"`
	TotalAmount     decimal.Decimal `json:"totalAmount"`
	Currency        string          `json:"currency"`
	DeliveryAddress Address         `json:"deliveryAddress"`
	History         []StatusChange  `json:"history"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// NewOrder builds and validates a fresh order in OrderStatusNew. Caller supplies
// the generated id and a clock so the constructor stays deterministic in tests.
func NewOrder(id, customerID, currency string, items []OrderItem, addr Address, now time.Time) (*Order, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("order id is required")
	}
	if strings.TrimSpace(customerID) == "" {
		return nil, ErrInvalidCustomer
	}
	if len(currency) != 3 {
		return nil, ErrInvalidCurrency
	}
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}
	if err := addr.validate(); err != nil {
		return nil, err
	}

	itemsCopy := make([]OrderItem, len(items))
	total := decimal.Zero
	for i := range items {
		itemsCopy[i] = items[i]
		if err := itemsCopy[i].validate(); err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		itemsCopy[i].recomputeTotal()
		total = total.Add(itemsCopy[i].TotalPrice)
	}

	o := &Order{
		ID:              id,
		CustomerID:      customerID,
		Status:          OrderStatusNew,
		Items:           itemsCopy,
		TotalAmount:     total,
		Currency:        strings.ToUpper(currency),
		DeliveryAddress: addr,
		History: []StatusChange{{
			From: "",
			To:   OrderStatusNew,
			At:   now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return o, nil
}

// Transition moves the order to the next status if the transition is allowed.
// Reason is optional, but recommended for CANCELLED / FAILED.
func (o *Order) Transition(to OrderStatus, reason string, now time.Time) error {
	if o.Status.IsTerminal() {
		return fmt.Errorf("%w: %s", ErrTerminalStatus, o.Status)
	}
	if err := validateTransition(o.Status, to); err != nil {
		return err
	}
	o.History = append(o.History, StatusChange{
		From:   o.Status,
		To:     to,
		At:     now,
		Reason: reason,
	})
	o.Status = to
	o.UpdatedAt = now
	return nil
}

// Cancel is a convenience for transitioning into CANCELLED.
func (o *Order) Cancel(reason string, now time.Time) error {
	return o.Transition(OrderStatusCancelled, reason, now)
}
