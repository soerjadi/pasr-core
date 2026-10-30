package mystore_test

import (
	"net/http"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

// completedStoreOrder completes the fixture's order, so the store has one sale of 30000 today.
func completedStoreOrder(t *testing.T) *storeOrder {
	t.Helper()
	f := placeStoreOrder(t, 2)
	f.mustPost(t, "accept", nil)
	f.mustPost(t, "ready", map[string]any{"note": "Siap diambil mulai jam 14.00"})
	f.mustPost(t, "complete", nil)
	return f
}

func TestGetMyStoreSummary_ShapeAndFigures(t *testing.T) {
	f := completedStoreOrder(t)
	// A second, waiting order gives newOrderCount and acceptBy.
	r := f.env.Do(http.MethodPost, "/v1/orders", map[string]any{"lines": []map[string]any{{"productId": f.product, "quantity": 1}}},
		auth(f.buyer), testapp.Header("Idempotency-Key", "k2"))
	if r.Status != http.StatusCreated {
		t.Fatalf("second order: %d %s", r.Status, r.Body)
	}
	// Hours: the fixture date (Thursday) closes at 15:00 in weekHours.
	if r := f.env.Do(http.MethodPatch, "/v1/me/store", map[string]any{"hours": weekHours()}, auth(f.seller)); r.Status != http.StatusOK {
		t.Fatalf("hours: %d %s", r.Status, r.Body)
	}
	r = f.env.Do(http.MethodGet, "/v1/me/store/summary", nil, auth(f.seller))
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getMyStoreSummary", r.Body)
	var s domain.StoreSummaryResponse
	r.JSON(&s)
	if !s.IsOpen || s.ClosesAt == nil || *s.ClosesAt != "15:00" || s.NewOrderCount != 1 || s.AcceptBy == nil ||
		s.OrdersToday != 2 || s.SalesToday != 30000 || len(s.LatestOrders) != 2 || s.LatestOrders[0].Status != domain.OrderCreated {
		t.Fatalf("summary = %s", r.Body)
	}
}

func TestGetMyStoreSummary_NoStore409AndAuth401(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	assertErr(t, env.Do(http.MethodGet, "/v1/me/store/summary", nil, auth(s)), http.StatusConflict, "NO_STORE")
	assertErr(t, env.Do(http.MethodGet, "/v1/me/store/stats?range=WEEK", nil, auth(s)), http.StatusConflict, "NO_STORE")
	for _, path := range []string{"/v1/me/store/summary", "/v1/me/store/stats?range=WEEK"} {
		if r := env.Do(http.MethodGet, path, nil); r.Status != http.StatusUnauthorized {
			t.Fatalf("%s anonymous: %d", path, r.Status)
		}
	}
}

func TestGetMyStoreStats_ShapeRangesAndValidation(t *testing.T) {
	f := completedStoreOrder(t)
	r := f.env.Do(http.MethodGet, "/v1/me/store/stats?range=WEEK", nil, auth(f.seller))
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getMyStoreStats", r.Body)
	var st domain.StoreStats
	r.JSON(&st)
	if st.Range != "WEEK" || st.SalesValue != 30000 || st.CompletedOrders != 1 || len(st.Daily) != 7 ||
		st.Daily[6] != (domain.DailyValue{Date: "2026-10-15", Value: 30000}) ||
		len(st.TopProducts) != 1 || st.TopProducts[0].ProductID != f.product || st.TopProducts[0].Name != "Nasi pecel" || st.TopProducts[0].UnitsSold != 2 {
		t.Fatalf("week = %s", r.Body)
	}
	r = f.env.Do(http.MethodGet, "/v1/me/store/stats?range=TODAY", nil, auth(f.seller))
	r.JSON(&st)
	if r.Status != http.StatusOK || st.Range != "TODAY" || st.SalesValue != 30000 || len(st.Daily) != 7 {
		t.Fatalf("today = %d %s", r.Status, r.Body)
	}
	for _, q := range []string{"", "?range=MONTH", "?range=week"} {
		assertErr(t, f.env.Do(http.MethodGet, "/v1/me/store/stats"+q, nil, auth(f.seller)), http.StatusBadRequest, "VALIDATION_FAILED")
	}
	other := f.env.SignIn("+6281234500002")
	seed.Store(t, f.db, seed.StoreOpts{OwnerID: other.User.ID})
	r = f.env.Do(http.MethodGet, "/v1/me/store/stats?range=WEEK", nil, auth(other))
	r.JSON(&st)
	if st.SalesValue != 0 || len(st.TopProducts) != 0 {
		t.Fatalf("other store sees %s", r.Body)
	}
}
