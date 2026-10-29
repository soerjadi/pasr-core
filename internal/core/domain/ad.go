package domain

// Ad placement keys; there is deliberately none for cart or checkout.
const (
	AdHomePartnerCarousel = "home_partner_carousel"
	AdHomeSponsoredCard   = "home_sponsored_card"
	AdPostPurchase        = "post_purchase"
	AdWaitState           = "wait_state"
)

// ValidAdPlacement reports whether p is one of the four placement keys.
func ValidAdPlacement(p string) bool {
	switch p {
	case AdHomePartnerCarousel, AdHomeSponsoredCard, AdPostPurchase, AdWaitState:
		return true
	}
	return false
}

// Ad matches the Ad schema in docs/api/openapi.yaml.
type Ad struct {
	ID        int64    `json:"id"`
	Placement string   `json:"placement"`
	Title     string   `json:"title"`
	Subtitle  string   `json:"subtitle"`
	Image     ImageSet `json:"image"`
	TargetURL string   `json:"targetUrl"`
	Label     string   `json:"label"`
}
