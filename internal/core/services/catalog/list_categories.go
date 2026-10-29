package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListCategories(ctx context.Context) ([]domain.Category, error) {
	ctx, span := tracer.Start(ctx, "service.catalog.ListCategories")
	defer span.End()

	items, err := s.categories.GetAll(ctx)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}
	return items, nil
}
