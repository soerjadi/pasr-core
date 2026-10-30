package ordering_test

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/services/ordering"
)

type fakeCarts struct {
	products map[int64]domain.CartProduct
	asked    []int64
}

func (f *fakeCarts) LoadProducts(_ context.Context, ids []int64, lock bool) (map[int64]domain.CartProduct, error) {
	f.asked = ids
	out := map[int64]domain.CartProduct{}
	for _, id := range ids {
		if p, ok := f.products[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

type fakeStorage struct{}

func (fakeStorage) Put(context.Context, string, string, io.Reader) error { return nil }
func (fakeStorage) Delete(context.Context, string) error                 { return nil }
func (fakeStorage) URL(key string) string                                { return "https://img.test/" + key }

func store(id int64, name string) domain.CartStore {
	return domain.CartStore{ID: id, Name: name, Status: domain.StoreVerified, IsOpen: true, PickupAddress: "Jl. " + name,
		WhatsApp: "+62812", Location: "Menteng", Hours: nil}
}

func product(id int64, st domain.CartStore, price int64, stock int32) domain.CartProduct {
	return domain.CartProduct{ID: id, Name: "P", Price: price, Stock: stock, Status: domain.ProductAvailable, Store: st,
		Image: &domain.ImageKeys{Small: "s", Medium: "m", Large: "l"}}
}

func run(t *testing.T, f *fakeCarts, lines ...domain.CartLineInput) (domain.CartValidation, error) {
	t.Helper()
	return ordering.NewService(f, nil, nil, nil, nil, fakeStorage{}, nil, ordering.Config{}).ValidateCart(context.Background(), lines)
}

func in(id int64, q int32) domain.CartLineInput {
	return domain.CartLineInput{ProductID: id, Quantity: q}
}

func TestValidateCart_PricesFromServerNotClient(t *testing.T) {
	f := &fakeCarts{products: map[int64]domain.CartProduct{1: product(1, store(1, "A"), 15000, 10)}}
	got, err := run(t, f, in(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	l := got.Groups[0].Lines[0]
	if l.UnitPrice != 15000 || l.LineTotal != 45000 || got.Total != 45000 {
		t.Fatalf("line=%+v total=%d", l, got.Total)
	}
	if l.Image.Small != "https://img.test/s" {
		t.Fatalf("image = %+v", l.Image)
	}
}

func TestValidateCart_GroupsBySellerWithPickupInfo(t *testing.T) {
	a, b := store(1, "A"), store(2, "B")
	b.Avatar = &domain.ImageKeys{Small: "as", Medium: "am", Large: "al"}
	f := &fakeCarts{products: map[int64]domain.CartProduct{
		1: product(1, a, 1000, 5), 2: product(2, b, 2000, 5), 3: product(3, a, 3000, 5),
	}}
	got, err := run(t, f, in(1, 1), in(2, 1), in(3, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Groups) != 2 || got.Groups[0].Seller.ID != 1 || got.Groups[1].Seller.ID != 2 {
		t.Fatalf("groups = %+v", got.Groups)
	}
	if len(got.Groups[0].Lines) != 2 || got.Groups[0].Lines[1].ProductID != 3 {
		t.Fatalf("lines of A = %+v", got.Groups[0].Lines)
	}
	s := got.Groups[0].Seller
	if s.PickupAddress != "Jl. A" || s.WhatsApp != "+62812" || s.OpeningHours != "Belum diatur" || s.Avatar != nil {
		t.Fatalf("seller = %+v", s)
	}
	if got.Groups[1].Seller.Avatar == nil || got.Groups[1].Seller.Avatar.Large != "https://img.test/al" {
		t.Fatalf("avatar = %+v", got.Groups[1].Seller.Avatar)
	}
}

func TestValidateCart_MergesDuplicates(t *testing.T) {
	f := &fakeCarts{products: map[int64]domain.CartProduct{1: product(1, store(1, "A"), 1000, 10), 2: product(2, store(1, "A"), 500, 10)}}
	got, err := run(t, f, in(1, 2), in(2, 1), in(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	lines := got.Groups[0].Lines
	if len(lines) != 2 || lines[0].ProductID != 1 || lines[0].Quantity != 5 || lines[0].LineTotal != 5000 || lines[1].ProductID != 2 {
		t.Fatalf("lines = %+v", lines)
	}
	if len(f.asked) != 2 {
		t.Fatalf("loaded ids = %v", f.asked)
	}
	if _, err := run(t, f, in(1, math.MaxInt32), in(1, 1)); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("quantity overflow: %v", err)
	}
}

func TestValidateCart_IssueTable(t *testing.T) {
	mut := func(fn func(*domain.CartProduct)) domain.CartProduct {
		p := product(1, store(1, "A"), 1000, 5)
		fn(&p)
		return p
	}
	tests := []struct {
		name  string
		p     domain.CartProduct
		qty   int32
		issue domain.LineIssue
		avail int32
	}{
		{"ok", mut(func(*domain.CartProduct) {}), 5, "", 5},
		{"out of stock by count", mut(func(p *domain.CartProduct) { p.Stock = 0 }), 1, domain.IssueOutOfStock, 0},
		{"out of stock by status", mut(func(p *domain.CartProduct) { p.Status = domain.ProductOutOfStock }), 1, domain.IssueOutOfStock, 5},
		{"insufficient", mut(func(*domain.CartProduct) {}), 6, domain.IssueInsufficientStock, 5},
		{"hidden", mut(func(p *domain.CartProduct) { p.Status = domain.ProductHidden }), 1, domain.IssueProductUnavailable, 0},
		{"deleted", mut(func(p *domain.CartProduct) { p.Deleted = true }), 1, domain.IssueProductUnavailable, 0},
		{"pending store", mut(func(p *domain.CartProduct) { p.Store.Status = domain.StorePending }), 1, domain.IssueProductUnavailable, 0},
		{"suspended store", mut(func(p *domain.CartProduct) { p.Store.Status = domain.StoreSuspended }), 1, domain.IssueProductUnavailable, 0},
		{"closed store", mut(func(p *domain.CartProduct) { p.Store.IsOpen = false }), 1, domain.IssueProductUnavailable, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := run(t, &fakeCarts{products: map[int64]domain.CartProduct{1: tc.p}}, in(1, tc.qty))
			if err != nil {
				t.Fatal(err)
			}
			l := got.Groups[0].Lines[0]
			if tc.issue == "" && l.Issue != nil || tc.issue != "" && (l.Issue == nil || *l.Issue != tc.issue) {
				t.Fatalf("issue = %v, want %q", l.Issue, tc.issue)
			}
			if l.AvailableStock != tc.avail {
				t.Fatalf("availableStock = %d, want %d", l.AvailableStock, tc.avail)
			}
		})
	}
}

func TestValidateCart_ReturnsEveryLineEvenWhenUnavailable(t *testing.T) {
	gone := product(2, store(1, "A"), 1000, 5)
	gone.Deleted = true
	f := &fakeCarts{products: map[int64]domain.CartProduct{1: product(1, store(1, "A"), 1000, 5), 2: gone}}
	got, err := run(t, f, in(1, 1), in(2, 1), in(404, 2))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, g := range got.Groups {
		n += len(g.Lines)
	}
	if n != 3 || len(got.Groups) != 2 {
		t.Fatalf("want 3 lines in 2 groups, got %d in %d: %+v", n, len(got.Groups), got.Groups)
	}
	missing := got.Groups[1] // the deleted product and the unknown id share the bare group
	if missing.Seller.ID != 0 || len(missing.Lines) != 2 || missing.Lines[1].ProductID != 404 || missing.Lines[1].Issue == nil ||
		*missing.Lines[1].Issue != domain.IssueProductUnavailable || missing.Lines[1].Quantity != 2 {
		t.Fatalf("missing product group = %+v", missing)
	}
}

func TestValidateCart_TotalExcludesIssueLines(t *testing.T) {
	f := &fakeCarts{products: map[int64]domain.CartProduct{
		1: product(1, store(1, "A"), 1000, 5),
		2: product(2, store(1, "A"), 700, 1),
		3: product(3, store(2, "B"), 300, 9),
	}}
	got, err := run(t, f, in(1, 2), in(2, 2), in(3, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2300 {
		t.Fatalf("total = %d, want 2300", got.Total)
	}
}

func TestValidateCart_HiddenDeletedUnverifiedLeakNothing(t *testing.T) {
	mut := func(fn func(*domain.CartProduct)) domain.CartProduct {
		p := product(1, store(1, "A"), 1000, 5)
		fn(&p)
		return p
	}
	cases := map[string]domain.CartProduct{
		"hidden":    mut(func(p *domain.CartProduct) { p.Status = domain.ProductHidden }),
		"deleted":   mut(func(p *domain.CartProduct) { p.Deleted = true }),
		"pending":   mut(func(p *domain.CartProduct) { p.Store.Status = domain.StorePending }),
		"suspended": mut(func(p *domain.CartProduct) { p.Store.Status = domain.StoreSuspended }),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := run(t, &fakeCarts{products: map[int64]domain.CartProduct{1: p}}, in(1, 2))
			if err != nil {
				t.Fatal(err)
			}
			g := got.Groups[0]
			l := g.Lines[0]
			if g.Seller != (domain.SellerSummary{}) || l.Name != "" || l.Image != (domain.ImageSet{}) ||
				l.UnitPrice != 0 || l.LineTotal != 0 || l.AvailableStock != 0 || l.Quantity != 2 ||
				l.Issue == nil || *l.Issue != domain.IssueProductUnavailable || got.Total != 0 {
				t.Fatalf("leaked or wrong shape: %+v", got)
			}
		})
	}
}

func TestValidateCart_ClosedVerifiedStoreStillShowsDetails(t *testing.T) {
	p := product(1, store(1, "A"), 1000, 5)
	p.Store.IsOpen = false
	got, err := run(t, &fakeCarts{products: map[int64]domain.CartProduct{1: p}}, in(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	g := got.Groups[0]
	l := g.Lines[0]
	if g.Seller.ID != 1 || g.Seller.PickupAddress == "" || l.Name != "P" || l.UnitPrice != 1000 || l.LineTotal != 2000 ||
		l.Issue == nil || *l.Issue != domain.IssueProductUnavailable || got.Total != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestValidateCart_OverflowOnUnavailableLineDoesNotFail(t *testing.T) {
	p := product(1, store(1, "A"), math.MaxInt64/2+1, 5)
	p.Store.IsOpen = false
	if _, err := run(t, &fakeCarts{products: map[int64]domain.CartProduct{1: p}}, in(1, 2)); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCart_LineTotalOverflowIs400(t *testing.T) {
	f := &fakeCarts{products: map[int64]domain.CartProduct{1: product(1, store(1, "A"), math.MaxInt64/2+1, 5)}}
	if _, err := run(t, f, in(1, 2)); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCart_Over50Lines400(t *testing.T) {
	f := &fakeCarts{}
	var lines []domain.CartLineInput
	for i := int64(1); i <= 51; i++ {
		lines = append(lines, in(i, 1))
	}
	if _, err := run(t, f, lines...); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("51 lines: %v", err)
	}
	if _, err := run(t, f, lines[:50]...); err != nil {
		t.Fatalf("50 lines: %v", err)
	}
	if _, err := run(t, f); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("empty: %v", err)
	}
}

func TestValidateCart_ZeroOrNegativeQuantity400(t *testing.T) {
	f := &fakeCarts{}
	for _, q := range []int32{0, -1} {
		if _, err := run(t, f, in(1, q)); !errors.Is(err, domain.ErrValidationFailed) {
			t.Fatalf("qty %d: %v", q, err)
		}
	}
	if _, err := run(t, f, in(0, 1)); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("product 0: %v", err)
	}
}
