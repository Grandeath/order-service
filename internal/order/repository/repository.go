package repository

import (
	"context"
	"sync"

	"github.com/Grandeath/order-service/internal/order/domain"
)

// Repository is the persistence boundary. Swap InMemory for a pgx-backed impl
// without touching the service layer.
type Repository interface {
	Save(ctx context.Context, o *domain.Order) error
	Get(ctx context.Context, id string) (*domain.Order, error)
	List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error)
	Delete(ctx context.Context, id string) error

	// SaveIdempotencyKey atomically claims key → orderID. If the key is already
	// bound it returns the existing orderID along with ErrIdempotencyExists.
	SaveIdempotencyKey(ctx context.Context, key, orderID string) (existing string, err error)
	GetIdempotencyKey(ctx context.Context, key string) (orderID string, ok bool)
}

type InMemory struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
	keys   map[string]string
}

func NewInMemory() *InMemory {
	return &InMemory{
		orders: make(map[string]*domain.Order),
		keys:   make(map[string]string),
	}
}

func (r *InMemory) Save(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = cloneOrder(o)
	return nil
}

func (r *InMemory) Get(_ context.Context, id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneOrder(o), nil
}

func (r *InMemory) List(_ context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	matched := make([]*domain.Order, 0, len(r.orders))
	for _, o := range r.orders {
		if customerID != "" && o.CustomerID != customerID {
			continue
		}
		matched = append(matched, o)
	}

	// Deterministic order: by CreatedAt then ID. Without sorting, map iteration
	// would make the API return arbitrary slices each call.
	sortByCreatedAt(matched)

	if offset >= len(matched) {
		return []*domain.Order{}, nil
	}
	end := min(offset+limit, len(matched))

	page := make([]*domain.Order, 0, end-offset)
	for _, o := range matched[offset:end] {
		page = append(page, cloneOrder(o))
	}
	return page, nil
}

func (r *InMemory) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.orders, id)
	return nil
}

func (r *InMemory) SaveIdempotencyKey(_ context.Context, key, orderID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.keys[key]; ok {
		return existing, domain.ErrIdempotencyExists
	}
	r.keys[key] = orderID
	return "", nil
}

func (r *InMemory) GetIdempotencyKey(_ context.Context, key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.keys[key]
	return id, ok
}

// cloneOrder returns a deep copy so callers cannot mutate the stored state.
func cloneOrder(o *domain.Order) *domain.Order {
	if o == nil {
		return nil
	}
	cp := *o
	cp.Items = append([]domain.OrderItem(nil), o.Items...)
	cp.History = append([]domain.StatusChange(nil), o.History...)
	return &cp
}
