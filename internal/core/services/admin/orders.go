package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// The order methods only delegate: the OrderingService owns the state machine.

func operator(id int64) domain.Actor {
	return domain.Actor{Role: domain.ActorOperator, UserID: id}
}

func (s *service) ListOrders(ctx context.Context, status, cursor string, limit int) (domain.Page[domain.AdminOrder], error) {
	ctx, span := tracer.Start(ctx, "service.admin.ListOrders")
	defer span.End()
	out, err := s.Ordering.ListAllStoreOrders(ctx, status, cursor, limit)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) AcceptOrder(ctx context.Context, operatorID, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.admin.AcceptOrder")
	defer span.End()
	out, err := s.Ordering.Accept(ctx, operator(operatorID), id)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) RejectOrder(ctx context.Context, operatorID, id int64, reason, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.admin.RejectOrder")
	defer span.End()
	out, err := s.Ordering.Reject(ctx, operator(operatorID), id, reason, note)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) ReadyOrder(ctx context.Context, operatorID, id int64, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.admin.ReadyOrder")
	defer span.End()
	out, err := s.Ordering.Ready(ctx, operator(operatorID), id, note)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) CompleteOrder(ctx context.Context, operatorID, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.admin.CompleteOrder")
	defer span.End()
	out, err := s.Ordering.Complete(ctx, operator(operatorID), id)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) CancelOrder(ctx context.Context, operatorID, id int64, reason, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.admin.CancelOrder")
	defer span.End()
	out, err := s.Ordering.Cancel(ctx, operator(operatorID), id, reason, note)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}
