package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	maxCategoryNameRunes = 60
	maxIconKeyRunes      = 40
)

func (s *service) CreateCategory(ctx context.Context, in domain.CategoryInput) (domain.Category, error) {
	ctx, span := tracer.Start(ctx, "service.admin.CreateCategory")
	defer span.End()

	if in.Name == nil || in.IconKey == nil {
		err := invalid("name and iconKey are required")
		tracer.RecordError(span, err)
		return domain.Category{}, err
	}
	name, icon, err := cleanCategory(in)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, err
	}
	out, err := s.Categories.Create(ctx, *name, *icon, in.SortOrder)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, err
	}
	return out, nil
}

func (s *service) UpdateCategory(ctx context.Context, id int64, in domain.CategoryInput) (domain.Category, error) {
	ctx, span := tracer.Start(ctx, "service.admin.UpdateCategory")
	defer span.End()

	name, icon, err := cleanCategory(in)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, err
	}
	out, err := s.Categories.Update(ctx, id, name, icon, in.SortOrder)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, err
	}
	return out, nil
}

func (s *service) DeleteCategory(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "service.admin.DeleteCategory")
	defer span.End()

	if err := s.Categories.Delete(ctx, id); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}

// cleanCategory validates the provided name and iconKey; nil stays nil.
func cleanCategory(in domain.CategoryInput) (name, icon *string, err error) {
	if in.Name != nil {
		n, err := cleanText("name", *in.Name, maxCategoryNameRunes, true)
		if err != nil {
			return nil, nil, err
		}
		name = &n
	}
	if in.IconKey != nil {
		k, err := cleanText("iconKey", *in.IconKey, maxIconKeyRunes, true)
		if err != nil {
			return nil, nil, err
		}
		icon = &k
	}
	return name, icon, nil
}
