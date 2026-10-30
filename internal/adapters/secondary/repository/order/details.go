package order

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

func scanOrder(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	if err := row.Scan(&o.ID, &o.Number, &o.Status, &o.CreatedAt, &o.Total, &o.PaymentMethod); err != nil {
		return domain.Order{}, err
	}
	o.CreatedAt = o.CreatedAt.UTC()
	return o, nil
}

// loadDetails fills seller-orders, items and the order-level history of every order in three queries.
func (r *orderRepository) loadDetails(ctx context.Context, orders []domain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, len(orders))
	at := make(map[int64]int, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
		at[orders[i].ID] = i
		orders[i].SellerOrders = []domain.SellerOrder{}
		orders[i].StatusHistory = []domain.StatusChange{}
	}

	type soRef struct{ order, idx int }
	soAt := map[int64]soRef{}
	rows, err := r.db.Query(ctx, sellerOrdersQuery, ids)
	if err != nil {
		return fmt.Errorf("order.loadDetails seller orders: %w", err)
	}
	for rows.Next() {
		var so domain.SellerOrder
		var orderID int64
		if err := rows.Scan(&so.ID, &orderID, &so.SellerID, &so.SellerName, &so.WhatsApp, &so.PickupAddress,
			&so.OpeningHours, &so.Status, &so.Note, &so.Subtotal); err != nil {
			rows.Close()
			return fmt.Errorf("order.loadDetails scan seller order: %w", err)
		}
		so.Items = []domain.OrderItem{}
		oi := at[orderID]
		soAt[so.ID] = soRef{oi, len(orders[oi].SellerOrders)}
		orders[oi].SellerOrders = append(orders[oi].SellerOrders, so)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order.loadDetails seller orders rows: %w", err)
	}

	rows, err = r.db.Query(ctx, itemsQuery, ids)
	if err != nil {
		return fmt.Errorf("order.loadDetails items: %w", err)
	}
	for rows.Next() {
		var it domain.OrderItem
		var soID int64
		if err := rows.Scan(&it.ID, &soID, &it.ProductID, &it.Name, &it.ImageKeys, &it.Quantity, &it.UnitPrice,
			&it.LineTotal, &it.Reviewed); err != nil {
			rows.Close()
			return fmt.Errorf("order.loadDetails scan item: %w", err)
		}
		ref := soAt[soID]
		so := &orders[ref.order].SellerOrders[ref.idx]
		so.Items = append(so.Items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order.loadDetails items rows: %w", err)
	}

	rows, err = r.db.Query(ctx, historyQuery, ids)
	if err != nil {
		return fmt.Errorf("order.loadDetails history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var c domain.StatusChange
		var when time.Time
		if err := rows.Scan(&orderID, &c.Status, &when); err != nil {
			return fmt.Errorf("order.loadDetails scan history: %w", err)
		}
		c.At = when.UTC()
		oi := at[orderID]
		orders[oi].StatusHistory = append(orders[oi].StatusHistory, c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order.loadDetails history rows: %w", err)
	}
	return nil
}
