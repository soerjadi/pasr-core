package domain

// ProductStatus matches the SellerProduct.status enum in docs/api/openapi.yaml.
type ProductStatus string

const (
	ProductAvailable  ProductStatus = "AVAILABLE"
	ProductOutOfStock ProductStatus = "OUT_OF_STOCK"
	ProductHidden     ProductStatus = "HIDDEN"
)

// Valid reports whether s is one of the three statuses.
func (s ProductStatus) Valid() bool {
	return s == ProductAvailable || s == ProductOutOfStock || s == ProductHidden
}

// ProductInput matches the CreateProductRequest schema in docs/api/openapi.yaml.
type ProductInput struct {
	Name        string  `json:"name"`
	Price       int64   `json:"price"`
	Stock       int32   `json:"stock"`
	Description string  `json:"description"`
	PhotoIDs    []int64 `json:"photoIds"`
}

// ProductPatch matches the UpdateProductRequest schema: nil fields are left unchanged.
type ProductPatch struct {
	Name        *string        `json:"name"`
	Price       *int64         `json:"price"`
	Stock       *int32         `json:"stock"`
	Description *string        `json:"description"`
	PhotoIDs    *[]int64       `json:"photoIds"`
	Status      *ProductStatus `json:"status"`
}

// SellerProduct matches the SellerProduct schema in docs/api/openapi.yaml.
type SellerProduct struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Price       int64         `json:"price"`
	Stock       int32         `json:"stock"`
	Description string        `json:"description"`
	Status      ProductStatus `json:"status"`
	Images      []ImageSet    `json:"images"`
}

// ProductCounts are the seller's product totals by status, deleted products excluded.
type ProductCounts struct {
	All        int32 `json:"all"`
	Available  int32 `json:"available"`
	OutOfStock int32 `json:"outOfStock"`
	Hidden     int32 `json:"hidden"`
}

// SellerProductPage matches the SellerProductPage schema.
type SellerProductPage struct {
	Items      []SellerProduct `json:"items"`
	NextCursor *string         `json:"nextCursor"`
	Counts     ProductCounts   `json:"counts"`
}

// SellerProductRecord is a product row as stored; images are loaded separately.
type SellerProductRecord struct {
	ID          int64
	StoreID     int64
	CategoryID  int64
	Name        string
	Price       int64
	Stock       int32
	Description string
	Status      ProductStatus
}

// StoreRef identifies the caller's own store.
type StoreRef struct {
	ID         int64
	CategoryID int64
}

// SellerProductQuery is a keyset page request over a store's non-deleted products.
// Status "" means all; BeforeID is exclusive when positive; Limit is the rows to fetch (limit+1).
type SellerProductQuery struct {
	StoreID  int64
	Status   ProductStatus
	BeforeID int64
	Limit    int
}
