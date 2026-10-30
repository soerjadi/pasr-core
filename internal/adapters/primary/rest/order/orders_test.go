package order_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

// t0 is 10:02 WIB on 15 October 2026.
var t0 = time.Date(2026, 10, 15, 3, 2, 0, 0, time.UTC)

type buyer struct {
	env   *testapp.Env
	token string
}

func signIn(t *testing.T, env *testapp.Env, phone string) buyer {
	t.Helper()
	return buyer{env: env, token: env.SignIn(phone).AccessToken}
}

func (b buyer) create(key string, lines []line, extra ...testapp.Opt) *testapp.Resp {
	opts := append([]testapp.Opt{testapp.Bearer(b.token)}, extra...)
	if key != "" {
		opts = append(opts, testapp.Header("Idempotency-Key", key))
	}
	return b.env.Do(http.MethodPost, "/v1/orders", line{"lines": lines}, opts...)
}

func (b buyer) get(path string) *testapp.Resp {
	return b.env.Do(http.MethodGet, path, nil, testapp.Bearer(b.token))
}

func decodeOrder(t *testing.T, r *testapp.Resp) domain.Order {
	t.Helper()
	var o domain.Order
	r.JSON(&o)
	return o
}

func code(t *testing.T, r *testapp.Resp) string {
	t.Helper()
	var e struct{ Code, Message string }
	r.JSON(&e)
	return e.Code
}

