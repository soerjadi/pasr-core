package ordering

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const maxReviewRunes = 500

func (s *service) SubmitReview(ctx context.Context, buyerID, orderItemID int64, rating int32, text string) (domain.Review, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.SubmitReview")
	defer span.End()

	text = strings.TrimSpace(text)
	if rating < 1 || rating > 5 {
		err := invalid("rating must be 1 to 5")
		tracer.RecordError(span, err)
		return domain.Review{}, err
	}
	if utf8.RuneCountInString(text) > maxReviewRunes {
		err := invalid("text must be at most 500 characters")
		tracer.RecordError(span, err)
		return domain.Review{}, err
	}

	var out domain.Review
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		target, err := s.orders.GetReviewTarget(ctx, buyerID, orderItemID)
		if err != nil {
			return err
		}
		if target.SellerOrderStatus != domain.OrderCompleted {
			return domain.NewError(domain.ErrConflict, "ORDER_NOT_COMPLETED", "only items of a completed order can be reviewed")
		}
		// Insert first: a concurrent duplicate waits on the unique index and fails here, so the
		// aggregates below are only touched by the one insert that won.
		out, err = s.reviews.Create(ctx, domain.NewReview{
			OrderItemID: target.ItemID, ProductID: target.ProductID, StoreID: target.StoreID,
			BuyerID: buyerID, Rating: rating, Text: text,
		})
		if err != nil {
			return err
		}
		return s.reviews.AddRating(ctx, target.ProductID, target.StoreID, rating)
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Review{}, err
	}
	return out, nil
}
