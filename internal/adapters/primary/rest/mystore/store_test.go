package mystore_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

const storePath = "/v1/me/store"

func weekHours() []map[string]any {
	days := []string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}
	out := make([]map[string]any, 7)
	for i, d := range days {
		out[i] = map[string]any{"day": d, "closed": false, "open": "07:00", "close": "15:00"}
	}
	out[6] = map[string]any{"day": "Min", "closed": true}
	return out
}

func getMyStore(t *testing.T, env *testapp.Env, s domain.Session) domain.MyStore {
	t.Helper()
	r := env.Do(http.MethodGet, storePath, nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("get store: %d %s", r.Status, r.Body)
	}
	var m domain.MyStore
	r.JSON(&m)
	return m
}

func TestGetMyStore_NoStore409NoStore(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	assertErr(t, env.Do(http.MethodGet, storePath, nil, auth(s)), http.StatusConflict, "NO_STORE")
	assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"name": "A"}, auth(s)), http.StatusConflict, "NO_STORE")
	assertErr(t, env.Do(http.MethodPut, storePath+"/open", map[string]any{"open": false}, auth(s)), http.StatusConflict, "NO_STORE")
	if r := env.Do(http.MethodGet, storePath, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.Status)
	}
}

func TestGetMyStore_ReturnsProfileEvenWhenSuspended(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	id, _ := seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID, Status: "SUSPENDED", Photo: true, Hours: nil})
	r := env.Do(http.MethodGet, storePath, nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var m domain.MyStore
	r.JSON(&m)
	if m.ID != id || m.Status != domain.StoreSuspended || m.Photo == nil || m.Photo.Small == "" || !m.IsOpen || m.Hours != nil {
		t.Fatalf("got %+v", m)
	}
	// Edits are still allowed while suspended and do not change the status.
	if r := env.Do(http.MethodPatch, storePath, map[string]any{"name": "Baru"}, auth(s)); r.Status != http.StatusOK {
		t.Fatalf("patch: %d %s", r.Status, r.Body)
	}
	if m := getMyStore(t, env, s); m.Status != domain.StoreSuspended || m.Name != "Baru" {
		t.Fatalf("got %+v", m)
	}
}

func TestUpdateMyStore_PartialPatchKeepsOtherFields(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID, Name: "Lama", Address: "Jl. Lama", Description: "Desc", Photo: true, Hours: seededHours()})

	r := env.Do(http.MethodPatch, storePath, map[string]any{"name": "  Toko Baru  ", "description": ""}, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "updateMyStore", r.Body)
	var m domain.MyStore
	r.JSON(&m)
	if m.Name != "Toko Baru" || m.Description != "" || m.Address != "Jl. Lama" || m.OwnerName != "Pemilik" ||
		m.Kelurahan != "Menteng" || m.Photo == nil || m.Status != domain.StoreVerified || m.WhatsApp != "+6281234567889" {
		t.Fatalf("got %+v", m)
	}
	// An empty patch changes nothing.
	if r := env.Do(http.MethodPatch, storePath, map[string]any{}, auth(s)); r.Status != http.StatusOK {
		t.Fatalf("empty patch: %d %s", r.Status, r.Body)
	}
	if got := getMyStore(t, env, s); got.Name != "Toko Baru" {
		t.Fatalf("got %+v", got)
	}
	// Required fields cannot be emptied or nulled into nothing.
	for _, body := range []map[string]any{
		{"name": ""}, {"name": "   "}, {"ownerName": ""}, {"kelurahan": ""}, {"address": ""}, {"categoryId": 0}, {"categoryId": 999999},
		{"description": string(make([]byte, 0)) + longString(2001)}, {"name": longString(51)}, {"address": longString(201)},
	} {
		assertErr(t, env.Do(http.MethodPatch, storePath, body, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	}
	if got := getMyStore(t, env, s); got.Name != "Toko Baru" || got.Address != "Jl. Lama" {
		t.Fatalf("rejected patches changed the store: %+v", got)
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func TestUpdateMyStore_HoursValidation(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID})

	bad := weekHours()
	bad[2] = map[string]any{"day": "Rab", "closed": false, "open": "16:00", "close": "08:00"}
	badFormat := weekHours()
	badFormat[0]["open"] = "7am"
	for name, hours := range map[string]any{
		"six days": weekHours()[:6], "eight days": append(weekHours(), weekHours()[0]), "empty": []any{},
		"open after close": bad, "bad format": badFormat,
	} {
		t.Run(name, func(t *testing.T) {
			assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"hours": hours}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}
	if m := getMyStore(t, env, s); m.Hours != nil {
		t.Fatalf("hours changed: %+v", m.Hours)
	}
	r := env.Do(http.MethodPatch, storePath, map[string]any{"hours": weekHours()}, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var m domain.MyStore
	r.JSON(&m)
	if len(m.Hours) != 7 || !m.Hours[6].Closed || m.Hours[0].Open != "07:00" {
		t.Fatalf("got %+v", m.Hours)
	}
}

func TestUpdateMyStore_WhatsappNormalized(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID})

	r := env.Do(http.MethodPatch, storePath, map[string]any{"whatsapp": "0812-3456-7000"}, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var m domain.MyStore
	r.JSON(&m)
	if m.WhatsApp != "+6281234567000" {
		t.Fatalf("whatsapp %q", m.WhatsApp)
	}
	for _, bad := range []string{"", "12345", "+1 202 555 0100", "abc"} {
		assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"whatsapp": bad}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	}
	if got := getMyStore(t, env, s); got.WhatsApp != "+6281234567000" {
		t.Fatalf("whatsapp %q", got.WhatsApp)
	}
}

func TestUpdateMyStore_HoursShowAsOpeningHoursInPublicSeller(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	id, _ := seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID, Status: "VERIFIED"})

	if r := env.Do(http.MethodPatch, storePath, map[string]any{"hours": weekHours()}, auth(s)); r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	r := env.Do(http.MethodGet, "/v1/sellers/"+itoa(id), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var d domain.SellerDetail
	r.JSON(&d)
	if want := "Sen–Sab 07.00–15.00, Min tutup"; d.Seller.OpeningHours != want {
		t.Fatalf("openingHours = %q, want %q", d.Seller.OpeningHours, want)
	}
}

