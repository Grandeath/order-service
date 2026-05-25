package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/Grandeath/order-service/internal/order/domain"
	"github.com/Grandeath/order-service/internal/order/events"
	"github.com/Grandeath/order-service/internal/order/repository"
)

// Clock and IDGen are injected so tests can pin time and id values. In prod
// they default to time.Now and uuid.NewString.
type Clock func() time.Time
type IDGen func() string

type Service struct {
	repo      repository.Repository
	publisher events.Publisher
	now       Clock
	newID     IDGen
}

type Option func(*Service)

func WithClock(c Clock) Option        { return func(s *Service) { s.now = c } }
func WithIDGen(g IDGen) Option        { return func(s *Service) { s.newID = g } }
func WithPublisher(p events.Publisher) Option {
	return func(s *Service) { s.publisher = p }
}

func New(repo repository.Repository, opts ...Option) *Service {
	s := &Service{
		repo:      repo,
		publisher: events.NoopPublisher{},
		now:       time.Now,
		newID:     uuid.NewString,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type ItemInput struct {
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   decimal.Decimal
}

type CreateInput struct {
	IdempotencyKey  string
	CustomerID      string
	Currency        string
	Items           []ItemInput
	DeliveryAddress domain.Address
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Order, error) {
	if key := strings.TrimSpace(in.IdempotencyKey); key != "" {
		if existing, ok := s.repo.GetIdempotencyKey(ctx, key); ok {
			return s.repo.Get(ctx, existing)
		}
	}

	items := make([]domain.OrderItem, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, domain.OrderItem{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
		})
	}

	now := s.now()
	order, err := domain.NewOrder(s.newID(), in.CustomerID, in.Currency, items, in.DeliveryAddress, now)
	if err != nil {
		return nil, err
	}

	if key := strings.TrimSpace(in.IdempotencyKey); key != "" {
		if existing, err := s.repo.SaveIdempotencyKey(ctx, key, order.ID); err != nil {
			if errors.Is(err, domain.ErrIdempotencyExists) {
				return s.repo.Get(ctx, existing)
			}
			return nil, fmt.Errorf("idempotency: %w", err)
		}
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}

	_ = s.publisher.Publish(ctx, events.OrderCreated(order, now))

	return order, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Order, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
	return s.repo.List(ctx, customerID, limit, offset)
}

// Advance moves the order to the next status (caller-supplied). Use this for
// transitions driven by external events (payment.completed → PAID, etc.).
func (s *Service) Advance(ctx context.Context, id string, to domain.OrderStatus, reason string) (*domain.Order, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	now := s.now()
	prev := o.Status
	if err := o.Transition(to, reason, now); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, o); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	_ = s.publisher.Publish(ctx, events.StatusChanged(o, prev, to, now))
	return o, nil
}

func (s *Service) Cancel(ctx context.Context, id, reason string) (*domain.Order, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	now := s.now()
	if err := o.Cancel(reason, now); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, o); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	_ = s.publisher.Publish(ctx, events.OrderCancelled(o, reason, now))
	return o, nil
}

// Delete is admin-only — removes the order entirely. Useful for cleanup in
// tests and admin tooling; production would soft-delete.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
