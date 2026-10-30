package admin_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
)

func validCampaign() map[string]any {
	return map[string]any{
		"placement":  "home_sponsored_card",
		"advertiser": "Bank BRI",
		"title":      "KUR Bank BRI",
		"subtitle":   "Modal usaha",
		"image": map[string]string{
			"small": "https://cdn.example.id/s.png", "medium": "https://cdn.example.id/m.png", "large": "https://cdn.example.id/l.png",
		},
		"targetUrl": "https://bri.co.id",
		"label":     "Kemitraan",
		"priority":  5,
	}
}

func with(base map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for key, val := range base {
		out[key] = val
	}
	if v == nil {
		delete(out, k)
	} else {
		out[k] = v
	}
	return out
}

func publicAds(t *testing.T, f *fx, placement string) []domain.Ad {
	t.Helper()
	r := f.env.Do(http.MethodGet, "/v1/ads?placement="+placement, nil)
	assertStatus(t, r, http.StatusOK)
	var list struct {
		Items []domain.Ad `json:"items"`
	}
	r.JSON(&list)
	return list.Items
}

func (f *fx) patchCampaign(t *testing.T, id int64, body any) domain.AdCampaign {
	t.Helper()
	r := f.do(http.MethodPatch, path("/v1/admin/ad-campaigns/%d", id), body)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "updateAdCampaign", r.Body)
	var c domain.AdCampaign
	r.JSON(&c)
	return c
}

