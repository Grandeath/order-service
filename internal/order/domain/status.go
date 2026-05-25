package domain

import "fmt"

type OrderStatus string

const (
	OrderStatusNew            OrderStatus = "NEW"
	OrderStatusValidated      OrderStatus = "VALIDATED"
	OrderStatusPaymentPending OrderStatus = "PAYMENT_PENDING"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusProcessing     OrderStatus = "PROCESSING"
	OrderStatusShipped        OrderStatus = "SHIPPED"
	OrderStatusCancelled      OrderStatus = "CANCELLED"
	OrderStatusFailed         OrderStatus = "FAILED"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusNew, OrderStatusValidated, OrderStatusPaymentPending,
		OrderStatusPaid, OrderStatusProcessing, OrderStatusShipped,
		OrderStatusCancelled, OrderStatusFailed:
		return true
	}
	return false
}

func (s OrderStatus) IsTerminal() bool {
	return s == OrderStatusShipped || s == OrderStatusCancelled || s == OrderStatusFailed
}

// allowedTransitions defines the order lifecycle.
//
//	NEW → VALIDATED → PAYMENT_PENDING → PAID → PROCESSING → SHIPPED
//	plus CANCELLED / FAILED reachable from any non-terminal state.
var allowedTransitions = map[OrderStatus]map[OrderStatus]struct{}{
	OrderStatusNew:            {OrderStatusValidated: {}, OrderStatusCancelled: {}, OrderStatusFailed: {}},
	OrderStatusValidated:      {OrderStatusPaymentPending: {}, OrderStatusCancelled: {}, OrderStatusFailed: {}},
	OrderStatusPaymentPending: {OrderStatusPaid: {}, OrderStatusCancelled: {}, OrderStatusFailed: {}},
	OrderStatusPaid:           {OrderStatusProcessing: {}, OrderStatusCancelled: {}, OrderStatusFailed: {}},
	OrderStatusProcessing:     {OrderStatusShipped: {}, OrderStatusCancelled: {}, OrderStatusFailed: {}},
}

func CanTransition(from, to OrderStatus) bool {
	if from == to {
		return false
	}
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func validateTransition(from, to OrderStatus) error {
	if !to.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, to)
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}
