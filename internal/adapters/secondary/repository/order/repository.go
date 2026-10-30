package order

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type orderRepository struct {
	db repository.DB
}

func NewOrderRepository(db repository.DB) repository.OrderRepository {
	return &orderRepository{db: db}
}
