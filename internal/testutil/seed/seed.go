// Package seed inserts rows straight into PostgreSQL for integration tests, bypassing the API.
// Every helper fails the test on error and returns the new row's id. Later tasks reuse and extend it.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// DB is satisfied by *pgxpool.Pool and pgx.Tx.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var counter atomic.Int64

func next() int64 { return counter.Add(1) }

func insert(t testing.TB, db DB, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("seed: %v\n%s", err, sql)
	}
	return id
}

// User inserts a buyer. An empty phone gets a unique one.
func User(t testing.TB, db DB, phone, name string) int64 {
	t.Helper()
	if phone == "" {
		phone = fmt.Sprintf("+62811%08d", next())
	}
	return insert(t, db, `INSERT INTO users (phone, name) VALUES ($1, $2) RETURNING id`, phone, name)
}

// Upload inserts an unattached-or-attached upload whose keys are "seed/<label>-{small,medium,large}.jpg".
func Upload(t testing.TB, db DB, ownerID int64, label string) (int64, domain.ImageKeys) {
	t.Helper()
	keys := domain.ImageKeys{
		Small: "seed/" + label + "-small.jpg", Medium: "seed/" + label + "-medium.jpg", Large: "seed/" + label + "-large.jpg",
	}
	b, _ := json.Marshal(keys)
	return insert(t, db, `INSERT INTO uploads (owner_id, keys, attached) VALUES ($1, $2, true) RETURNING id`, ownerID, b), keys
}

// StoreOpts configures Store; zero values pick sensible defaults.
type StoreOpts struct {
	OwnerID     int64 // 0 creates a new user
	Name        string
	CategoryID  int64  // default 1
	Kelurahan   string // default "Menteng"
	Address     string
	Description string
	Status      string // default VERIFIED
	WhatsApp    string
	Photo       bool // attach an avatar upload
	Hours       domain.Hours
	RatingSum   int64
	RatingCount int32
}

// Store inserts a store (a seller). It returns the store id and its owner's user id.
func Store(t testing.TB, db DB, o StoreOpts) (storeID, ownerID int64) {
	t.Helper()
	if o.OwnerID == 0 {
		o.OwnerID = User(t, db, "", "Owner")
	}
	if o.Name == "" {
		o.Name = fmt.Sprintf("Toko %d", next())
	}
	if o.CategoryID == 0 {
		o.CategoryID = 1
	}
	if o.Kelurahan == "" {
		o.Kelurahan = "Menteng"
	}
	if o.Address == "" {
		o.Address = "Jl. Mawar 3"
	}
	if o.Status == "" {
		o.Status = "VERIFIED"
	}
	if o.WhatsApp == "" {
		o.WhatsApp = "+6281234567889"
	}
	var photo *int64
	if o.Photo {
		id, _ := Upload(t, db, o.OwnerID, fmt.Sprintf("store-%d", next()))
		photo = &id
	}
	var hours any
	if o.Hours != nil {
		b, _ := json.Marshal(o.Hours)
		hours = b
	}
	id := insert(t, db, `
		INSERT INTO stores (owner_id, name, category_id, owner_name, kelurahan, address, description, photo_upload_id,
		                    status, whatsapp, hours, rating_sum, rating_count)
		VALUES ($1, $2, $3, 'Pemilik', $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		o.OwnerID, o.Name, o.CategoryID, o.Kelurahan, o.Address, o.Description, photo, o.Status, o.WhatsApp, hours,
		o.RatingSum, o.RatingCount)
	return id, o.OwnerID
}

// ProductOpts configures Product; zero values pick sensible defaults.
type ProductOpts struct {
	StoreID       int64
	CategoryID    int64 // default 1
	Name          string
	Description   string
	Price         int64 // default 10000
	Stock         int32
	Status        string // default AVAILABLE (OUT_OF_STOCK, HIDDEN)
	Featured      bool
	HalalVerified bool
	RatingSum     int64
	RatingCount   int32
	Deleted       bool
	Images        int // number of images to attach
}

// Product inserts a product of StoreID with o.Images images; Stock defaults to 10 unless Status is OUT_OF_STOCK.
func Product(t testing.TB, db DB, o ProductOpts) int64 {
	t.Helper()
	if o.CategoryID == 0 {
		o.CategoryID = 1
	}
	if o.Name == "" {
		o.Name = fmt.Sprintf("Produk %d", next())
	}
	if o.Price == 0 {
		o.Price = 10000
	}
	if o.Status == "" {
		o.Status = "AVAILABLE"
	}
	if o.Stock == 0 && o.Status != "OUT_OF_STOCK" {
		o.Stock = 10
	}
	var deleted any
	if o.Deleted {
		deleted = time.Now()
	}
	id := insert(t, db, `
		INSERT INTO products (store_id, category_id, name, description, price, stock, status, featured,
		                      halal_verified, rating_sum, rating_count, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		o.StoreID, o.CategoryID, o.Name, o.Description, o.Price, o.Stock, o.Status, o.Featured,
		o.HalalVerified, o.RatingSum, o.RatingCount, deleted)
	var owner int64
	if err := db.QueryRow(context.Background(), `SELECT owner_id FROM stores WHERE id = $1`, o.StoreID).Scan(&owner); err != nil {
		t.Fatalf("seed: store owner: %v", err)
	}
	for i := 0; i < o.Images; i++ {
		up, _ := Upload(t, db, owner, fmt.Sprintf("product-%d-%d", id, i+1))
		if _, err := db.Exec(context.Background(),
			`INSERT INTO product_images (product_id, upload_id, position) VALUES ($1, $2, $3)`, id, up, i); err != nil {
			t.Fatalf("seed: product image: %v", err)
		}
	}
	return id
}

// Favorite marks the product as a favorite of the user.
func Favorite(t testing.TB, db DB, userID, productID int64) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `INSERT INTO favorites (user_id, product_id) VALUES ($1, $2)`, userID, productID); err != nil {
		t.Fatalf("seed: favorite: %v", err)
	}
}

