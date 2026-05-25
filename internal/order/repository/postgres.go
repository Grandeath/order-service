package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/Grandeath/order-service/internal/order/domain"
)

// orderColumns is the canonical column list for SELECT. Numeric total is cast
// to text so decimal.Decimal can be reconstructed without depending on a pgx
// numeric adapter.
const orderColumns = `id, customer_id, status, items, total_amount::text, currency, delivery_address, history, created_at, updated_at`

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (r *Postgres) Save(ctx context.Context, o *domain.Order) error {
	items, err := json.Marshal(o.Items)
	if err != nil {
		return fmt.Errorf("marshal items: %w", err)
	}
	addr, err := json.Marshal(o.DeliveryAddress)
	if err != nil {
		return fmt.Errorf("marshal address: %w", err)
	}
	hist, err := json.Marshal(o.History)
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO orders (id, customer_id, status, items, total_amount, currency, delivery_address, history, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::numeric, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			customer_id      = EXCLUDED.customer_id,
			status           = EXCLUDED.status,
			items            = EXCLUDED.items,
			total_amount     = EXCLUDED.total_amount,
			currency         = EXCLUDED.currency,
			delivery_address = EXCLUDED.delivery_address,
			history          = EXCLUDED.history,
			updated_at       = EXCLUDED.updated_at
	`,
		o.ID, o.CustomerID, string(o.Status), items,
		o.TotalAmount.String(), o.Currency, addr, hist,
		o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (r *Postgres) Get(ctx context.Context, id string) (*domain.Order, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+orderColumns+" FROM orders WHERE id = $1", id)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (r *Postgres) List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var (
		rows pgx.Rows
		err  error
	)
	if customerID == "" {
		rows, err = r.pool.Query(ctx, "SELECT "+orderColumns+" FROM orders ORDER BY created_at, id LIMIT $1 OFFSET $2", limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, "SELECT "+orderColumns+" FROM orders WHERE customer_id = $1 ORDER BY created_at, id LIMIT $2 OFFSET $3", customerID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return out, nil
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SaveIdempotencyKey atomically claims the key. The ON CONFLICT … DO UPDATE
// is a no-op that lets RETURNING return the existing order_id when the key is
// already taken; we compare it with the requested orderID to detect collisions.
func (r *Postgres) SaveIdempotencyKey(ctx context.Context, key, orderID string) (string, error) {
	var stored string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO idempotency_keys (key, order_id) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET key = idempotency_keys.key
		RETURNING order_id
	`, key, orderID).Scan(&stored)
	if err != nil {
		return "", fmt.Errorf("save idempotency key: %w", err)
	}
	if stored != orderID {
		return stored, domain.ErrIdempotencyExists
	}
	return "", nil
}

func (r *Postgres) GetIdempotencyKey(ctx context.Context, key string) (string, bool) {
	var orderID string
	err := r.pool.QueryRow(ctx, "SELECT order_id FROM idempotency_keys WHERE key = $1", key).Scan(&orderID)
	if err != nil {
		return "", false
	}
	return orderID, true
}

func scanOrder(row pgx.Row) (*domain.Order, error) {
	var (
		o          domain.Order
		status     string
		totalStr   string
		itemsRaw   []byte
		addrRaw    []byte
		historyRaw []byte
	)
	if err := row.Scan(
		&o.ID, &o.CustomerID, &status, &itemsRaw, &totalStr,
		&o.Currency, &addrRaw, &historyRaw, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return nil, err
	}
	o.Status = domain.OrderStatus(status)

	total, err := decimal.NewFromString(totalStr)
	if err != nil {
		return nil, fmt.Errorf("parse total amount: %w", err)
	}
	o.TotalAmount = total

	if err := json.Unmarshal(itemsRaw, &o.Items); err != nil {
		return nil, fmt.Errorf("unmarshal items: %w", err)
	}
	if err := json.Unmarshal(addrRaw, &o.DeliveryAddress); err != nil {
		return nil, fmt.Errorf("unmarshal address: %w", err)
	}
	if err := json.Unmarshal(historyRaw, &o.History); err != nil {
		return nil, fmt.Errorf("unmarshal history: %w", err)
	}
	return &o, nil
}