func stock(t *testing.T, db *pgxpool.Pool, id int64) int32 {
	t.Helper()
	var n int32
	if err := db.QueryRow(context.Background(), `SELECT stock FROM products WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateOrder_SingleSeller_SnapshotsAndTotals(t *testing.T) {
	env, db := setup(t)
	env.Clock.Set(t0)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{Name: "Bu Siti", Address: "Jl. Merdeka 11", WhatsApp: "+6281234567889", Hours: allDays("07:00", "17:00")})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Name: "Keripik Tempe", Price: 15000, Stock: 10, Images: 1})

	r := b.create("k1", []line{{"productId": p, "quantity": 2}})
	if r.Status != http.StatusCreated {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "createOrder", r.Body)
	o := decodeOrder(t, r)
	so := o.SellerOrders[0]
	if o.Number != "NM-20261015-001" || o.Total != 30000 || o.Status != domain.OrderCreated || !o.CreatedAt.Equal(t0) ||
		so.SellerName != "Bu Siti" || so.PickupAddress != "Jl. Merdeka 11" || so.OpeningHours != "Setiap hari 07.00–17.00" ||
		so.Items[0].Name != "Keripik Tempe" || so.Items[0].UnitPrice != 15000 || so.Items[0].Image.Small == "" {
		t.Fatalf("order = %s", r.Body)
	}
	// Timestamps are UTC on the wire.
	var raw struct {
		CreatedAt     string `json:"createdAt"`
		StatusHistory []struct {
			At string `json:"at"`
		} `json:"statusHistory"`
	}
	r.JSON(&raw)
	if raw.CreatedAt != "2026-10-15T03:02:00Z" || raw.StatusHistory[0].At != "2026-10-15T03:02:00Z" {
		t.Fatalf("timestamps = %+v", raw)
	}
}

func TestCreateOrder_TwoSellers_TwoSellerOrdersAndOneNumber(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	sa, _ := seed.Store(t, db, seed.StoreOpts{})
	sb, _ := seed.Store(t, db, seed.StoreOpts{})
	pa := seed.Product(t, db, seed.ProductOpts{StoreID: sa, Price: 1000})
	pb := seed.Product(t, db, seed.ProductOpts{StoreID: sb, Price: 2000})
	r := b.create("k1", []line{{"productId": pa, "quantity": 1}, {"productId": pb, "quantity": 1}})
	if r.Status != http.StatusCreated {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	o := decodeOrder(t, r)
	if len(o.SellerOrders) != 2 || o.Total != 3000 || o.SellerOrders[0].SellerID == o.SellerOrders[1].SellerID {
		t.Fatalf("order = %s", r.Body)
	}
}

func TestCreateOrder_IgnoresClientPrice(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Price: 15000})
	r := env.Do(http.MethodPost, "/v1/orders",
		line{"lines": []line{{"productId": p, "quantity": 2, "unitPrice": 1, "lineTotal": 2}}, "total": 3},
		testapp.Bearer(b.token), testapp.Header("Idempotency-Key", "k"))
	if r.Status != http.StatusCreated {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if o := decodeOrder(t, r); o.Total != 30000 || o.SellerOrders[0].Items[0].UnitPrice != 15000 {
		t.Fatalf("order = %s", r.Body)
	}
}

func TestCreateOrder_DecrementsStock(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 5})
	if r := b.create("k", []line{{"productId": p, "quantity": 3}}); r.Status != http.StatusCreated {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if got := stock(t, db, p); got != 2 {
		t.Fatalf("stock = %d", got)
	}
}

func TestCreateOrder_LastUnitRace_ExactlyOneWins(t *testing.T) {
	env, db := setup(t)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 1})
	buyers := []buyer{signIn(t, env, "081234500001"), signIn(t, env, "081234500002")}

	statuses, codes := make([]int, 2), make([]string, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := buyers[i].create("k", []line{{"productId": p, "quantity": 1}})
			statuses[i] = r.Status
			if r.Status != http.StatusCreated {
				var e struct{ Code string }
				_ = json.Unmarshal(r.Body, &e)
				codes[i] = e.Code
			}
		}()
	}
	close(start)
	wg.Wait()
	won, lost := 0, 0
	for i := range 2 {
		switch {
		case statuses[i] == http.StatusCreated:
			won++
		case statuses[i] == http.StatusConflict && codes[i] == "INSUFFICIENT_STOCK":
			lost++
		}
	}
	if won != 1 || lost != 1 || stock(t, db, p) != 0 {
		t.Fatalf("statuses %v codes %v stock %d", statuses, codes, stock(t, db, p))
	}
}

func TestCreateOrder_MultiLineRollsBackAllStockOnFailure(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p1 := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 5})
	p2 := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 1})
	r := b.create("k", []line{{"productId": p1, "quantity": 2}, {"productId": p2, "quantity": 2}})
	var e struct{ Code, Message string }
	r.JSON(&e)
	if r.Status != http.StatusConflict || e.Code != "INSUFFICIENT_STOCK" || e.Message != fmt.Sprintf("insufficient stock for products: %d", p2) {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if stock(t, db, p1) != 5 || stock(t, db, p2) != 1 {
		t.Fatal("stock changed")
	}
}

func TestCreateOrder_SameKeySequential_ReturnsSameOrder(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 5, Images: 1})
	first := b.create("k", []line{{"productId": p, "quantity": 1}})
	env.Clock.Advance(time.Minute)
	again := b.create("k", []line{{"productId": p, "quantity": 1}})
	if first.Status != http.StatusCreated || again.Status != first.Status || string(again.Body) != string(first.Body) {
		t.Fatalf("first %d %s\nagain %d %s", first.Status, first.Body, again.Status, again.Body)
	}
	if stock(t, db, p) != 4 {
		t.Fatal("replay decremented stock")
	}
}

func TestCreateOrder_SameKeyConcurrent_OneOrder(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 50})
	const n = 10
	bodies, statuses := make([]string, n), make([]int, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := b.create("same", []line{{"productId": p, "quantity": 2}})
			bodies[i], statuses[i] = string(r.Body), r.Status
		}()
	}
	close(start)
	wg.Wait()
	for i := range n {
		if statuses[i] != http.StatusCreated || bodies[i] != bodies[0] {
			t.Fatalf("request %d: %d %s\nfirst: %s", i, statuses[i], bodies[i], bodies[0])
		}
	}
	var orders int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM orders`).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if orders != 1 || stock(t, db, p) != 48 {
		t.Fatalf("orders %d stock %d", orders, stock(t, db, p))
	}
}

