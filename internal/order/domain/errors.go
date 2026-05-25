package domain

import "errors"

var (
	ErrInvalidStatus     = errors.New("invalid order status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrTerminalStatus    = errors.New("order is in terminal status")
	ErrEmptyItems        = errors.New("order must contain at least one item")
	ErrInvalidQuantity   = errors.New("item quantity must be positive")
	ErrInvalidUnitPrice  = errors.New("item unit price must be non-negative")
	ErrInvalidCurrency   = errors.New("currency must be a 3-letter ISO 4217 code")
	ErrInvalidCustomer   = errors.New("customer id is required")
	ErrInvalidAddress    = errors.New("delivery address is incomplete")
	ErrNotFound          = errors.New("order not found")
	ErrIdempotencyExists = errors.New("idempotency key already used")
)