func TestAdCampaigns_CRUDValidatesURLLabelPlacement(t *testing.T) {
	f := setup(t)

	r := f.do(http.MethodPost, "/v1/admin/ad-campaigns", validCampaign())
	assertStatus(t, r, http.StatusCreated)
	contract.AssertShape(t, "createAdCampaign", r.Body)
	var c domain.AdCampaign
	r.JSON(&c)
	if c.ID == 0 || c.Placement != "home_sponsored_card" || c.Label != "Kemitraan" || !c.Active || c.Priority != 5 ||
		c.Image.Large != "https://cdn.example.id/l.png" || c.StartsAt != nil {
		t.Fatalf("create: %+v", c)
	}
	if ads := publicAds(t, f, "home_sponsored_card"); len(ads) != 1 || ads[0].ID != c.ID || ads[0].TargetURL != "https://bri.co.id" {
		t.Fatalf("public: %+v", ads)
	}

	r = f.do(http.MethodGet, "/v1/admin/ad-campaigns", nil)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "listAdCampaigns", r.Body)
	var page domain.Page[domain.AdCampaign]
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != c.ID || page.Items[0].Advertiser != "Bank BRI" {
		t.Fatalf("admin list: %+v", page)
	}

	// Each invalid create is refused and stores nothing.
	img := func(small string) map[string]string {
		return map[string]string{"small": small, "medium": "https://cdn.example.id/m.png", "large": "https://cdn.example.id/l.png"}
	}
	now := f.env.Clock.Now().UTC()
	for name, body := range map[string]map[string]any{
		"http target":        with(validCampaign(), "targetUrl", "http://bri.co.id"),
		"javascript target":  with(validCampaign(), "targetUrl", "javascript:alert(1)"),
		"relative target":    with(validCampaign(), "targetUrl", "/promo"),
		"missing target":     with(validCampaign(), "targetUrl", nil),
		"blank label":        with(validCampaign(), "label", "   "),
		"missing label":      with(validCampaign(), "label", nil),
		"unknown placement":  with(validCampaign(), "placement", "checkout"),
		"missing placement":  with(validCampaign(), "placement", nil),
		"missing title":      with(validCampaign(), "title", nil),
		"NUL in title":       with(validCampaign(), "title", "KUR\u0000"),
		"long label":         with(validCampaign(), "label", strings.Repeat("x", 41)),
		"http image":         with(validCampaign(), "image", img("http://cdn.example.id/s.png")),
		"missing image":      with(validCampaign(), "image", nil),
		"ends before starts": with(with(validCampaign(), "startsAt", now.Add(time.Hour)), "endsAt", now),
	} {
		t.Run(name, func(t *testing.T) {
			assertErr(t, f.do(http.MethodPost, "/v1/admin/ad-campaigns", body), http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}
	if n := f.scalar(t, `SELECT count(*) FROM ad_campaigns`); n != 1 {
		t.Fatalf("invalid creates stored rows: %d", n)
	}

	// PATCH merges, then validates the result.
	if got := f.patchCampaign(t, c.ID, map[string]any{"label": "Iklan", "title": "KUR BRI"}); got.Label != "Iklan" || got.Title != "KUR BRI" || got.TargetURL != "https://bri.co.id" {
		t.Fatalf("patch: %+v", got)
	}
	for _, body := range []map[string]any{
		{"targetUrl": "http://bri.co.id"}, {"label": ""}, {"placement": "cart"}, {"endsAt": now.Add(-time.Hour), "startsAt": now},
	} {
		assertErr(t, f.do(http.MethodPatch, path("/v1/admin/ad-campaigns/%d", c.ID), body), http.StatusBadRequest, "VALIDATION_FAILED")
	}
	if ads := publicAds(t, f, "home_sponsored_card"); len(ads) != 1 || ads[0].Label != "Iklan" || ads[0].TargetURL != "https://bri.co.id" {
		t.Fatalf("after invalid patches: %+v", ads)
	}

	// A future start hides it; clearing the start with null shows it again; inactive hides it.
	if got := f.patchCampaign(t, c.ID, map[string]any{"startsAt": now.Add(time.Hour)}); got.StartsAt == nil {
		t.Fatalf("startsAt: %+v", got)
	}
	if ads := publicAds(t, f, "home_sponsored_card"); len(ads) != 0 {
		t.Fatalf("future campaign shown: %+v", ads)
	}
	if got := f.patchCampaign(t, c.ID, map[string]any{"startsAt": nil}); got.StartsAt != nil {
		t.Fatalf("clear startsAt: %+v", got)
	}
	if ads := publicAds(t, f, "home_sponsored_card"); len(ads) != 1 {
		t.Fatalf("cleared start not shown: %+v", ads)
	}
	if got := f.patchCampaign(t, c.ID, map[string]any{"active": false}); got.Active {
		t.Fatalf("deactivate: %+v", got)
	}
	if ads := publicAds(t, f, "home_sponsored_card"); len(ads) != 0 {
		t.Fatalf("inactive shown: %+v", ads)
	}

	assertStatus(t, f.do(http.MethodDelete, path("/v1/admin/ad-campaigns/%d", c.ID), nil), http.StatusNoContent)
	assertErr(t, f.do(http.MethodDelete, path("/v1/admin/ad-campaigns/%d", c.ID), nil), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.do(http.MethodPatch, path("/v1/admin/ad-campaigns/%d", c.ID), map[string]any{"label": "x"}), http.StatusNotFound, "NOT_FOUND")
	if n := f.scalar(t, `SELECT count(*) FROM ad_campaigns`); n != 0 {
		t.Fatalf("not deleted: %d", n)
	}
}

func TestAdCampaigns_ListPages(t *testing.T) {
	f := setup(t)
	var ids []int64
	for i := 0; i < 3; i++ {
		r := f.do(http.MethodPost, "/v1/admin/ad-campaigns", validCampaign())
		assertStatus(t, r, http.StatusCreated)
		var c domain.AdCampaign
		r.JSON(&c)
		ids = append(ids, c.ID)
	}
	r := f.do(http.MethodGet, "/v1/admin/ad-campaigns?limit=2", nil)
	var page domain.Page[domain.AdCampaign]
	r.JSON(&page)
	if len(page.Items) != 2 || page.Items[0].ID != ids[2] || page.NextCursor == nil {
		t.Fatalf("page 1: %+v", page)
	}
	r = f.do(http.MethodGet, "/v1/admin/ad-campaigns?limit=2&cursor="+*page.NextCursor, nil)
	page = domain.Page[domain.AdCampaign]{}
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != ids[0] || page.NextCursor != nil {
		t.Fatalf("page 2: %+v", page)
	}
	assertErr(t, f.do(http.MethodGet, "/v1/admin/ad-campaigns?cursor=!", nil), http.StatusBadRequest, "VALIDATION_FAILED")
}
