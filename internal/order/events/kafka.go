package events

import (
	"context"
	"fmt"

	"github.com/Grandeath/order-service/internal/producer"
)

type KafkaPublisher struct {
	notifier *producer.EventNotifier
}

func NewKafkaPublisher(n *producer.EventNotifier) *KafkaPublisher {
	return &KafkaPublisher{notifier: n}
}

func (k *KafkaPublisher) Publish(_ context.Context, evt Event) error {
	if k == nil || k.notifier == nil {
		return nil
	}
	if err := k.notifier.Notify(evt); err != nil {
		return fmt.Errorf("kafka notify: %w", err)
	}
	return nil
}
