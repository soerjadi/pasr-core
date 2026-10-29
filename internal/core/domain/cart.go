package domain

// CartStore is the store side of a CartProduct: its status, whether it takes orders and the
// pickup snapshot fields. Avatar is object keys, not URLs.
type CartStore struct {
	ID            int64
	Name          string
	Avatar        *ImageKeys
	Location      string
	Rating        float64
	RatingCount   int32
	PickupAddress string
	Hours         Hours
	WhatsApp      string
	Status        string // PENDING, VERIFIED or SUSPENDED
	IsOpen        bool
}

// CartProduct is a product row loaded for cart validation or order creation, deleted and
// hidden rows included, so the caller classifies them explicitly. Image is the first image's keys.
type CartProduct struct {
	ID      int64
	Name    string
	Price   int64 // rupiah
	Stock   int32
	Status  ProductStatus
	Deleted bool
	Image   *ImageKeys
	Store   CartStore
}

// Orderable reports whether the product can be bought at all: not deleted, not hidden and its
// store is VERIFIED and open. Stock is judged separately.
func (p CartProduct) Orderable() bool {
	return !p.Deleted && p.Status != ProductHidden && p.Store.Status == "VERIFIED" && p.Store.IsOpen
}
