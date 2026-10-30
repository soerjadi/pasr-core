package ordering_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	cartrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/cart"
	devicerepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/device"
	idemrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/idempotency"
	notificationrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/notification"
	orderrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/order"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
	"github.com/oemah/nusantara/core/internal/core/services/notify"
	"github.com/oemah/nusantara/core/internal/core/services/ordering"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

var wib = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return loc
}()

// t0 is 10:02 WIB on 15 October 2026.
var t0 = time.Date(2026, 10, 15, 3, 2, 0, 0, time.UTC)

type ofx struct {
	pool  *pgxpool.Pool
	svc   services.OrderingService
	clock *testapp.FakeClock
}

func setupOrders(t *testing.T) *ofx {
	t.Helper()
	pool := pgtest.Pool(t)
	exec := db.NewExecutor(pool)
	clock := testapp.NewFakeClock(t0)
	notifier := notify.NewService(notificationrepo.NewNotificationRepository(exec), devicerepo.NewDeviceRepository(exec),
		testapp.NewFakePusher(), exec)
	svc := ordering.NewService(cartrepo.NewCartRepository(exec), orderrepo.NewOrderRepository(exec),
		idemrepo.NewIdempotencyRepository(exec), exec, notifier, fakeStorage{}, clock,
		ordering.Config{AcceptWindow: 10 * time.Minute, Location: wib})
	return &ofx{pool: pool, svc: svc, clock: clock}
}

func ln(id int64, q int32) domain.CartLineInput {
	return domain.CartLineInput{ProductID: id, Quantity: q}
}

func (f *ofx) stock(t *testing.T, id int64) int32 {
	t.Helper()
	var n int32
	if err := f.pool.QueryRow(context.Background(), `SELECT stock FROM products WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *ofx) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func codeOf(err error) string {
	var c *domain.CodedError
	if errors.As(err, &c) {
		return c.Code
	}
	return ""
}

func TestCreateOrder_SingleSeller_SnapshotsAndTotals(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "Budi")
	hours := make(domain.Hours, 7)
	for i := range hours {
		hours[i] = domain.DayHours{Day: "x", Open: "07:00", Close: "17:00"}
	}
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{Name: "Bu Siti", Address: "Jl. Merdeka 11", WhatsApp: "+6281234567889", Hours: hours})
	tempe := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Name: "Keripik Tempe", Price: 15000, Stock: 10, Images: 2})
	madu := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Name: "Madu", Price: 45000, Stock: 3})

	o, err := f.svc.CreateOrder(context.Background(), buyer, "k1", []domain.CartLineInput{ln(tempe, 2), ln(madu, 1), ln(tempe, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if o.ID == 0 || o.Number != "NM-20261015-001" || o.Status != domain.OrderCreated || o.Total != 3*15000+45000 ||
		o.PaymentMethod != domain.PaymentCashAtPickup || !o.CreatedAt.Equal(t0) || o.CreatedAt.Location() != time.UTC {
		t.Fatalf("order = %+v", o)
	}
	if len(o.StatusHistory) != 1 || o.StatusHistory[0].Status != domain.OrderCreated || !o.StatusHistory[0].At.Equal(t0) {
		t.Fatalf("history = %+v", o.StatusHistory)
	}
	if len(o.SellerOrders) != 1 {
		t.Fatalf("seller orders = %+v", o.SellerOrders)
	}
	so := o.SellerOrders[0]
	if so.SellerID != store || so.SellerName != "Bu Siti" || so.PickupAddress != "Jl. Merdeka 11" || so.WhatsApp != "+6281234567889" ||
		so.OpeningHours != "Setiap hari 07.00–17.00" || so.Status != domain.OrderCreated || so.Subtotal != o.Total || len(so.Items) != 2 {
		t.Fatalf("seller order = %+v", so)
	}
	first := so.Items[0]
	if first.ProductID != tempe || first.Name != "Keripik Tempe" || first.Quantity != 3 || first.UnitPrice != 15000 ||
		first.LineTotal != 45000 || first.Reviewed || first.Image.Small == "" ||
		first.Image.Large != "https://img.test/seed/product-"+fmt.Sprint(tempe)+"-1-large.jpg" {
		t.Fatalf("first item = %+v", first)
	}
	if so.Items[1].ProductID != madu || so.Items[1].Image != (domain.ImageSet{}) {
		t.Fatalf("second item = %+v", so.Items[1])
	}
	if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE order_id = $1 AND seller_order_id IS NOT NULL`, o.ID); n != 1 {
		t.Fatalf("seller-order history rows = %d", n)
	}
}

