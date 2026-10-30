package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) DeleteReview(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "service.admin.DeleteReview")
	defer span.End()

	// Delete first: a concurrent delete of the same review waits on the row and then finds
	// nothing, so the aggregates are only reduced by the one delete that won.
	err := s.Tx.WithTx(ctx, func(ctx context.Context) error {
		d, err := s.Reviews.Delete(ctx, id)
		if err != nil {
			return err
		}
		return s.Reviews.SubtractRating(ctx, d.ProductID, d.StoreID, d.Rating)
	})
	if err != nil {
		tracer.RecordError(span, err)
	}
	return err
}
