package ordering

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	maxKeyLen = 255
	// txTimeout bounds every statement of the order transaction, waits on the idempotency key and
	// product locks included, so a stuck holder fails requests instead of piling them up.
	txTimeout = 5 * time.Second
)

// CreateOrder takes its locks in a fixed order: the idempotency key row, then the products by
// ascending id, then the day's order counter. Seller-orders are never locked here.
func (s *service) CreateOrder(ctx context.Context, buyerID int64, key string, lines []domain.CartLineInput) (domain.Order, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.CreateOrder")
	defer span.End()

	if key == "" || len(key) > maxKeyLen {
		err := invalid("Idempotency-Key header is required (1 to 255 characters)")
		tracer.RecordError(span, err)
		return domain.Order{}, err
	}
	merged, err := mergeLines(lines)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Order{}, err
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ProductID < merged[j].ProductID })
	hash := requestHash(merged)

	var out domain.Order
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.orders.SetStatementTimeout(ctx, txTimeout); err != nil {
			return err
		}
		// A concurrent request holding the same key makes Claim wait for it to finish.
		existing, claimed, existingHash, err := s.keys.Claim(ctx, buyerID, key, hash)
		if err != nil {
			return err
		}
		if !claimed {
			if existingHash != hash {
				return domain.NewError(domain.ErrUnprocessable, "IDEMPOTENCY_KEY_REUSED",
					"Idempotency-Key was already used for a different cart")
			}
			if existing == 0 {
				return fmt.Errorf("ordering.CreateOrder: committed idempotency key without an order")
			}
			out, err = s.orders.Get(ctx, buyerID, existing)
			return err
		}
		out, err = s.place(ctx, buyerID, key, merged)
		return err
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Order{}, err
	}
	s.present(&out)
	return out, nil
}

// place runs inside the transaction after the key was claimed. merged is sorted by product id.
func (s *service) place(ctx context.Context, buyerID int64, key string, merged []domain.CartLineInput) (domain.Order, error) {
	ids := make([]int64, len(merged))
	for i, l := range merged {
		ids[i] = l.ProductID
	}
	products, err := s.carts.LoadProducts(ctx, ids, true) // FOR UPDATE, ascending id
	if err != nil {
		return domain.Order{}, err
	}
	if err := checkLines(merged, products); err != nil {
		return domain.Order{}, err
	}

	now := s.clock.Now()
	order := domain.NewOrder{BuyerID: buyerID, Status: domain.OrderCreated, CreatedAt: now}
	storeAt := map[int64]int{}
	owners := map[int64]int64{} // store id -> owner user id
	for _, l := range merged {
		p := products[l.ProductID]
		total, err := domain.LineTotal(p.Price, l.Quantity)
		if err != nil {
			return domain.Order{}, err
		}
		i, ok := storeAt[p.Store.ID]
		if !ok {
			i = len(order.SellerOrders)
			storeAt[p.Store.ID] = i
			owners[p.Store.ID] = p.Store.OwnerID
			st := p.Store
			order.SellerOrders = append(order.SellerOrders, domain.NewSellerOrder{
				StoreID: st.ID, StoreName: st.Name, WhatsApp: st.WhatsApp, PickupAddress: st.PickupAddress,
				OpeningHours: st.Hours.Display(), Status: domain.OrderCreated, AcceptBy: now.Add(s.cfg.AcceptWindow),
			})
		}
		so := &order.SellerOrders[i]
		if so.Subtotal > math.MaxInt64-total || order.Total > math.MaxInt64-total {
			return domain.Order{}, invalid("order total overflows")
		}
		so.Subtotal += total
		order.Total += total
		so.Items = append(so.Items, domain.NewOrderItem{
			ProductID: p.ID, Name: p.Name, Image: p.Image, Quantity: l.Quantity, UnitPrice: p.Price, LineTotal: total,
		})
	}

	for _, l := range merged {
		ok, err := s.orders.DecrementStock(ctx, l.ProductID, l.Quantity)
		if err != nil {
			return domain.Order{}, err
		}
		if !ok { // cannot happen while the rows are locked and checked; kept as a guard
			return domain.Order{}, conflict("INSUFFICIENT_STOCK", "insufficient stock for products: ", []int64{l.ProductID})
		}
	}

	if order.Number, err = s.orders.NextOrderNumber(ctx, now.In(s.cfg.Location)); err != nil {
		return domain.Order{}, err
	}
	id, err := s.orders.Insert(ctx, order)
	if err != nil {
		return domain.Order{}, err
	}
	if err := s.keys.Attach(ctx, buyerID, key, id); err != nil {
		return domain.Order{}, err
	}
	out, err := s.orders.Get(ctx, buyerID, id)
	if err != nil {
		return domain.Order{}, err
	}
	for _, so := range out.SellerOrders {
		err := s.notify.Enqueue(ctx, domain.Notification{
			UserID: owners[so.SellerID], Kind: domain.NotifNewOrder,
			Data: map[string]string{
				"orderNumber": out.Number, "orderId": strconv.FormatInt(out.ID, 10),
				"sellerOrderId": strconv.FormatInt(so.ID, 10), "store": so.SellerName,
			},
		})
		if err != nil {
			return domain.Order{}, err
		}
	}
	return out, nil
}

// checkLines rejects the cart when any line cannot be bought. Products that are not publicly
// visible (missing, deleted, hidden or of a store that is not VERIFIED) are PRODUCT_UNAVAILABLE
// with nothing more said; then a closed store is STORE_CLOSED; then short stock is INSUFFICIENT_STOCK.
func checkLines(merged []domain.CartLineInput, products map[int64]domain.CartProduct) error {
	var unavailable, closed, short []int64
	for _, l := range merged {
		p, ok := products[l.ProductID]
		switch {
		case !ok || !p.PubliclyVisible():
			unavailable = append(unavailable, l.ProductID)
		case !p.Orderable():
			closed = append(closed, l.ProductID)
		case p.Status != domain.ProductAvailable || p.Stock < l.Quantity:
			short = append(short, l.ProductID)
		}
	}
	switch {
	case len(unavailable) > 0:
		return conflict("PRODUCT_UNAVAILABLE", "products unavailable: ", unavailable)
	case len(closed) > 0:
		return conflict("STORE_CLOSED", "store is closed for products: ", closed)
	case len(short) > 0:
		return conflict("INSUFFICIENT_STOCK", "insufficient stock for products: ", short)
	}
	return nil
}

func conflict(code, prefix string, ids []int64) error {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return domain.NewError(domain.ErrConflict, code, prefix+strings.Join(parts, ", "))
}

// requestHash is the SHA-256 of the merged lines sorted by product id, so equal carts in any order
// or split into duplicate lines hash the same.
func requestHash(sorted []domain.CartLineInput) string {
	var b strings.Builder
	for _, l := range sorted {
		fmt.Fprintf(&b, "%d:%d;", l.ProductID, l.Quantity)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
