package category

import (
	"context"
	"fmt"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *categoryRepository) GetAll(ctx context.Context) ([]domain.Category, error) {
	ctx, span := tracer.Start(ctx, "repository.category.GetAll")
	defer span.End()

	rows, err := r.db.Query(ctx, getAllQuery)
	if err != nil {
		log.ErrorCtx(ctx, "[repository.category.GetAll] query failed", log.Field("error", err))
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("category.GetAll query: %w", err)
	}
	defer rows.Close()

	items := []domain.Category{}
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.IconKey); err != nil {
			log.ErrorCtx(ctx, "[repository.category.GetAll] scan failed", log.Field("error", err))
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("category.GetAll scan: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		log.ErrorCtx(ctx, "[repository.category.GetAll] iteration failed", log.Field("error", err))
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("category.GetAll rows: %w", err)
	}
	return items, nil
}
