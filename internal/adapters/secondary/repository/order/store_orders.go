package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) GetStoreOrder(ctx context.Context, storeID, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "repository.order.GetStoreOrder")
	defer span.End()

	o, err := scanStoreOrder(r.db.QueryRow(ctx, getStoreOrderQuery, id, storeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoreOrder{}, notFound()
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrder{}, fmt.Errorf("order.GetStoreOrder scan: %w", err)
	}
	orders := []domain.StoreOrder{o}
	if err := r.loadStoreDetails(ctx, orders); err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrder{}, err
	}
	return orders[0], nil
}

func (r *orderRepository) ListStoreOrders(ctx context.Context, storeID int64, statuses []domain.OrderStatus, afterID int64, limit int) ([]domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "repository.order.ListStoreOrders")
	defer span.End()

	filter := make([]string, len(statuses))
	for i, s := range statuses {
		filter[i] = string(s)
	}
	rows, err := r.db.Query(ctx, listStoreOrdersQuery, storeID, filter, afterID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.ListStoreOrders query: %w", err)
	}
	defer rows.Close()
	orders := []domain.StoreOrder{}
	for rows.Next() {
		o, err := scanStoreOrder(rows)
		if err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("order.ListStoreOrders scan: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.ListStoreOrders rows: %w", err)
	}
	rows.Close()
	if err := r.loadStoreDetails(ctx, orders); err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}
	return orders, nil
}

func (r *orderRepository) ListAllStoreOrders(ctx context.Context, status domain.OrderStatus, afterID int64, limit int) ([]domain.AdminOrder, error) {
	ctx, span := tracer.Start(ctx, "repository.order.ListAllStoreOrders")
	defer span.End()

	rows, err := r.db.Query(ctx, listAllStoreOrdersQuery, string(status), afterID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.ListAllStoreOrders query: %w", err)
	}
	defer rows.Close()
	out := []domain.AdminOrder{}
	for rows.Next() {
		var a domain.AdminOrder
		var acceptBy *time.Time
		o := &a.StoreOrder
		if err := rows.Scan(&o.ID, &o.Number, &o.Status, &o.CreatedAt, &acceptBy, &o.BuyerName, &o.Note, &o.Subtotal,
			&a.StoreID, &a.StoreName); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("order.ListAllStoreOrders scan: %w", err)
		}
		o.CreatedAt = o.CreatedAt.UTC()
		o.AcceptBy = utc(acceptBy)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.ListAllStoreOrders rows: %w", err)
	}
	rows.Close()

	orders := make([]domain.StoreOrder, len(out))
	for i := range out {
		orders[i] = out[i].StoreOrder
	}
	if err := r.loadStoreDetails(ctx, orders); err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}
	for i := range out {
		out[i].StoreOrder = orders[i]
	}
	return out, nil
}

func (r *orderRepository) CountStoreOrders(ctx context.Context, storeID int64) (domain.StoreOrderCounts, error) {
	ctx, span := tracer.Start(ctx, "repository.order.CountStoreOrders")
	defer span.End()

	var c domain.StoreOrderCounts
	if err := r.db.QueryRow(ctx, countStoreOrdersQuery, storeID).Scan(&c.New, &c.Processing, &c.Done); err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderCounts{}, fmt.Errorf("order.CountStoreOrders scan: %w", err)
	}
	return c, nil
}

func scanStoreOrder(row pgx.Row) (domain.StoreOrder, error) {
	var o domain.StoreOrder
	var acceptBy *time.Time
	if err := row.Scan(&o.ID, &o.Number, &o.Status, &o.CreatedAt, &acceptBy, &o.BuyerName, &o.Note, &o.Subtotal); err != nil {
		return domain.StoreOrder{}, err
	}
	o.CreatedAt = o.CreatedAt.UTC()
	o.AcceptBy = utc(acceptBy)
	return o, nil
}

// loadStoreDetails fills the items and the seller-order history of every order in two queries.
func (r *orderRepository) loadStoreDetails(ctx context.Context, orders []domain.StoreOrder) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, len(orders))
	at := make(map[int64]int, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
		at[orders[i].ID] = i
		orders[i].Items = []domain.OrderItem{}
		orders[i].StatusHistory = []domain.StatusChange{}
	}

	rows, err := r.db.Query(ctx, storeOrderItemsQuery, ids)
	if err != nil {
		return fmt.Errorf("order.loadStoreDetails items: %w", err)
	}
	for rows.Next() {
		var it domain.OrderItem
		var soID int64
		if err := rows.Scan(&it.ID, &soID, &it.ProductID, &it.Name, &it.ImageKeys, &it.Quantity, &it.UnitPrice,
			&it.LineTotal, &it.Reviewed); err != nil {
			rows.Close()
			return fmt.Errorf("order.loadStoreDetails scan item: %w", err)
		}
		o := &orders[at[soID]]
		o.Items = append(o.Items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order.loadStoreDetails items rows: %w", err)
	}

	rows, err = r.db.Query(ctx, storeOrderHistoryQuery, ids)
	if err != nil {
		return fmt.Errorf("order.loadStoreDetails history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var soID int64
		var c domain.StatusChange
		var when time.Time
		if err := rows.Scan(&soID, &c.Status, &when); err != nil {
			return fmt.Errorf("order.loadStoreDetails scan history: %w", err)
		}
		c.At = when.UTC()
		o := &orders[at[soID]]
		o.StatusHistory = append(o.StatusHistory, c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order.loadStoreDetails history rows: %w", err)
	}
	return nil
}
