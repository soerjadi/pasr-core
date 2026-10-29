package ads

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) List(ctx context.Context, placement string) ([]domain.Ad, error) {
	ctx, span := tracer.Start(ctx, "service.ads.List")
	defer span.End()

	if !domain.ValidAdPlacement(placement) {
		err := fmt.Errorf("%w: placement must be one of home_partner_carousel, home_sponsored_card, post_purchase, wait_state", domain.ErrValidationFailed)
		tracer.RecordError(span, err)
		return nil, err
	}
	rows, err := s.ads.ListActive(ctx, placement, s.clock.Now())
	if err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}
	items := make([]domain.Ad, 0, len(rows))
	for _, a := range rows {
		if !safeTarget(a.TargetURL) || strings.TrimSpace(a.Label) == "" {
			log.WarnCtx(ctx, "skipping unsafe ad campaign", log.Field("adId", a.ID), log.Field("placement", placement))
			continue
		}
		items = append(items, a)
	}
	return items, nil
}

func safeTarget(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