// Follow makes the user follow the store.
func Follow(t testing.TB, db DB, userID, storeID int64) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `INSERT INTO follows (user_id, store_id) VALUES ($1, $2)`, userID, storeID); err != nil {
		t.Fatalf("seed: follow: %v", err)
	}
}

// OrderItem inserts a completed order with one seller order and one line for productID and returns
// the order item id. The buyer must exist; the product must belong to storeID.
func OrderItem(t testing.TB, db DB, buyerID, storeID, productID int64) int64 {
	t.Helper()
	n := next()
	orderID := insert(t, db, `
		INSERT INTO orders (number, buyer_id, total, status) VALUES ($1, $2, 10000, 'COMPLETED') RETURNING id`,
		fmt.Sprintf("NM-SEED-%d", n), buyerID)
	soID := insert(t, db, `
		INSERT INTO seller_orders (order_id, store_id, status, subtotal, store_name, pickup_address, whatsapp)
		VALUES ($1, $2, 'COMPLETED', 10000, 'Toko', 'Jl. Mawar 3', '+6281234567889') RETURNING id`, orderID, storeID)
	return insert(t, db, `
		INSERT INTO order_items (seller_order_id, product_id, name, quantity, unit_price, line_total)
		VALUES ($1, $2, 'Produk', 1, 10000, 10000) RETURNING id`, soID, productID)
}

// ReviewOpts configures Review.
type ReviewOpts struct {
	BuyerID   int64 // 0 creates a buyer with an empty name
	StoreID   int64
	ProductID int64
	Rating    int32 // default 5
	Text      string
	CreatedAt time.Time // default now
}

// Review inserts a review together with the order line it belongs to.
func Review(t testing.TB, db DB, o ReviewOpts) int64 {
	t.Helper()
	if o.BuyerID == 0 {
		o.BuyerID = User(t, db, "", "")
	}
	if o.Rating == 0 {
		o.Rating = 5
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	item := OrderItem(t, db, o.BuyerID, o.StoreID, o.ProductID)
	return insert(t, db, `
		INSERT INTO reviews (order_item_id, product_id, store_id, buyer_id, rating, text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		item, o.ProductID, o.StoreID, o.BuyerID, o.Rating, o.Text, o.CreatedAt)
}
