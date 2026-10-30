package ad

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func scanCampaign(row pgx.Row) (domain.AdCampaign, error) {
	var c domain.AdCampaign
	if err := row.Scan(&c.ID, &c.Placement, &c.Advertiser, &c.Title, &c.Subtitle, &c.Image, &c.TargetURL, &c.Label,
		&c.Priority, &c.Active, &c.StartsAt, &c.EndsAt); err != nil {
		return domain.AdCampaign{}, err
	}
	c.StartsAt, c.EndsAt = utc(c.StartsAt), utc(c.EndsAt)
	return c, nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (r *adRepository) ListCampaigns(ctx context.Context, beforeID int64, limit int) ([]domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.ListCampaigns")
	defer span.End()

	rows, err := r.db.Query(ctx, listCampaignsQuery, beforeID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("ad.ListCampaigns query: %w", err)
	}
	defer rows.Close()
	items := []domain.AdCampaign{}
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("ad.ListCampaigns scan: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("ad.ListCampaigns rows: %w", err)
	}
	return items, nil
}

func (r *adRepository) LockCampaign(ctx context.Context, id int64) (*domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.LockCampaign")
	defer span.End()

	c, err := scanCampaign(r.db.QueryRow(ctx, lockCampaignQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("ad.LockCampaign query: %w", err)
	}
	return &c, nil
}

func (r *adRepository) CreateCampaign(ctx context.Context, c domain.AdCampaign) (domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.CreateCampaign")
	defer span.End()

	out, err := scanCampaign(r.db.QueryRow(ctx, createCampaignQuery, c.Placement, c.Advertiser, c.Title, c.Subtitle, c.Image,
		c.TargetURL, c.Label, c.Priority, c.Active, c.StartsAt, c.EndsAt))
	if err != nil {
		tracer.RecordError(span, err)
		return domain.AdCampaign{}, fmt.Errorf("ad.CreateCampaign insert: %w", err)
	}
	return out, nil
}

func (r *adRepository) UpdateCampaign(ctx context.Context, c domain.AdCampaign) (domain.AdCampaign, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.UpdateCampaign")
	defer span.End()

	out, err := scanCampaign(r.db.QueryRow(ctx, updateCampaignQuery, c.ID, c.Placement, c.Advertiser, c.Title, c.Subtitle,
		c.Image, c.TargetURL, c.Label, c.Priority, c.Active, c.StartsAt, c.EndsAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdCampaign{}, domain.NewError(domain.ErrNotFound, "NOT_FOUND", "ad campaign not found")
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.AdCampaign{}, fmt.Errorf("ad.UpdateCampaign exec: %w", err)
	}
	return out, nil
}

func (r *adRepository) DeleteCampaign(ctx context.Context, id int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.DeleteCampaign")
	defer span.End()

	tag, err := r.db.Exec(ctx, deleteCampaignQuery, id)
	if err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("ad.DeleteCampaign exec: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
