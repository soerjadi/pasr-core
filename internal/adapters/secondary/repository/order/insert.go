package order

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) Insert(ctx context.Context, o domain.NewOrder) (int64, error) {
	ctx, span := tracer.Start(ctx, "repository.order.Insert")
	defer span.End()

	var orderID int64
	if err := r.db.QueryRow(ctx, insertOrderQuery, o.Number, o.BuyerID, o.Total, o.Status, o.CreatedAt).Scan(&orderID); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("order.Insert order: %w", err)
	}
	historyFor := []*int64{nil} // nil is the order-level entry
	for _, so := range o.SellerOrders {
		var soID int64
		if err := r.db.QueryRow(ctx, insertSellerOrderQuery, orderID, so.StoreID, so.Status, so.Subtotal, so.AcceptBy,
			so.StoreName, so.PickupAddress, so.OpeningHours, so.WhatsApp, o.CreatedAt).Scan(&soID); err != nil {
			tracer.RecordError(span, err)
			return 0, fmt.Errorf("order.Insert seller order: %w", err)
		}
		historyFor = append(historyFor, &soID)
		if err := r.insertItems(ctx, soID, so.Items); err != nil {
			tracer.RecordError(span, err)
			return 0, err
		}
	}
	if _, err := r.db.Exec(ctx, insertHistoryQuery, orderID, historyFor, o.Status, o.CreatedAt); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("order.Insert history: %w", err)
	}
	return orderID, nil
}

func (r *orderRepository) insertItems(ctx context.Context, sellerOrderID int64, items []domain.NewOrderItem) error {
	n := len(items)
	productIDs, names, images := make([]int64, n), make([]string, n), make([]*string, n)
	qtys, prices, totals := make([]int32, n), make([]int64, n), make([]int64, n)
	for i, it := range items {
		productIDs[i], names[i], qtys[i], prices[i], totals[i] = it.ProductID, it.Name, it.Quantity, it.UnitPrice, it.LineTotal
		if it.Image != nil {
			b, err := json.Marshal(it.Image)
			if err != nil {
				return fmt.Errorf("order.Insert marshal image: %w", err)
			}
			s := string(b)
			images[i] = &s
		}
	}
	if _, err := r.db.Exec(ctx, insertItemsQuery, sellerOrderID, productIDs, names, images, qtys, prices, totals); err != nil {
		return fmt.Errorf("order.Insert items: %w", err)
	}
	return nil
}
