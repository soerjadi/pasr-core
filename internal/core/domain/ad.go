package domain

import (
	"bytes"
	"encoding/json"
	"net/url"
	"time"
)

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

// SafeAdURL reports whether raw is an absolute https URL with a host, the only kind of link or
// image an ad may carry.
func SafeAdURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
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

// AdCampaign matches the AdCampaign schema of the operator API: a stored campaign with its
// scheduling. Image holds absolute https URLs. A nil StartsAt or EndsAt leaves that side open.
type AdCampaign struct {
	ID         int64      `json:"id"`
	Placement  string     `json:"placement"`
	Advertiser string     `json:"advertiser"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	Image      ImageSet   `json:"image"`
	TargetURL  string     `json:"targetUrl"`
	Label      string     `json:"label"`
	Priority   int32      `json:"priority"`
	Active     bool       `json:"active"`
	StartsAt   *time.Time `json:"startsAt"`
	EndsAt     *time.Time `json:"endsAt"`
}

// AdCampaignInput matches the AdCampaignInput schema: on update nil fields are left unchanged,
// and startsAt or endsAt sent as null clear that bound.
type AdCampaignInput struct {
	Placement  *string      `json:"placement"`
	Advertiser *string      `json:"advertiser"`
	Title      *string      `json:"title"`
	Subtitle   *string      `json:"subtitle"`
	Image      *ImageSet    `json:"image"`
	TargetURL  *string      `json:"targetUrl"`
	Label      *string      `json:"label"`
	Priority   *int32       `json:"priority"`
	Active     *bool        `json:"active"`
	StartsAt   OptionalTime `json:"startsAt"`
	EndsAt     OptionalTime `json:"endsAt"`
}

// OptionalTime tells an absent JSON field (Set false) from an explicit null (Set true, Value nil).
type OptionalTime struct {
	Set   bool
	Value *time.Time
}

func (o *OptionalTime) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(b, []byte("null")) {
		o.Value = nil
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	o.Value = &t
	return nil
}
