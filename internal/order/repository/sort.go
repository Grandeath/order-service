package repository

import (
	"sort"

	"github.com/Grandeath/order-service/internal/order/domain"
)

func sortByCreatedAt(orders []*domain.Order) {
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreatedAt.Equal(orders[j].CreatedAt) {
			return orders[i].ID < orders[j].ID
		}
		return orders[i].CreatedAt.Before(orders[j].CreatedAt)
	})
}
