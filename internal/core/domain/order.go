package domain

import "time"

// OrderStatus matches the OrderStatus schema in docs/api/openapi.yaml.
type OrderStatus string

const (
	OrderCreated          OrderStatus = "CREATED"
	OrderSellerProcessing OrderStatus = "SELLER_PROCESSING"
	OrderReadyForPickup   OrderStatus = "READY_FOR_PICKUP"
	OrderCompleted        OrderStatus = "COMPLETED"
	OrderCancelled        OrderStatus = "CANCELLED"
)

// PaymentMethod matches the PaymentMethod schema in docs/api/openapi.yaml.
type PaymentMethod string

const PaymentCashAtPickup PaymentMethod = "CASH_AT_PICKUP"

// CartLineInput matches the CartLineInput schema in docs/api/openapi.yaml (request line).
type CartLineInput struct {
	ProductID int64 `json:"productId"`
	Quantity  int32 `json:"quantity"`
}

// LineIssue matches the LineIssue schema in docs/api/openapi.yaml.
type LineIssue string

const (
	IssueOutOfStock         LineIssue = "OUT_OF_STOCK"
	IssueInsufficientStock  LineIssue = "INSUFFICIENT_STOCK"
	IssueProductUnavailable LineIssue = "PRODUCT_UNAVAILABLE"
)

// ValidatedLine matches the ValidatedLine schema (the response line; the request line is CartLineInput).
type ValidatedLine struct {
	ProductID      int64      `json:"productId"`
	Name           string     `json:"name"`
	Image          ImageSet   `json:"image"`
	Quantity       int32      `json:"quantity"`
	UnitPrice      int64      `json:"unitPrice"`
	LineTotal      int64      `json:"lineTotal"`
	Issue          *LineIssue `json:"issue,omitempty"`
	AvailableStock int32      `json:"availableStock"`
}

// CartGroup matches the CartGroup schema: validated lines of one seller.
type CartGroup struct {
	Seller SellerSummary   `json:"seller"`
	Lines  []ValidatedLine `json:"lines"`
}

// CartValidation matches the CartValidation schema.
type CartValidation struct {
	Groups []CartGroup `json:"groups"`
	Total  int64       `json:"total"`
}

// StatusChange matches the StatusChange schema.
type StatusChange struct {
	Status OrderStatus `json:"status"`
	At     time.Time   `json:"at"`
}

// OrderItem matches the OrderItem schema.
type OrderItem struct {
	ID        int64    `json:"id"`
	ProductID int64    `json:"productId"`
	Name      string   `json:"name"`
	Image     ImageSet `json:"image"`
	Quantity  int32    `json:"quantity"`
	UnitPrice int64    `json:"unitPrice"`
	LineTotal int64    `json:"lineTotal"`
	Reviewed  bool     `json:"reviewed"`
	// ImageKeys is the snapshot of the product's first image as stored; services turn it into Image.
	ImageKeys *ImageKeys `json:"-"`
}

// SellerOrder matches the SellerOrder schema. Pickup fields are the snapshot taken at order time.
type SellerOrder struct {
	ID            int64       `json:"id"`
	SellerID      int64       `json:"sellerId"`
	SellerName    string      `json:"sellerName"`
	WhatsApp      string      `json:"whatsapp"`
	PickupAddress string      `json:"pickupAddress"`
	OpeningHours  string      `json:"openingHours"`
	Status        OrderStatus `json:"status"`
	Note          string      `json:"note,omitempty"`
	Subtotal      int64       `json:"subtotal"`
	Items         []OrderItem `json:"items"`
}

// Order matches the Order schema (buyer view).
type Order struct {
	ID            int64          `json:"id"`
	Number        string         `json:"number"`
	Status        OrderStatus    `json:"status"`
	CreatedAt     time.Time      `json:"createdAt"`
	Total         int64          `json:"total"`
	PaymentMethod PaymentMethod  `json:"paymentMethod"`
	SellerOrders  []SellerOrder  `json:"sellerOrders"`
	StatusHistory []StatusChange `json:"statusHistory"`
}

// StoreOrder is the seller's view of one seller-order. It is not yet in docs/api/openapi.yaml;
// Task 17 aligns it with the store orders endpoint.
type StoreOrder struct {
	ID            int64          `json:"id"`
	OrderID       int64          `json:"orderId"`
	Number        string         `json:"number"`
	BuyerName     string         `json:"buyerName"`
	Status        OrderStatus    `json:"status"`
	CreatedAt     time.Time      `json:"createdAt"`
	Note          string         `json:"note,omitempty"`
	Subtotal      int64          `json:"subtotal"`
	Items         []OrderItem    `json:"items"`
	StatusHistory []StatusChange `json:"statusHistory"`
}

// Actor is who performs a state transition. Role is "buyer", "seller", "operator" or "system".
type Actor struct {
	Role   string
	UserID int64
}

// NewOrder is an order to insert: the buyer-level row with its seller-orders and their items.
// CreatedAt is used for the order, its seller-orders and the initial CREATED history entries.
type NewOrder struct {
	Number       string
	BuyerID      int64
	Total        int64
	Status       OrderStatus
	CreatedAt    time.Time
	SellerOrders []NewSellerOrder
}

// NewSellerOrder is one store's part of a NewOrder with the pickup snapshot taken from the store.
type NewSellerOrder struct {
	StoreID       int64
	StoreName     string
	WhatsApp      string
	PickupAddress string
	OpeningHours  string
	Status        OrderStatus
	Subtotal      int64
	AcceptBy      time.Time
	Items         []NewOrderItem
}

// NewOrderItem is one line of a NewSellerOrder: name, price and first image are snapshots.
type NewOrderItem struct {
	ProductID int64
	Name      string
	Image     *ImageKeys
	Quantity  int32
	UnitPrice int64
	LineTotal int64
}
