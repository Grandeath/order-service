package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/Grandeath/order-service/internal/order/domain"
)

// Topics mirrors the Kafka topics the service would publish/subscribe to.
// Kept as constants so callers don't pass magic strings.
const (
	TopicOrderCreated   = "order.created"
	TopicOrderCancelled = "order.cancelled"
	TopicOrderStatus    = "order.status_changed"
)

type Event struct {
	Topic     string         `json:"topic"`
	Key       string         `json:"key"`
	OccurredAt time.Time     `json:"occurredAt"`
	Payload   map[string]any `json:"payload"`
}

// Publisher abstracts the messaging transport. Real impl would be Kafka.
type Publisher interface {
	Publish(ctx context.Context, evt Event) error
}

// NoopPublisher logs the event and drops it. Useful for the in-memory build.
type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, evt Event) error {
	slog.Info("event published", "topic", evt.Topic, "key", evt.Key, "payload", evt.Payload)
	return nil
}

// OrderCreated builds the event emitted right after the order is persisted.
func OrderCreated(o *domain.Order, now time.Time) Event {
	return Event{
		Topic:      TopicOrderCreated,
		Key:        o.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"orderId":     o.ID,
			"customerId":  o.CustomerID,
			"status":      o.Status,
			"totalAmount": o.TotalAmount.String(),
			"currency":    o.Currency,
			"itemCount":   len(o.Items),
		},
	}
}

func OrderCancelled(o *domain.Order, reason string, now time.Time) Event {
	return Event{
		Topic:      TopicOrderCancelled,
		Key:        o.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"orderId":    o.ID,
			"customerId": o.CustomerID,
			"reason":     reason,
		},
	}
}

func StatusChanged(o *domain.Order, from, to domain.OrderStatus, now time.Time) Event {
	return Event{
		Topic:      TopicOrderStatus,
		Key:        o.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"orderId": o.ID,
			"from":    from,
			"to":      to,
		},
	}
}

