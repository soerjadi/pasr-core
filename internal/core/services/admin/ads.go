package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	maxAdvertiserRunes = 100
	maxAdTitleRunes    = 100
	maxAdSubtitleRunes = 200
	maxAdLabelRunes    = 40
	maxAdURLRunes      = 2048
)

func (s *service) ListAdCampaigns(ctx context.Context, cursor string, limit int) (domain.Page[domain.AdCampaign], error) {
	ctx, span := tracer.Start(ctx, "service.admin.ListAdCampaigns")
	defer span.End()

	before, err := domain.DecodeCursor(cursor)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.AdCampaign]{}, err
	}
	limit = domain.ClampLimit(limit)
	rows, err := s.Ads.ListCampaigns(ctx, before, limit+1)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.AdCampaign]{}, err
	}
	return domain.BuildPage(rows, limit, func(c domain.AdCampaign) int64 { return c.ID }), nil
}

func (s *service) CreateAdCampaign(ctx context.Context, in domain.AdCampaignInput) (domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "service.admin.CreateAdCampaign")
	defer span.End()

	c := domain.AdCampaign{Active: true}
	merge(&c, in)
	if err := validateCampaign(&c); err != nil {
		tracer.RecordError(span, err)
		return domain.AdCampaign{}, err
	}
	out, err := s.Ads.CreateCampaign(ctx, c)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.AdCampaign{}, err
	}
	return out, nil
}

func (s *service) UpdateAdCampaign(ctx context.Context, id int64, in domain.AdCampaignInput) (domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "service.admin.UpdateAdCampaign")
	defer span.End()

	var out domain.AdCampaign
	err := s.Tx.WithTx(ctx, func(ctx context.Context) error {
		c, err := s.Ads.LockCampaign(ctx, id)
		if err != nil {
			return err
		}
		if c == nil {
			return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "ad campaign not found")
		}
		merge(c, in)
		if err := validateCampaign(c); err != nil {
			return err
		}
		out, err = s.Ads.UpdateCampaign(ctx, *c)
		return err
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.AdCampaign{}, err
	}
	return out, nil
}

func (s *service) DeleteAdCampaign(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "service.admin.DeleteAdCampaign")
	defer span.End()

	ok, err := s.Ads.DeleteCampaign(ctx, id)
	if err == nil && !ok {
		err = domain.NewError(domain.ErrNotFound, "NOT_FOUND", "ad campaign not found")
	}
	if err != nil {
		tracer.RecordError(span, err)
	}
	return err
}

// merge copies the provided fields of in onto c.
func merge(c *domain.AdCampaign, in domain.AdCampaignInput) {
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	set(&c.Placement, in.Placement)
	set(&c.Advertiser, in.Advertiser)
	set(&c.Title, in.Title)
	set(&c.Subtitle, in.Subtitle)
	set(&c.TargetURL, in.TargetURL)
	set(&c.Label, in.Label)
	if in.Image != nil {
		c.Image = *in.Image
	}
	if in.Priority != nil {
		c.Priority = *in.Priority
	}
	if in.Active != nil {
		c.Active = *in.Active
	}
	if in.StartsAt.Set {
		c.StartsAt = in.StartsAt.Value
	}
	if in.EndsAt.Set {
		c.EndsAt = in.EndsAt.Value
	}
}

// validateCampaign checks a complete campaign and trims its text fields in place. The public ads
// endpoint also skips non-https targets and blank labels, so a bad row can never be served.
func validateCampaign(c *domain.AdCampaign) error {
	if !domain.ValidAdPlacement(c.Placement) {
		return invalid("placement must be one of home_partner_carousel, home_sponsored_card, post_purchase, wait_state")
	}
	texts := []struct {
		field    string
		v        *string
		max      int
		required bool
	}{
		{"advertiser", &c.Advertiser, maxAdvertiserRunes, true},
		{"title", &c.Title, maxAdTitleRunes, true},
		{"subtitle", &c.Subtitle, maxAdSubtitleRunes, false},
		{"label", &c.Label, maxAdLabelRunes, true},
		{"targetUrl", &c.TargetURL, maxAdURLRunes, true},
		{"image.small", &c.Image.Small, maxAdURLRunes, true},
		{"image.medium", &c.Image.Medium, maxAdURLRunes, true},
		{"image.large", &c.Image.Large, maxAdURLRunes, true},
	}
	for _, t := range texts {
		v, err := cleanText(t.field, *t.v, t.max, t.required)
		if err != nil {
			return err
		}
		*t.v = v
	}
	for _, u := range []string{c.TargetURL, c.Image.Small, c.Image.Medium, c.Image.Large} {
		if !domain.SafeAdURL(u) {
			return invalid("targetUrl and image URLs must be absolute https URLs")
		}
	}
	if c.StartsAt != nil && c.EndsAt != nil && !c.StartsAt.Before(*c.EndsAt) {
		return invalid("startsAt must be before endsAt")
	}
	return nil
}
