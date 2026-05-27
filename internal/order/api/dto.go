package api

import (
	"time"

	"github.com/Grandeath/order-service/internal/order/domain"
	"github.com/shopspring/decimal"
)

type CreateOrderRequest struct {
	Currency        string            `json:"currency"`
	Items           []CreateOrderItem `json:"items"`
	DeliveryAddress domain.Address    `json:"deliveryAddress"`
}

type CreateOrderItem struct {
	ProductID   string          `json:"productId"`
	ProductName string          `json:"productName"`
	Quantity    int             `json:"quantity"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
}

type ChangeStatusRequest struct {
	Status domain.OrderStatus `json:"status"`
	Reason string             `json:"reason"`
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type OrderResponse struct {
	ID              string                `json:"id"`
	CustomerID      string                `json:"customerId"`
	Status          domain.OrderStatus    `json:"status"`
	Items           []domain.OrderItem    `json:"items"`
	TotalAmount     decimal.Decimal       `json:"totalAmount"`
	Currency        string                `json:"currency"`
	DeliveryAddress domain.Address        `json:"deliveryAddress"`
	History         []domain.StatusChange `json:"history"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

type ListOrdersResponse struct {
	Orders []OrderResponse `json:"orders"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func toResponse(o *domain.Order) OrderResponse {
	return OrderResponse{
		ID:              o.ID,
		CustomerID:      o.CustomerID,
		Status:          o.Status,
		Items:           o.Items,
		TotalAmount:     o.TotalAmount,
		Currency:        o.Currency,
		DeliveryAddress: o.DeliveryAddress,
		History:         o.History,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
	}
}
