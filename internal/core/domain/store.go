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

// MyStore matches the MyStore schema in docs/api/openapi.yaml: the owner's full store profile.
type MyStore struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	CategoryID  int64       `json:"categoryId"`
	OwnerName   string      `json:"ownerName"`
	Kelurahan   string      `json:"kelurahan"`
	Address     string      `json:"address"`
	Description string      `json:"description"`
	Photo       *ImageSet   `json:"photo,omitempty"`
	Status      StoreStatus `json:"status"`
	IsOpen      bool        `json:"isOpen"`
	WhatsApp    string      `json:"whatsapp"`
	Hours       Hours       `json:"hours,omitempty"`
}

// StorePatch matches the UpdateStoreRequest schema: nil fields are left unchanged.
type StorePatch struct {
	Name        *string `json:"name"`
	CategoryID  *int64  `json:"categoryId"`
	OwnerName   *string `json:"ownerName"`
	Kelurahan   *string `json:"kelurahan"`
	Address     *string `json:"address"`
	Description *string `json:"description"`
	PhotoID     *int64  `json:"photoId"`
	WhatsApp    *string `json:"whatsapp"`
	Hours       *Hours  `json:"hours"`
}

// SetOpenRequest matches the SetOpenRequest schema.
type SetOpenRequest struct {
	Open *bool `json:"open"`
}

// MyStoreRecord is a store row as stored for its owner; PhotoKeys is nil without a photo.
type MyStoreRecord struct {
	ID            int64
	Name          string
	CategoryID    int64
	OwnerName     string
	Kelurahan     string
	Address       string
	Description   string
	PhotoUploadID *int64
	PhotoKeys     *ImageKeys
	Status        StoreStatus
	IsOpen        bool
	WhatsApp      string
	Hours         Hours
}

// StoreUpdate is a validated patch for the repository: nil fields are left unchanged.
// WhatsApp is already E.164 and Hours already validated.
type StoreUpdate struct {
	Name          *string
	CategoryID    *int64
	OwnerName     *string
	Kelurahan     *string
	Address       *string
	Description   *string
	PhotoUploadID *int64
	WhatsApp      *string
	Hours         Hours
}
