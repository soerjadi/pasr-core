package ordering_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func TestGetOrder_OtherUser404(t *testing.T) {
	f := setupOrders(t)
	buyer, other := seed.User(t, f.pool, "", ""), seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Images: 1})
	o, err := f.svc.CreateOrder(context.Background(), buyer, "k", []domain.CartLineInput{ln(p, 1)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.GetOrder(context.Background(), buyer, o.ID)
	if err != nil || got.ID != o.ID || got.SellerOrders[0].Items[0].Image.Small == "" {
		t.Fatalf("owner: %v %+v", err, got)
	}
	for _, id := range []int64{o.ID, 987654} {
		if _, err := f.svc.GetOrder(context.Background(), other, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("order %d for another user: %v", id, err)
		}
	}
}

func TestCreateOrder_SnapshotSurvivesPriceChangeAndProductDelete(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{Name: "Bu Siti", Address: "Jl. Lama 1"})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Name: "Tempe", Price: 15000, Images: 1})
	ctx := context.Background()
	o, err := f.svc.CreateOrder(ctx, buyer, "k", []domain.CartLineInput{ln(p, 2)})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE products SET price = 99000, name = 'Tempe Baru', deleted_at = now() WHERE id = $1`,
		`DELETE FROM product_images WHERE product_id = $1`,
	} {
		if _, err := f.pool.Exec(ctx, q, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.pool.Exec(ctx, `UPDATE stores SET name = 'Toko Baru', address = 'Jl. Baru 2', whatsapp = '+62899' WHERE id = $1`, store); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.GetOrder(ctx, buyer, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	so := got.SellerOrders[0]
	it := so.Items[0]
	if so.SellerName != "Bu Siti" || so.PickupAddress != "Jl. Lama 1" || so.WhatsApp != o.SellerOrders[0].WhatsApp ||
		it.Name != "Tempe" || it.UnitPrice != 15000 || it.LineTotal != 30000 || got.Total != 30000 || it.Image != o.SellerOrders[0].Items[0].Image ||
		it.Image.Small == "" {
		t.Fatalf("snapshot changed: %+v", got)
	}
}

// placeWithStatus creates an order through the service, then forces its stored status.
func (f *ofx) placeWithStatus(t *testing.T, buyer, product int64, status domain.OrderStatus) int64 {
	t.Helper()
	o, err := f.svc.CreateOrder(context.Background(), buyer, fmt.Sprintf("k-%d-%s-%d", buyer, status, seedN()), []domain.CartLineInput{ln(product, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE orders SET status = $2 WHERE id = $1`, o.ID, status); err != nil {
		t.Fatal(err)
	}
	return o.ID
}

var keySeq int

func seedN() int { keySeq++; return keySeq }

func ids(p domain.Page[domain.Order]) []int64 {
	out := make([]int64, len(p.Items))
	for i, o := range p.Items {
		out[i] = o.ID
	}
	return out
}

func TestListOrders_TabsMapToStatuses(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 100})
	byStatus := map[domain.OrderStatus]int64{}
	for _, st := range []domain.OrderStatus{domain.OrderCreated, domain.OrderSellerProcessing, domain.OrderReadyForPickup,
		domain.OrderCompleted, domain.OrderCancelled} {
		byStatus[st] = f.placeWithStatus(t, buyer, p, st)
	}
	want := map[string][]int64{
		"ALL":        {byStatus[domain.OrderCancelled], byStatus[domain.OrderCompleted], byStatus[domain.OrderReadyForPickup], byStatus[domain.OrderSellerProcessing], byStatus[domain.OrderCreated]},
		"":           {byStatus[domain.OrderCancelled], byStatus[domain.OrderCompleted], byStatus[domain.OrderReadyForPickup], byStatus[domain.OrderSellerProcessing], byStatus[domain.OrderCreated]},
		"PROCESSING": {byStatus[domain.OrderSellerProcessing], byStatus[domain.OrderCreated]},
		"READY":      {byStatus[domain.OrderReadyForPickup]},
		"DONE":       {byStatus[domain.OrderCancelled], byStatus[domain.OrderCompleted]},
	}
	for tab, w := range want {
		page, err := f.svc.ListOrders(context.Background(), buyer, tab, "", 0)
		if err != nil {
			t.Fatalf("%s: %v", tab, err)
		}
		if fmt.Sprint(ids(page)) != fmt.Sprint(w) || page.NextCursor != nil {
			t.Errorf("tab %q: got %v, want %v", tab, ids(page), w)
		}
	}
	if _, err := f.svc.ListOrders(context.Background(), buyer, "NEW", "", 0); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("unknown tab: %v", err)
	}
	if _, err := f.svc.ListOrders(context.Background(), buyer, "ALL", "!!", 0); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("bad cursor: %v", err)
	}
}

func TestListOrders_NewestFirstCursorStable(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 100, Images: 1})
	var all []int64
	for range 5 {
		all = append([]int64{f.placeWithStatus(t, buyer, p, domain.OrderCreated)}, all...)
	}
	ctx := context.Background()
	first, err := f.svc.ListOrders(ctx, buyer, "ALL", "", 2)
	if err != nil || fmt.Sprint(ids(first)) != fmt.Sprint(all[:2]) || first.NextCursor == nil {
		t.Fatalf("first page: %v %v", err, ids(first))
	}
	if first.Items[0].SellerOrders[0].Items[0].Image.Small == "" || len(first.Items[0].StatusHistory) != 1 {
		t.Fatalf("details: %+v", first.Items[0])
	}
	// A new order between pages does not shift the next page.
	f.placeWithStatus(t, buyer, p, domain.OrderCreated)
	second, err := f.svc.ListOrders(ctx, buyer, "ALL", *first.NextCursor, 2)
	if err != nil || fmt.Sprint(ids(second)) != fmt.Sprint(all[2:4]) || second.NextCursor == nil {
		t.Fatalf("second page: %v %v", err, ids(second))
	}
	third, err := f.svc.ListOrders(ctx, buyer, "ALL", *second.NextCursor, 2)
	if err != nil || fmt.Sprint(ids(third)) != fmt.Sprint(all[4:]) || third.NextCursor != nil {
		t.Fatalf("last page: %v %v", err, ids(third))
	}
}

func TestListOrders_BuyerOnlySeesOwn(t *testing.T) {
	f := setupOrders(t)
	a, b := seed.User(t, f.pool, "", ""), seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 100})
	mine := f.placeWithStatus(t, a, p, domain.OrderCreated)
	f.placeWithStatus(t, b, p, domain.OrderCreated)
	page, err := f.svc.ListOrders(context.Background(), a, "ALL", "", 20)
	if err != nil || fmt.Sprint(ids(page)) != fmt.Sprint([]int64{mine}) {
		t.Fatalf("got %v %v", err, ids(page))
	}
	empty, err := f.svc.ListOrders(context.Background(), seed.User(t, f.pool, "", ""), "ALL", "", 20)
	if err != nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("no orders: %v %+v", err, empty)
	}
}
