package domain

import "time"

// StoreStatus matches the StoreStatus schema in docs/api/openapi.yaml.
type StoreStatus string

const (
	StorePending   StoreStatus = "PENDING"
	StoreVerified  StoreStatus = "VERIFIED"
	StoreSuspended StoreStatus = "SUSPENDED"
)

// StoreSummary matches the StoreSummary schema in docs/api/openapi.yaml: the user's own store.
type StoreSummary struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	Status        StoreStatus `json:"status"`
	NewOrderCount int32       `json:"newOrderCount"`
	AcceptBy      *time.Time  `json:"acceptBy,omitempty"`
}

// CreateStoreInput matches the CreateStoreRequest schema in docs/api/openapi.yaml.
type CreateStoreInput struct {
	Name       string `json:"name"`
	CategoryID int64  `json:"categoryId"`
	OwnerName  string `json:"ownerName"`
	Kelurahan  string `json:"kelurahan"`
	Address    string `json:"address"`
	PhotoID    *int64 `json:"photoId,omitempty"`
}

// Store is a store row to be created. Status starts PENDING and hours start NULL.
type Store struct {
	OwnerID       int64
	Name          string
	CategoryID    int64
	OwnerName     string
	Kelurahan     string
	Address       string
	WhatsApp      string
	PhotoUploadID *int64
}