func TestUpdateMyStore_PhotoOwnershipChecked(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID})
	other := env.SignIn("+6281299990000")
	foreign := uploadImage(t, env, other)

	assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"photoId": foreign}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"photoId": 999999}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"photoId": 0}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	if m := getMyStore(t, env, s); m.Photo != nil {
		t.Fatalf("photo set by rejected patch: %+v", m.Photo)
	}
	var attached bool
	if err := db.QueryRow(context.Background(), `SELECT attached FROM uploads WHERE id = $1`, foreign).Scan(&attached); err != nil || attached {
		t.Fatalf("foreign upload attached=%v err=%v", attached, err)
	}

	mine := uploadImage(t, env, s)
	r := env.Do(http.MethodPatch, storePath, map[string]any{"photoId": mine}, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var m domain.MyStore
	r.JSON(&m)
	if m.Photo == nil || m.Photo.Small == "" {
		t.Fatalf("got %+v", m)
	}
	if err := db.QueryRow(context.Background(), `SELECT attached FROM uploads WHERE id = $1`, mine).Scan(&attached); err != nil || !attached {
		t.Fatalf("upload attached=%v err=%v", attached, err)
	}
	// An already used upload cannot be claimed again.
	assertErr(t, env.Do(http.MethodPatch, storePath, map[string]any{"photoId": mine}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestSetOpen_ClosedStoreStillListedButFlagStored(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	id, _ := seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID, Status: "VERIFIED"})

	r := env.Do(http.MethodPut, storePath+"/open", map[string]any{"open": false}, auth(s))
	if r.Status != http.StatusNoContent {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if m := getMyStore(t, env, s); m.IsOpen {
		t.Fatalf("isOpen still true")
	}
	if r := env.Do(http.MethodGet, "/v1/sellers/"+itoa(id), nil); r.Status != http.StatusOK {
		t.Fatalf("closed store not visible: %d %s", r.Status, r.Body)
	}
	if r := env.Do(http.MethodPut, storePath+"/open", map[string]any{"open": true}, auth(s)); r.Status != http.StatusNoContent {
		t.Fatalf("%d", r.Status)
	}
	if m := getMyStore(t, env, s); !m.IsOpen {
		t.Fatalf("isOpen not restored")
	}
	assertErr(t, env.Do(http.MethodPut, storePath+"/open", map[string]any{}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func seededHours() domain.Hours {
	h := make(domain.Hours, 7)
	for i := range h {
		h[i] = domain.DayHours{Day: "x", Open: "08:00", Close: "16:00"}
	}
	return h
}