func TestCreateOrder_SameKeyDifferentCart422(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	if r := b.create("k", []line{{"productId": p, "quantity": 1}}); r.Status != http.StatusCreated {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	r := b.create("k", []line{{"productId": p, "quantity": 3}})
	if r.Status != http.StatusUnprocessableEntity || code(t, r) != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestCreateOrder_MissingKey400(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	r := b.create("", []line{{"productId": p, "quantity": 1}})
	if r.Status != http.StatusBadRequest || code(t, r) != "VALIDATION_FAILED" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if r := env.Do(http.MethodPost, "/v1/orders", "nope", testapp.Bearer(b.token), testapp.Header("Idempotency-Key", "k")); r.Status != http.StatusBadRequest {
		t.Fatalf("malformed body: %d %s", r.Status, r.Body)
	}
	if stock(t, db, p) != 10 {
		t.Fatal("stock changed")
	}
}

func TestCreateOrder_ClosedStore409StoreClosed(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	if _, err := db.Exec(context.Background(), `UPDATE stores SET is_open = false WHERE id = $1`, store); err != nil {
		t.Fatal(err)
	}
	r := b.create("k", []line{{"productId": p, "quantity": 1}})
	if r.Status != http.StatusConflict || code(t, r) != "STORE_CLOSED" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestCreateOrder_PendingStoreAndHiddenProduct409ProductUnavailable(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	ok, _ := seed.Store(t, db, seed.StoreOpts{})
	pp := seed.Product(t, db, seed.ProductOpts{StoreID: pending})
	hidden := seed.Product(t, db, seed.ProductOpts{StoreID: ok, Status: "HIDDEN"})
	for i, id := range []int64{pp, hidden} {
		r := b.create(fmt.Sprint("k", i), []line{{"productId": id, "quantity": 1}})
		var e struct{ Code, Message string }
		r.JSON(&e)
		if r.Status != http.StatusConflict || e.Code != "PRODUCT_UNAVAILABLE" || e.Message != fmt.Sprintf("products unavailable: %d", id) {
			t.Fatalf("product %d: %d %s", id, r.Status, r.Body)
		}
	}
}

func TestCreateOrder_NumberResetsOnNewWIBDay(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	env.Clock.Set(time.Date(2026, 10, 15, 16, 59, 0, 0, time.UTC)) // 23:59 WIB
	r1 := b.create("a", []line{{"productId": p, "quantity": 1}})
	env.Clock.Advance(2 * time.Minute) // 00:01 WIB on the 16th
	r2 := b.create("b", []line{{"productId": p, "quantity": 1}})
	if n1, n2 := decodeOrder(t, r1).Number, decodeOrder(t, r2).Number; n1 != "NM-20261015-001" || n2 != "NM-20261016-001" {
		t.Fatalf("numbers %s %s", n1, n2)
	}
}

func TestCreateOrder_AcceptByIsCreatedPlus10Min(t *testing.T) {
	env, db := setup(t)
	env.Clock.Set(t0)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	o := decodeOrder(t, b.create("k", []line{{"productId": p, "quantity": 1}}))
	var acceptBy time.Time
	if err := db.QueryRow(context.Background(), `SELECT accept_by FROM seller_orders WHERE order_id = $1`, o.ID).Scan(&acceptBy); err != nil {
		t.Fatal(err)
	}
	if !acceptBy.Equal(t0.Add(10 * time.Minute)) {
		t.Fatalf("accept_by = %v", acceptBy)
	}
}

func TestCreateOrder_EnqueuesNewOrderPushForSeller(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, owner := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	o := decodeOrder(t, b.create("k", []line{{"productId": p, "quantity": 1}}))
	var kind string
	var data map[string]string
	if err := db.QueryRow(context.Background(), `SELECT kind, data FROM notifications WHERE user_id = $1 AND sent_at IS NULL`, owner).
		Scan(&kind, &data); err != nil {
		t.Fatal(err)
	}
	if kind != domain.NotifNewOrder || data["orderNumber"] != o.Number {
		t.Fatalf("notification %s %v", kind, data)
	}
}

func TestCreateOrder_SnapshotSurvivesPriceChangeAndProductDelete(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Name: "Tempe", Price: 15000, Images: 1})
	created := b.create("k", []line{{"productId": p, "quantity": 1}})
	o := decodeOrder(t, created)
	if _, err := db.Exec(context.Background(), `UPDATE products SET price = 1, name = 'X', deleted_at = now() WHERE id = $1`, p); err != nil {
		t.Fatal(err)
	}
	r := b.get(fmt.Sprintf("/v1/orders/%d", o.ID))
	if r.Status != http.StatusOK || string(r.Body) != string(created.Body) {
		t.Fatalf("got %d %s\nwant %s", r.Status, r.Body, created.Body)
	}
}

func TestOrders_RequireAuth(t *testing.T) {
	env, _ := setup(t)
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/v1/orders"}, {http.MethodGet, "/v1/orders"}, {http.MethodGet, "/v1/orders/1"},
	} {
		if r := env.Do(req.method, req.path, nil); r.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", req.method, req.path, r.Status)
		}
	}
}

func TestGetOrder_OtherUser404(t *testing.T) {
	env, db := setup(t)
	a, other := signIn(t, env, "081234500001"), signIn(t, env, "081234500002")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	o := decodeOrder(t, a.create("k", []line{{"productId": p, "quantity": 1}}))
	setNote(t, db, o.ID) // the fixture shows the optional seller note
	path := fmt.Sprintf("/v1/orders/%d", o.ID)
	r := a.get(path)
	if r.Status != http.StatusOK {
		t.Fatalf("owner: %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getOrder", r.Body)
	for _, p := range []string{path, "/v1/orders/987654", "/v1/orders/abc"} {
		if r := other.get(p); r.Status != http.StatusNotFound {
			t.Fatalf("%s: %d %s", p, r.Status, r.Body)
		}
	}
}

func setStatus(t *testing.T, db *pgxpool.Pool, id int64, status domain.OrderStatus) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `UPDATE orders SET status = $2 WHERE id = $1`, id, status); err != nil {
		t.Fatal(err)
	}
}

