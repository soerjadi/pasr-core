package domain

import "time"

// SellerSummary matches the SellerSummary schema in docs/api/openapi.yaml. The seller is the store.
type SellerSummary struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Avatar        *ImageSet `json:"avatar,omitempty"`
	Location      string    `json:"location"` // the kelurahan
	Rating        float64   `json:"rating"`
	RatingCount   int32     `json:"ratingCount"`
	PickupAddress string    `json:"pickupAddress"`
	OpeningHours  string    `json:"openingHours"`
	WhatsApp      string    `json:"whatsapp"`
}

// SellerDetail matches the SellerDetail schema in docs/api/openapi.yaml.
type SellerDetail struct {
	Seller        SellerSummary `json:"seller"`
	Description   string        `json:"description"`
	FollowerCount int32         `json:"followerCount"`
	IsFollowing   *bool         `json:"isFollowing"`
}

// ProductSummary matches the ProductSummary schema in docs/api/openapi.yaml.
type ProductSummary struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	SellerID      int64    `json:"sellerId"`
	SellerName    string   `json:"sellerName"`
	Price         int64    `json:"price"`
	Image         ImageSet `json:"image"`
	Rating        float64  `json:"rating"`
	RatingCount   int32    `json:"ratingCount"`
	Stock         int32    `json:"stock"`
	HalalVerified bool     `json:"halalVerified"`
}

// ProductDetail matches the ProductDetail schema in docs/api/openapi.yaml.
type ProductDetail struct {
	Product     ProductSummary `json:"product"`
	Description string         `json:"description"`
	Images      []ImageSet     `json:"images"`
	Seller      SellerSummary  `json:"seller"`
	IsFavorite  *bool          `json:"isFavorite"`
}

// ProductPage matches the ProductPage schema.
type ProductPage = Page[ProductSummary]

// ProductFilter narrows listProducts. Zero values mean no filter; ViewerID 0 is anonymous.
type ProductFilter struct {
	CategoryID int64
	SellerID   int64
	Q          string
	Featured   bool
	Cursor     string
	Limit      int
	ViewerID   int64
}

// ProductRecord is a product row as stored: image fields are object keys, not URLs.
type ProductRecord struct {
	ID            int64
	Name          string
	SellerID      int64
	SellerName    string
	Description   string
	Price         int64
	Rating        float64
	RatingCount   int32
	Stock         int32
	HalalVerified bool
	IsFavorite    bool
}

// SellerRecord is a store row as stored: the avatar is object keys, not URLs.
type SellerRecord struct {
	ID            int64
	Name          string
	Avatar        *ImageKeys
	Location      string
	Rating        float64
	RatingCount   int32
	PickupAddress string
	Hours         Hours
	WhatsApp      string
	Description   string
	FollowerCount int32
	IsFollowing   bool
}

// ProductQuery is a keyset page request over visible products: only rows with ID below
// BeforeID are returned when it is positive. Limit is the number of rows to fetch (callers pass limit+1).
type ProductQuery struct {
	CategoryID int64
	SellerID   int64
	Q          string // already validated; the repository escapes LIKE wildcards
	Featured   bool
	BeforeID   int64
	Limit      int
}

var _ = time.Time{}
