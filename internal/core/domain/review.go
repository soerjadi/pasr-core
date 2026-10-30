package domain

import "time"

// Review matches the Review schema in docs/api/openapi.yaml.
type Review struct {
	ID         int64     `json:"id"`
	Rating     int32     `json:"rating"`
	Text       string    `json:"text"`
	AuthorName string    `json:"authorName"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ReviewPage matches the ReviewPage schema.
type ReviewPage = Page[Review]

// ReviewTarget is an order item a buyer may review, with the rows its rating counts toward.
type ReviewTarget struct {
	ItemID            int64
	ProductID         int64
	StoreID           int64
	SellerOrderStatus OrderStatus
}

// NewReview is a review to insert.
type NewReview struct {
	OrderItemID int64
	ProductID   int64
	StoreID     int64
	BuyerID     int64
	Rating      int32
	Text        string
}

// DeletedReview is what a removed review counted toward: its product, store and rating.
type DeletedReview struct {
	ProductID int64
	StoreID   int64
	Rating    int32
}