func setNote(t *testing.T, db *pgxpool.Pool, orderID int64) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `UPDATE seller_orders SET note = 'Siap diambil mulai jam 14.00' WHERE order_id = $1`, orderID); err != nil {
		t.Fatal(err)
	}
}

func listIDs(t *testing.T, r *testapp.Resp) ([]int64, *string) {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("list: %d %s", r.Status, r.Body)
	}
	var page domain.Page[domain.Order]
	r.JSON(&page)
	ids := make([]int64, len(page.Items))
	for i, o := range page.Items {
		ids[i] = o.ID
	}
	return ids, page.NextCursor
}

func TestListOrders_TabsMapToStatuses(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 100})
	byStatus := map[domain.OrderStatus]int64{}
	for i, st := range []domain.OrderStatus{domain.OrderCreated, domain.OrderSellerProcessing, domain.OrderReadyForPickup,
		domain.OrderCompleted, domain.OrderCancelled} {
		id := decodeOrder(t, b.create(fmt.Sprint("k", i), []line{{"productId": p, "quantity": 1}})).ID
		setStatus(t, db, id, st)
		setNote(t, db, id)
		byStatus[st] = id
	}
	r := b.get("/v1/orders")
	contract.AssertShape(t, "listOrders", r.Body)
	want := map[string][]int64{
		"":                {byStatus[domain.OrderCancelled], byStatus[domain.OrderCompleted], byStatus[domain.OrderReadyForPickup], byStatus[domain.OrderSellerProcessing], byStatus[domain.OrderCreated]},
		"?tab=PROCESSING": {byStatus[domain.OrderSellerProcessing], byStatus[domain.OrderCreated]},
		"?tab=READY":      {byStatus[domain.OrderReadyForPickup]},
		"?tab=DONE":       {byStatus[domain.OrderCancelled], byStatus[domain.OrderCompleted]},
	}
	for q, w := range want {
		if got, _ := listIDs(t, b.get("/v1/orders"+q)); fmt.Sprint(got) != fmt.Sprint(w) {
			t.Errorf("%q: got %v want %v", q, got, w)
		}
	}
	for _, q := range []string{"?tab=NEW", "?limit=x", "?cursor=!!"} {
		if r := b.get("/v1/orders" + q); r.Status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", q, r.Status, r.Body)
		}
	}
}

func TestListOrders_NewestFirstCursorStable(t *testing.T) {
	env, db := setup(t)
	b := signIn(t, env, "081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Stock: 100})
	var all []int64
	for i := range 3 {
		all = append([]int64{decodeOrder(t, b.create(fmt.Sprint("k", i), []line{{"productId": p, "quantity": 1}})).ID}, all...)
	}
	first, cur := listIDs(t, b.get("/v1/orders?limit=2"))
	if fmt.Sprint(first) != fmt.Sprint(all[:2]) || cur == nil {
		t.Fatalf("first page %v", first)
	}
	b.create("new", []line{{"productId": p, "quantity": 1}})
	second, cur := listIDs(t, b.get("/v1/orders?limit=2&cursor="+*cur))
	if fmt.Sprint(second) != fmt.Sprint(all[2:]) || cur != nil {
		t.Fatalf("second page %v %v", second, cur)
	}
}

func TestListOrders_BuyerOnlySeesOwn(t *testing.T) {
	env, db := setup(t)
	a, other := signIn(t, env, "081234500001"), signIn(t, env, "081234500002")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	mine := decodeOrder(t, a.create("k", []line{{"productId": p, "quantity": 1}})).ID
	other.create("k", []line{{"productId": p, "quantity": 1}})
	if got, _ := listIDs(t, a.get("/v1/orders")); fmt.Sprint(got) != fmt.Sprint([]int64{mine}) {
		t.Fatalf("got %v", got)
	}
	third := signIn(t, env, "081234500003")
	r := third.get("/v1/orders")
	if r.Status != http.StatusOK || string(r.Body) != "{\"items\":[],\"nextCursor\":null}\n" {
		t.Fatalf("empty: %d %s", r.Status, r.Body)
	}
}
