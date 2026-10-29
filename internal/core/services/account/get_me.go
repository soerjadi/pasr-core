package account

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetMe(ctx context.Context, userID int64) (domain.User, error) {
	ctx, span := tracer.Start(ctx, "service.account.GetMe")
	defer span.End()

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, err
	}
	return s.withStore(ctx, u)
}

func (s *service) withStore(ctx context.Context, u domain.User) (domain.User, error) {
	st, err := s.stores.GetSummaryByOwner(ctx, u.ID)
	if err != nil {
		return domain.User{}, err
	}
	u.Store = st
	return u, nil
}
