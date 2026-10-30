package domain

import "time"

// AdminStore matches the AdminStore schema in docs/api/openapi.yaml: a store as an operator sees it.
type AdminStore struct {
	ID         int64       `json:"id"`
	OwnerID    int64       `json:"ownerId"`
	Name       string      `json:"name"`
	OwnerName  string      `json:"ownerName"`
	CategoryID int64       `json:"categoryId"`
	Kelurahan  string      `json:"kelurahan"`
	Address    string      `json:"address"`
	WhatsApp   string      `json:"whatsapp"`
	Status     StoreStatus `json:"status"`
	IsOpen     bool        `json:"isOpen"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// ValidStoreStatus reports whether s is PENDING, VERIFIED or SUSPENDED.
func ValidStoreStatus(s StoreStatus) bool {
	return s == StorePending || s == StoreVerified || s == StoreSuspended
}

// AdminStoreQuery is a keyset page request over all stores, newest first. Status "" means all;
// BeforeID is exclusive when positive; Limit is the rows to fetch (limit+1).
type AdminStoreQuery struct {
	Status   StoreStatus
	BeforeID int64
	Limit    int
}

// AdminProductPatch matches the AdminProductPatch schema: nil fields are left unchanged. An empty
// HalalCertNo clears the certificate number.
type AdminProductPatch struct {
	Featured      *bool          `json:"featured"`
	HalalVerified *bool          `json:"halalVerified"`
	HalalCertNo   *string        `json:"halalCertNo"`
	Status        *ProductStatus `json:"status"`
}

// AdminProduct matches the AdminProduct schema: a product with the operator-controlled fields.
type AdminProduct struct {
	ID            int64         `json:"id"`
	StoreID       int64         `json:"storeId"`
	Name          string        `json:"name"`
	Price         int64         `json:"price"`
	Stock         int32         `json:"stock"`
	Status        ProductStatus `json:"status"`
	Featured      bool          `json:"featured"`
	HalalVerified bool          `json:"halalVerified"`
	HalalCertNo   *string       `json:"halalCertNo"`
}