func TestCreateOrder_TwoSellers_TwoSellerOrdersAndOneNumber(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "Budi")
	a, _ := seed.Store(t, f.pool, seed.StoreOpts{Name: "A"})
	b, _ := seed.Store(t, f.pool, seed.StoreOpts{Name: "B"})
	pb := seed.Product(t, f.pool, seed.ProductOpts{StoreID: b, Price: 2000})
	pa := seed.Product(t, f.pool, seed.ProductOpts{StoreID: a, Price: 1000})

	o, err := f.svc.CreateOrder(context.Background(), buyer, "k1", []domain.CartLineInput{ln(pb, 1), ln(pa, 2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.SellerOrders) != 2 || o.Total != 4000 || o.Number != "NM-20261015-001" {
		t.Fatalf("order = %+v", o)
	}
	subtotals := map[int64]int64{}
	for _, so := range o.SellerOrders {
		subtotals[so.SellerID] = so.Subtotal
	}
	if subtotals[a] != 2000 || subtotals[b] != 2000 {
		t.Fatalf("subtotals = %v", subtotals)
	}
	if n := f.count(t, `SELECT count(*) FROM orders`); n != 1 {
		t.Fatalf("orders = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE order_id = $1`, o.ID); n != 3 {
		t.Fatalf("history rows = %d, want order + 2 seller-orders", n)
	}
}

func TestCreateOrder_DecrementsStock(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 5})
	last := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 2})

	if _, err := f.svc.CreateOrder(context.Background(), buyer, "k1", []domain.CartLineInput{ln(p, 3), ln(last, 2)}); err != nil {
		t.Fatal(err)
	}
	if got := f.stock(t, p); got != 2 {
		t.Fatalf("stock = %d, want 2", got)
	}
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM products WHERE id = $1`, last).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if f.stock(t, last) != 0 || status != "OUT_OF_STOCK" {
		t.Fatalf("sold-out product: stock %d status %s", f.stock(t, last), status)
	}
}

func TestCreateOrder_LastUnitRace_ExactlyOneWins(t *testing.T) {
	f := setupOrders(t)
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 1})
	buyers := []int64{seed.User(t, f.pool, "", ""), seed.User(t, f.pool, "", "")}

	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = f.svc.CreateOrder(context.Background(), buyers[i], "k", []domain.CartLineInput{ln(p, 1)})
		}()
	}
	close(start)
	wg.Wait()
	ok, lost := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrConflict) && codeOf(err) == "INSUFFICIENT_STOCK":
			lost++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || lost != 1 || f.stock(t, p) != 0 || f.count(t, `SELECT count(*) FROM orders`) != 1 {
		t.Fatalf("ok=%d lost=%d stock=%d", ok, lost, f.stock(t, p))
	}
}

func TestCreateOrder_MultiLineRollsBackAllStockOnFailure(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p1 := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 5})
	p2 := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 1})
	p3 := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 5})

	_, err := f.svc.CreateOrder(context.Background(), buyer, "k1", []domain.CartLineInput{ln(p1, 2), ln(p2, 2), ln(p3, 1)})
	if codeOf(err) != "INSUFFICIENT_STOCK" || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("got %v", err)
	}
	if msg := err.Error(); msg != fmt.Sprintf("insufficient stock for products: %d", p2) {
		t.Fatalf("message = %q", msg)
	}
	if f.stock(t, p1) != 5 || f.stock(t, p2) != 1 || f.stock(t, p3) != 5 {
		t.Fatal("stock changed after a failed order")
	}
	for _, table := range []string{"orders", "seller_orders", "order_items", "order_status_history", "notifications", "idempotency_keys", "order_counters"} {
		if n := f.count(t, `SELECT count(*) FROM `+table); n != 0 {
			t.Fatalf("%s has %d rows after rollback", table, n)
		}
	}
	// The key was released with the rollback: the same key works once the cart is valid.
	if _, err := f.svc.CreateOrder(context.Background(), buyer, "k1", []domain.CartLineInput{ln(p1, 2), ln(p2, 1), ln(p3, 1)}); err != nil {
		t.Fatalf("retry with the same key: %v", err)
	}
}

func TestCreateOrder_SameKeySequential_ReturnsSameOrder(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 5, Images: 1})
	ctx := context.Background()

	first, err := f.svc.CreateOrder(ctx, buyer, "k1", []domain.CartLineInput{ln(p, 1), ln(p, 1)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	// Same cart after merging, in another shape: the stored order comes back unchanged.
	again, err := f.svc.CreateOrder(ctx, buyer, "k1", []domain.CartLineInput{ln(p, 2)})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(again)
	b, _ := json.Marshal(first)
	if string(a) != string(b) {
		t.Fatalf("replay differs:\n%s\n%s", a, b)
	}
	if f.stock(t, p) != 3 || f.count(t, `SELECT count(*) FROM orders`) != 1 || f.count(t, `SELECT count(*) FROM notifications`) != 1 {
		t.Fatal("replay must not create, decrement or notify again")
	}
	// A different buyer may use the same key string.
	other := seed.User(t, f.pool, "", "")
	if o, err := f.svc.CreateOrder(ctx, other, "k1", []domain.CartLineInput{ln(p, 2)}); err != nil || o.ID == first.ID {
		t.Fatalf("other buyer: %v %+v", err, o)
	}
}

func TestCreateOrder_SameKeyConcurrent_OneOrder(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 50})

	const n = 10
	ids := make([]int64, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			o, err := f.svc.CreateOrder(context.Background(), buyer, "same", []domain.CartLineInput{ln(p, 2)})
			ids[i], errs[i] = o.ID, err
		}()
	}
	close(start)
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if ids[i] == 0 || ids[i] != ids[0] {
			t.Fatalf("different orders: %v", ids)
		}
	}
	if got := f.count(t, `SELECT count(*) FROM orders`); got != 1 {
		t.Fatalf("orders = %d", got)
	}
	if got := f.stock(t, p); got != 48 {
		t.Fatalf("stock = %d, want 48 (one decrement)", got)
	}
	if got := f.count(t, `SELECT count(*) FROM notifications`); got != 1 {
		t.Fatalf("notifications = %d", got)
	}
}

func TestCreateOrder_SameKeyDifferentCart422(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 5})
	ctx := context.Background()
	if _, err := f.svc.CreateOrder(ctx, buyer, "k1", []domain.CartLineInput{ln(p, 1)}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.CreateOrder(ctx, buyer, "k1", []domain.CartLineInput{ln(p, 2)})
	if !errors.Is(err, domain.ErrUnprocessable) || codeOf(err) != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("got %v", err)
	}
	if f.stock(t, p) != 4 {
		t.Fatal("stock changed by a rejected replay")
	}
}

func TestCreateOrder_MissingKey400(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store})
	for _, key := range []string{"", string(make([]byte, 256))} {
		if _, err := f.svc.CreateOrder(context.Background(), buyer, key, []domain.CartLineInput{ln(p, 1)}); !errors.Is(err, domain.ErrValidationFailed) {
			t.Fatalf("key len %d: %v", len(key), err)
		}
	}
	if _, err := f.svc.CreateOrder(context.Background(), buyer, "k", nil); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("empty cart: %v", err)
	}
}

func TestCreateOrder_ClosedStore409StoreClosed(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	open, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	closed, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	if _, err := f.pool.Exec(context.Background(), `UPDATE stores SET is_open = false WHERE id = $1`, closed); err != nil {
		t.Fatal(err)
	}
	po := seed.Product(t, f.pool, seed.ProductOpts{StoreID: open})
	pc := seed.Product(t, f.pool, seed.ProductOpts{StoreID: closed})

	_, err := f.svc.CreateOrder(context.Background(), buyer, "k", []domain.CartLineInput{ln(po, 1), ln(pc, 1)})
	if !errors.Is(err, domain.ErrConflict) || codeOf(err) != "STORE_CLOSED" || err.Error() != fmt.Sprintf("store is closed for products: %d", pc) {
		t.Fatalf("got %v", err)
	}
	if f.stock(t, po) != 10 {
		t.Fatal("stock changed")
	}
}

func TestCreateOrder_PendingStoreAndHiddenProduct409ProductUnavailable(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	ok, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	pending, _ := seed.Store(t, f.pool, seed.StoreOpts{Status: "PENDING"})
	suspended, _ := seed.Store(t, f.pool, seed.StoreOpts{Status: "SUSPENDED"})
	closed, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	if _, err := f.pool.Exec(context.Background(), `UPDATE stores SET is_open = false WHERE id = $1`, closed); err != nil {
		t.Fatal(err)
	}
	good := seed.Product(t, f.pool, seed.ProductOpts{StoreID: ok})
	pp := seed.Product(t, f.pool, seed.ProductOpts{StoreID: pending})
	ps := seed.Product(t, f.pool, seed.ProductOpts{StoreID: suspended})
	hidden := seed.Product(t, f.pool, seed.ProductOpts{StoreID: ok, Status: "HIDDEN"})
	deleted := seed.Product(t, f.pool, seed.ProductOpts{StoreID: ok, Deleted: true})
	pc := seed.Product(t, f.pool, seed.ProductOpts{StoreID: closed})

	cases := map[string][]domain.CartLineInput{
		"pending store":   {ln(good, 1), ln(pp, 1)},
		"suspended store": {ln(ps, 1)},
		"hidden product":  {ln(hidden, 1), ln(good, 1)},
		"deleted product": {ln(deleted, 1)},
		"unknown product": {ln(987654, 1)},
		// unavailable wins over a closed store and is reported without the closed store's product
		"mixed": {ln(pc, 1), ln(hidden, 1), ln(pp, 1)},
	}
	for name, lines := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.CreateOrder(context.Background(), buyer, "k-"+name, lines)
			if !errors.Is(err, domain.ErrConflict) || codeOf(err) != "PRODUCT_UNAVAILABLE" {
				t.Fatalf("got %v", err)
			}
		})
	}
	_, err := f.svc.CreateOrder(context.Background(), buyer, "k", []domain.CartLineInput{ln(pp, 1), ln(hidden, 1), ln(good, 1)})
	ids := []int64{pp, hidden}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if want := fmt.Sprintf("products unavailable: %d, %d", ids[0], ids[1]); err == nil || err.Error() != want {
		t.Fatalf("message = %v, want %q", err, want)
	}
	if f.stock(t, good) != 10 || f.count(t, `SELECT count(*) FROM orders`) != 0 {
		t.Fatal("state changed")
	}
}

func TestCreateOrder_NumbersUniqueUnderConcurrency(t *testing.T) {
	f := setupOrders(t)
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 1000})
	const n = 50
	buyers := make([]int64, n)
	for i := range buyers {
		buyers[i] = seed.User(t, f.pool, "", "")
	}
	numbers := make([]string, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			o, err := f.svc.CreateOrder(context.Background(), buyers[i], "k", []domain.CartLineInput{ln(p, 1)})
			numbers[i], errs[i] = o.Number, err
		}()
	}
	close(start)
	wg.Wait()
	seen := map[string]bool{}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("order %d: %v", i, errs[i])
		}
		seen[numbers[i]] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[fmt.Sprintf("NM-20261015-%03d", i)] {
			t.Fatalf("numbers are not 001..%03d: %v", n, numbers)
		}
	}
	if f.stock(t, p) != 1000-n {
		t.Fatalf("stock = %d", f.stock(t, p))
	}
}

func TestCreateOrder_NumberResetsOnNewWIBDay(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 100})
	ctx := context.Background()
	place := func(key string) string {
		t.Helper()
		o, err := f.svc.CreateOrder(ctx, buyer, key, []domain.CartLineInput{ln(p, 1)})
		if err != nil {
			t.Fatal(err)
		}
		return o.Number
	}
	// 16:58 UTC on the 15th is 23:58 WIB.
	f.clock.Advance(time.Date(2026, 10, 15, 16, 58, 0, 0, time.UTC).Sub(f.clock.Now()))
	if a, b := place("a"), place("b"); a != "NM-20261015-001" || b != "NM-20261015-002" {
		t.Fatalf("before midnight WIB: %s %s", a, b)
	}
	f.clock.Advance(3 * time.Minute) // 00:01 WIB on the 16th, still the 15th in UTC
	if c := place("c"); c != "NM-20261016-001" {
		t.Fatalf("after midnight WIB: %s", c)
	}
}

func TestCreateOrder_AcceptByIsCreatedPlus10Min(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	a, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	b, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	pa := seed.Product(t, f.pool, seed.ProductOpts{StoreID: a})
	pb := seed.Product(t, f.pool, seed.ProductOpts{StoreID: b})
	o, err := f.svc.CreateOrder(context.Background(), buyer, "k", []domain.CartLineInput{ln(pa, 1), ln(pb, 1)})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.pool.Query(context.Background(), `SELECT accept_by, created_at FROM seller_orders WHERE order_id = $1`, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var acceptBy, created time.Time
		if err := rows.Scan(&acceptBy, &created); err != nil {
			t.Fatal(err)
		}
		if !created.Equal(t0) || !acceptBy.Equal(t0.Add(10*time.Minute)) {
			t.Fatalf("created %v accept_by %v", created, acceptBy)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("seller orders = %d", n)
	}
}

func TestCreateOrder_EnqueuesNewOrderPushForSeller(t *testing.T) {
	f := setupOrders(t)
	buyer := seed.User(t, f.pool, "", "")
	a, ownerA := seed.Store(t, f.pool, seed.StoreOpts{Name: "A"})
	b, ownerB := seed.Store(t, f.pool, seed.StoreOpts{Name: "B"})
	pa := seed.Product(t, f.pool, seed.ProductOpts{StoreID: a})
	pb := seed.Product(t, f.pool, seed.ProductOpts{StoreID: b})
	o, err := f.svc.CreateOrder(context.Background(), buyer, "k", []domain.CartLineInput{ln(pa, 1), ln(pb, 1)})
	if err != nil {
		t.Fatal(err)
	}
	soByStore := map[int64]int64{}
	for _, so := range o.SellerOrders {
		soByStore[so.SellerID] = so.ID
	}
	for store, owner := range map[int64]int64{a: ownerA, b: ownerB} {
		var kind, body string
		var data map[string]string
		if err := f.pool.QueryRow(context.Background(), `SELECT kind, body, data FROM notifications WHERE user_id = $1`, owner).
			Scan(&kind, &body, &data); err != nil {
			t.Fatalf("store %d owner: %v", store, err)
		}
		if kind != domain.NotifNewOrder || data["orderNumber"] != o.Number || data["orderId"] != fmt.Sprint(o.ID) ||
			data["sellerOrderId"] != fmt.Sprint(soByStore[store]) || body == "" {
			t.Fatalf("notification = %s %q %v", kind, body, data)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1`, buyer); n != 0 {
		t.Fatalf("buyer got %d notifications", n)
	}
}
