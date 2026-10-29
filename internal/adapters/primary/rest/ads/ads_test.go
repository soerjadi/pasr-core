package ads_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

func setup(t *testing.T) (*testapp.Env, *pgxpool.Pool) {
	t.Helper()
	env := testapp.New(t)
	return env, pgtest.Pool(t) // after New: both reset the shared DB, so no rows exist yet
}

type campaign struct {
	placement, title, url, label string
	priority                     int
	active                       bool
	starts, ends                 *time.Time
}

func ok(placement, title string, priority int) campaign {
	return campaign{placement: placement, title: title, url: "https://example.id/" + title, label: "Iklan", priority: priority, active: true}
}

func insert(t *testing.T, db *pgxpool.Pool, c campaign) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(context.Background(), `
		INSERT INTO ad_campaigns (placement, advertiser, title, subtitle, image, target_url, label, priority, active, starts_at, ends_at)
		VALUES ($1, 'adv', $2, 'sub', '{"small":"https://i.test/s.png","medium":"https://i.test/m.png","large":"https://i.test/l.png"}', $3, $4, $5, $6, $7, $8)
		RETURNING id`, c.placement, c.title, c.url, c.label, c.priority, c.active, c.starts, c.ends).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type list struct {
	Items []domain.Ad `json:"items"`
}

func get(t *testing.T, env *testapp.Env, placement string) (*testapp.Resp, list) {
	t.Helper()
	r := env.Do("GET", "/v1/ads?placement="+placement, nil)
	var l list
	if r.Status == http.StatusOK {
		r.JSON(&l)
	}
	return r, l
}

func titles(l list) []string {
	out := []string{}
	for _, a := range l.Items {
		out = append(out, a.Title)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListAds_PriorityOrder(t *testing.T) {
	env, db := setup(t)
	insert(t, db, ok("wait_state", "low", 1))
	insert(t, db, ok("wait_state", "high", 9))
	insert(t, db, ok("wait_state", "tie-a", 5))
	insert(t, db, ok("wait_state", "tie-b", 5))

	r, l := get(t, env, "wait_state")
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "listAds", r.Body)
	if got, want := titles(l), []string{"high", "tie-a", "tie-b", "low"}; !equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if cc := r.Header.Get("Cache-Control"); cc != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if l.Items[0].Image.Small == "" || l.Items[0].Subtitle != "sub" {
		t.Fatalf("unmapped fields: %+v", l.Items[0])
	}
}

func TestListAds_InactiveAndOutOfDateRangeExcluded(t *testing.T) {
	env, db := setup(t)
	now := env.Clock.Now()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)

	inactive := ok("post_purchase", "inactive", 0)
	inactive.active = false
	notStarted := ok("post_purchase", "not-started", 0)
	notStarted.starts = &future
	ended := ok("post_purchase", "ended", 0)
	ended.ends = &past
	endsNow := ok("post_purchase", "ends-now", 0)
	endsNow.ends = &now
	open := ok("post_purchase", "open", 0)
	windowed := ok("post_purchase", "windowed", 0)
	windowed.starts, windowed.ends = &past, &future
	startsNow := ok("post_purchase", "starts-now", 0)
	startsNow.starts = &now
	for _, c := range []campaign{inactive, notStarted, ended, endsNow, open, windowed, startsNow} {
		insert(t, db, c)
	}

	_, l := get(t, env, "post_purchase")
	if got, want := titles(l), []string{"open", "windowed", "starts-now"}; !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	env.Clock.Advance(2 * time.Hour) // windowed expires, not-started begins
	_, l = get(t, env, "post_purchase")
	if got, want := titles(l), []string{"not-started", "open", "starts-now"}; !equal(got, want) {
		t.Fatalf("after advance got %v, want %v", got, want)
	}
}

func TestListAds_PlacementIsolation(t *testing.T) {
	env, db := setup(t)
	insert(t, db, ok("wait_state", "w", 0))
	insert(t, db, ok("home_sponsored_card", "h", 0))

	_, l := get(t, env, "home_sponsored_card")
	if got := titles(l); !equal(got, []string{"h"}) {
		t.Fatalf("got %v", got)
	}
	r, l := get(t, env, "home_partner_carousel")
	if r.Status != http.StatusOK || l.Items == nil || len(l.Items) != 0 {
		t.Fatalf("empty placement: %d %s", r.Status, r.Body)
	}
	var raw map[string]json.RawMessage
	r.JSON(&raw)
	if string(raw["items"]) != "[]" {
		t.Fatalf("items = %s, want []", raw["items"])
	}
}

func TestListAds_UnknownPlacement400(t *testing.T) {
	env, _ := setup(t)
	for _, path := range []string{"/v1/ads", "/v1/ads?placement=", "/v1/ads?placement=cart", "/v1/ads?placement=checkout"} {
		r := env.Do("GET", path, nil)
		var e struct{ Code string }
		r.JSON(&e)
		if r.Status != http.StatusBadRequest || e.Code != "VALIDATION_FAILED" {
			t.Fatalf("%s: got %d %s", path, r.Status, r.Body)
		}
	}
}

func TestListAds_UnsafeURLOrBlankLabelSkipped(t *testing.T) {
	env, db := setup(t)
	httpURL := ok("wait_state", "http", 5)
	httpURL.url = "http://example.id"
	js := ok("wait_state", "js", 5)
	js.url = "javascript:alert(1)"
	blank := ok("wait_state", "blank", 5)
	blank.label = "   "
	insert(t, db, httpURL)
	insert(t, db, js)
	insert(t, db, blank)
	insert(t, db, ok("wait_state", "good", 0))

	_, l := get(t, env, "wait_state")
	if got := titles(l); !equal(got, []string{"good"}) {
		t.Fatalf("got %v", got)
	}
}

func TestListAds_NoAuthRequired(t *testing.T) {
	env, db := setup(t)
	insert(t, db, ok("home_partner_carousel", "x", 0))
	r := env.Do("GET", "/v1/ads?placement=home_partner_carousel", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}
