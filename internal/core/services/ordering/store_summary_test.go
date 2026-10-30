package ordering_test

import (
	"context"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func (f *ofx) summary(t *testing.T, owner int64) domain.StoreSummaryResponse {
	t.Helper()
	sum, err := f.svc.StoreSummary(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestSummary_CountsTodayExcludingCancelled(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	f.finish(t, s, t0.AddDate(0, 0, -1), ln(s.product, 1)) // yesterday
	f.clock.Set(t0)
	buyer := seed.User(t, f.pool, "", "B")
	f.place(t, buyer, 1, s)
	rejected := f.place(t, buyer, 1, s)
	if _, err := f.svc.Reject(context.Background(), sellerActor(s.owner), soOf(t, rejected, s.store), "CANNOT_SERVE", ""); err != nil {
		t.Fatal(err)
	}
	if got := f.summary(t, s.owner).OrdersToday; got != 1 {
		t.Fatalf("ordersToday = %d", got)
	}
	// 00:30 WIB the next day: the two orders of "today" no longer count.
	f.clock.Set(time.Date(2026, 10, 15, 17, 30, 0, 0, time.UTC))
	if got := f.summary(t, s.owner).OrdersToday; got != 0 {
		t.Fatalf("ordersToday next WIB day = %d", got)
	}
}

func TestSummary_SalesOnlyCompleted(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	f.finish(t, s, t0, ln(s.product, 2))
	f.clock.Set(t0)
	buyer := seed.User(t, f.pool, "", "B")
	pending := f.place(t, buyer, 3, s)
	if _, err := f.svc.Accept(context.Background(), sellerActor(s.owner), soOf(t, pending, s.store)); err != nil {
		t.Fatal(err)
	}
	if got := f.summary(t, s.owner).SalesToday; got != 10000 {
		t.Fatalf("salesToday = %d", got)
	}
	// Created yesterday but completed today counts today.
	f.clock.Set(t0.AddDate(0, 0, -1))
	o := f.place(t, buyer, 1, s)
	id := soOf(t, o, s.store)
	a := sellerActor(s.owner)
	if _, err := f.svc.Accept(context.Background(), a, id); err != nil {
		t.Fatal(err)
	}
	f.clock.Set(t0)
	for _, err := range []error{
		func() error { _, e := f.svc.Ready(context.Background(), a, id, ""); return e }(),
		func() error { _, e := f.svc.Complete(context.Background(), a, id); return e }(),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := f.summary(t, s.owner).SalesToday; got != 15000 {
		t.Fatalf("salesToday after late completion = %d", got)
	}
}

func TestSummary_LatestThreeOrders(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	buyer := seed.User(t, f.pool, "", "B")
	var ids []int64
	for i := 0; i < 4; i++ {
		ids = append(ids, soOf(t, f.place(t, buyer, 1, s), s.store))
	}
	got := f.summary(t, s.owner).LatestOrders
	if len(got) != 3 || got[0].ID != ids[3] || got[1].ID != ids[2] || got[2].ID != ids[1] || got[0].Items == nil {
		t.Fatalf("latest = %+v", got)
	}
	empty := newShop(t, f, "Empty")
	if l := f.summary(t, empty.owner).LatestOrders; l == nil || len(l) != 0 {
		t.Fatalf("empty latest = %#v", l)
	}
}

func TestSummary_NewOrderCountAndAcceptBy(t *testing.T) {
	f := setupOrders(t)
	s := newShop(t, f, "A")
	if sum := f.summary(t, s.owner); sum.NewOrderCount != 0 || sum.AcceptBy != nil {
		t.Fatalf("none waiting: %+v", sum)
	}
	buyer := seed.User(t, f.pool, "", "B")
	f.clock.Set(t0)
	f.place(t, buyer, 1, s)
	f.clock.Set(t0.Add(3 * time.Minute))
	f.place(t, buyer, 1, s)
	sum := f.summary(t, s.owner)
	if sum.NewOrderCount != 2 || sum.AcceptBy == nil || !sum.AcceptBy.Equal(t0.Add(10*time.Minute)) || sum.AcceptBy.Location() != time.UTC {
		t.Fatalf("waiting: %+v", sum)
	}
}

func TestSummary_ClosesAtFromTodaysHours(t *testing.T) {
	f := setupOrders(t)
	hours := make(domain.Hours, 7)
	for i := range hours {
		hours[i] = domain.DayHours{Day: "x", Open: "07:00", Close: "15:00"}
	}
	hours[3].Close = "16:30"                                  // Thursday, 15 Oct 2026
	hours[4] = domain.DayHours{Closed: true}                  // Friday
	hours[6] = domain.DayHours{Open: "07:00", Close: "12:00"} // Sunday
	store, owner := seed.Store(t, f.pool, seed.StoreOpts{Hours: hours})
	_ = store
	noHours, noHoursOwner := seed.Store(t, f.pool, seed.StoreOpts{})
	_ = noHours

	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{t0, "16:30"},                                         // Thursday
		{t0.AddDate(0, 0, 1), ""},                             // Friday: closed
		{t0.AddDate(0, 0, 3), "12:00"},                        // Sunday (index 6)
		{t0.AddDate(0, 0, 4), "15:00"},                        // Monday (index 0)
		{time.Date(2026, 10, 15, 17, 30, 0, 0, time.UTC), ""}, // 00:30 WIB Friday, UTC still Thursday
	} {
		f.clock.Set(c.at)
		got := f.summary(t, owner).ClosesAt
		if (c.want == "") != (got == nil) || (got != nil && *got != c.want) {
			t.Fatalf("at %v: closesAt = %v want %q", c.at, got, c.want)
		}
	}
	f.clock.Set(t0)
	if got := f.summary(t, noHoursOwner).ClosesAt; got != nil {
		t.Fatalf("unset hours: closesAt = %q", *got)
	}
}
