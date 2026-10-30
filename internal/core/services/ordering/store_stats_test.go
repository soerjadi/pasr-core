package ordering_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

// finish orders lines from one store at time at, then the seller accepts, readies and completes
// them at that same time. It returns the seller-order id.
func (f *ofx) finish(t *testing.T, s shop, at time.Time, lines ...domain.CartLineInput) int64 {
	t.Helper()
	ctx := context.Background()
	f.clock.Set(at)
	buyer := seed.User(t, f.pool, "", "Budi")
	o, err := f.svc.CreateOrder(ctx, buyer, fmt.Sprintf("fin-%d", seedN()), lines)
	if err != nil {
		t.Fatal(err)
	}
	id := soOf(t, o, s.store)
	a := sellerActor(s.owner)
	for _, step := range []func() (domain.StoreOrder, error){
		func() (domain.StoreOrder, error) { return f.svc.Accept(ctx, a, id) },
		func() (domain.StoreOrder, error) { return f.svc.Ready(ctx, a, id, "") },
		func() (domain.StoreOrder, error) { return f.svc.Complete(ctx, a, id) },
	} {
		if _, err := step(); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func (f *ofx) stats(t *testing.T, owner int64, rng string) domain.StoreStats {
	t.Helper()
	st, err := f.svc.StoreStats(context.Background(), owner, rng)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestStats_WeekDailyZeroFilledSevenEntries(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	f.clock.Set(t0)
	st := f.stats(t, s.owner, "WEEK")
	if st.Range != "WEEK" || len(st.Daily) != 7 || st.SalesValue != 0 || st.CompletedOrders != 0 ||
		st.Daily[0].Date != "2026-10-09" || st.Daily[6].Date != "2026-10-15" || st.TopProducts == nil || len(st.TopProducts) != 0 {
		t.Fatalf("empty week = %+v", st)
	}
	f.finish(t, s, t0.AddDate(0, 0, -6), ln(s.product, 1)) // oldest day in range
	f.finish(t, s, t0.AddDate(0, 0, -7), ln(s.product, 1)) // one day too old
	f.finish(t, s, t0.AddDate(0, 0, -2), ln(s.product, 2))
	f.finish(t, s, t0, ln(s.product, 3))
	f.clock.Set(t0)
	st = f.stats(t, s.owner, "WEEK")
	var vals []int64
	for _, d := range st.Daily {
		vals = append(vals, d.Value)
	}
	if fmt.Sprint(vals) != "[5000 0 0 0 10000 0 15000]" || st.SalesValue != 30000 || st.CompletedOrders != 3 {
		t.Fatalf("week = %+v", st)
	}
}

func TestStats_TodayRangeLimitsTotalsButKeepsSevenDays(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	f.finish(t, s, t0.AddDate(0, 0, -1), ln(s.product, 2))
	f.finish(t, s, t0, ln(s.product, 1))
	f.clock.Set(t0)
	st := f.stats(t, s.owner, "TODAY")
	if st.Range != "TODAY" || len(st.Daily) != 7 || st.Daily[5].Value != 10000 || st.Daily[6].Value != 5000 ||
		st.SalesValue != 5000 || st.CompletedOrders != 1 || len(st.TopProducts) != 1 || st.TopProducts[0].UnitsSold != 1 {
		t.Fatalf("today = %+v", st)
	}
}

func TestStats_WIBBoundary(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	before := time.Date(2026, 10, 15, 16, 59, 0, 0, time.UTC) // 23:59 WIB on the 15th
	after := time.Date(2026, 10, 15, 17, 30, 0, 0, time.UTC)  // 00:30 WIB on the 16th
	f.finish(t, s, before, ln(s.product, 1))
	f.finish(t, s, after, ln(s.product, 2))
	st := f.stats(t, s.owner, "WEEK") // the clock is still at `after`
	if st.Daily[5].Date != "2026-10-15" || st.Daily[5].Value != 5000 || st.Daily[6].Date != "2026-10-16" || st.Daily[6].Value != 10000 {
		t.Fatalf("daily = %+v", st.Daily)
	}
	if today := f.stats(t, s.owner, "TODAY"); today.SalesValue != 10000 || today.CompletedOrders != 1 {
		t.Fatalf("today = %+v", today)
	}
}

func TestStats_TopProductsRankedByUnits(t *testing.T) {
	f := setupOrders(t)
	store, owner := seed.Store(t, f.pool, seed.StoreOpts{})
	s := shop{store: store, owner: owner}
	var p [4]int64
	for i := range p {
		p[i] = seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Name: fmt.Sprintf("P%d", i), Price: 1000, Stock: 50})
	}
	f.finish(t, s, t0, ln(p[0], 2), ln(p[1], 7), ln(p[2], 5), ln(p[3], 1))
	f.finish(t, s, t0, ln(p[0], 4))
	f.clock.Set(t0)
	top := f.stats(t, owner, "WEEK").TopProducts
	if len(top) != 3 || top[0] != (domain.TopProduct{ProductID: p[1], Name: "P1", UnitsSold: 7}) ||
		top[1] != (domain.TopProduct{ProductID: p[0], Name: "P0", UnitsSold: 6}) || top[2].ProductID != p[2] {
		t.Fatalf("top = %+v", top)
	}
}

func TestStats_InvalidRange400(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	for _, r := range []string{"", "MONTH", "week"} {
		if _, err := f.svc.StoreStats(context.Background(), s.owner, r); codeOf(err) != "VALIDATION_FAILED" {
			t.Fatalf("range %q: %v", r, err)
		}
	}
}

func TestStats_OnlyOwnStoreData(t *testing.T) {
	f := setupOrders(t)
	mine, other := newShop(t, f, "Mine"), newShop(t, f, "Other")
	f.finish(t, other, t0, ln(other.product, 3))
	f.clock.Set(t0)
	st := f.stats(t, mine.owner, "WEEK")
	if st.SalesValue != 0 || st.CompletedOrders != 0 || len(st.TopProducts) != 0 {
		t.Fatalf("stats leaked another store: %+v", st)
	}
}

func TestStats_NoStore409NoStore(t *testing.T) {
	f := setupOrders(t)
	user := seed.User(t, f.pool, "", "Nobody")
	if _, err := f.svc.StoreStats(context.Background(), user, "WEEK"); codeOf(err) != "NO_STORE" {
		t.Fatalf("stats: %v", err)
	}
	if _, err := f.svc.StoreSummary(context.Background(), user); codeOf(err) != "NO_STORE" {
		t.Fatalf("summary: %v", err)
	}
}
