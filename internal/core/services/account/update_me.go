package account

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) UpdateMe(ctx context.Context, userID int64, name string) (domain.User, error) {
	ctx, span := tracer.Start(ctx, "service.account.UpdateMe")
	defer span.End()

	name, err := requireRunes("name", name, 50)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, err
	}
	u, err := s.users.UpdateName(ctx, userID, name)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, err
	}
	u, err = s.withStore(ctx, u)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, err
	}
	return u, nil
}
