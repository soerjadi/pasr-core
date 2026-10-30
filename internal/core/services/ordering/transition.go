package ordering

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const maxNoteRunes = 500

// params are the inputs of a transition besides the action.
type params struct {
	reason domain.RejectReason
	note   string
}

func (s *service) Accept(ctx context.Context, actor domain.Actor, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.Accept")
	defer span.End()
	out, _, err := s.apply(ctx, actor, id, domain.ActionAccept, params{})
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) Reject(ctx context.Context, actor domain.Actor, id int64, reason, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.Reject")
	defer span.End()
	out, err := s.cancelWith(ctx, actor, id, domain.ActionReject, reason, note)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) Ready(ctx context.Context, actor domain.Actor, id int64, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.Ready")
	defer span.End()
	note, err := cleanNote(note)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrder{}, err
	}
	out, _, err := s.apply(ctx, actor, id, domain.ActionReady, params{note: note})
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) Complete(ctx context.Context, actor domain.Actor, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.Complete")
	defer span.End()
	out, _, err := s.apply(ctx, actor, id, domain.ActionComplete, params{})
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) Cancel(ctx context.Context, actor domain.Actor, id int64, reason, note string) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.Cancel")
	defer span.End()
	out, err := s.cancelWith(ctx, actor, id, domain.ActionCancel, reason, note)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

// cancelWith validates a seller or operator cancellation reason, then applies a.
func (s *service) cancelWith(ctx context.Context, actor domain.Actor, id int64, a domain.Action, reason, note string) (domain.StoreOrder, error) {
	r := domain.RejectReason(reason)
	if !domain.ValidRejectReason(r) {
		return domain.StoreOrder{}, invalid("reason must be OUT_OF_STOCK, BUSY_OR_CLOSED, CANNOT_SERVE or OTHER")
	}
	note, err := cleanNote(note)
	if err != nil {
		return domain.StoreOrder{}, err
	}
	if r == domain.ReasonOther && note == "" {
		return domain.StoreOrder{}, invalid("note is required when reason is OTHER")
	}
	out, _, err := s.apply(ctx, actor, id, a, params{reason: r, note: note})
	return out, err
}

func cleanNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteRunes {
		return "", invalid("note must be at most " + strconv.Itoa(maxNoteRunes) + " characters")
	}
	return note, nil
}

// apply is the only place that changes order state. In one transaction it locks the seller-order
// (FOR UPDATE), checks the move, restores stock on cancellation (products locked in ascending id
// order), stores the new status with its history, recomputes the order status under the order row
// lock and queues the notifications. Lock order: seller-order, products, order; createOrder never
// waits on a seller-order or order row, so the two cannot deadlock. changed is false when the
// seller-order already had the target status.
func (s *service) apply(ctx context.Context, actor domain.Actor, id int64, a domain.Action, p params) (out domain.StoreOrder, changed bool, err error) {
	var storeID int64 // 0: any store
	switch actor.Role {
	case domain.ActorSeller:
		if storeID, err = s.storeOf(ctx, actor.UserID); err != nil {
			return domain.StoreOrder{}, false, err
		}
	case domain.ActorOperator, domain.ActorSystem:
	default:
		return domain.StoreOrder{}, false, orderNotFound()
	}
	target, ok := domain.ActionTarget(a)
	if !ok {
		return domain.StoreOrder{}, false, domain.NewError(domain.ErrConflict, "ORDER_NOT_ACTIONABLE", "unknown action "+string(a))
	}

	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.orders.SetStatementTimeout(ctx, txTimeout); err != nil {
			return err
		}
		l, err := s.orders.LockSellerOrder(ctx, id, storeID)
		if err != nil {
			return err
		}
		if l.Status == target {
			out, err = s.orders.GetStoreOrder(ctx, 0, id)
			return err
		}
		now := s.clock.Now()
		if a == domain.ActionAccept && (expiredCancel(l) || (l.Status == domain.OrderCreated && l.AcceptBy != nil && now.After(*l.AcceptBy))) {
			return domain.NewError(domain.ErrConflict, "ORDER_EXPIRED", "the time to accept this order has passed")
		}
		next, err := domain.Transition(l.Status, a, actor)
		if err != nil {
			return err
		}
		if a == domain.ActionExpire && (l.AcceptBy == nil || now.Before(*l.AcceptBy)) {
			return domain.NewError(domain.ErrConflict, "ORDER_NOT_ACTIONABLE", "order is not overdue")
		}

		change := domain.SellerOrderChange{ID: id, Status: next, At: now}
		if a == domain.ActionReady && p.note != "" {
			change.Note = &p.note
		}
		if next == domain.OrderCancelled {
			reason, by := string(p.reason), actor.Role
			change.CancelReason, change.CancelledBy = &reason, &by
			if p.note != "" {
				change.CancelNote = &p.note
			}
			if err := s.orders.RestoreStock(ctx, id); err != nil {
				return err
			}
		}
		if err := s.orders.UpdateSellerOrder(ctx, change); err != nil {
			return err
		}

		current, statuses, err := s.orders.LockOrderStatuses(ctx, l.OrderID)
		if err != nil {
			return err
		}
		if derived := domain.DeriveOrderStatus(statuses); derived != current {
			if err := s.orders.SetOrderStatus(ctx, l.OrderID, derived, now); err != nil {
				return err
			}
		}

		if err := s.notifyTransition(ctx, l, a, p.reason); err != nil {
			return err
		}
		out, err = s.orders.GetStoreOrder(ctx, 0, id)
		changed = true
		return err
	})
	if err != nil {
		return domain.StoreOrder{}, false, err
	}
	s.presentStore(&out)
	return out, changed, nil
}

// expiredCancel reports whether the seller-order was cancelled by the auto-cancel job.
func expiredCancel(l domain.SellerOrderLock) bool {
	return l.Status == domain.OrderCancelled && l.CancelReason == string(domain.ReasonExpired)
}

var buyerKind = map[domain.Action]string{
	domain.ActionAccept:   domain.NotifOrderAccepted,
	domain.ActionReady:    domain.NotifOrderReady,
	domain.ActionComplete: domain.NotifOrderCompleted,
	domain.ActionReject:   domain.NotifOrderRejected,
	domain.ActionCancel:   domain.NotifOrderCancelled,
	domain.ActionExpire:   domain.NotifOrderCancelled,
}

// notifyTransition queues the buyer's push and, for an expiry, the seller's, in the caller's transaction.
func (s *service) notifyTransition(ctx context.Context, l domain.SellerOrderLock, a domain.Action, reason domain.RejectReason) error {
	data := map[string]string{
		"orderNumber": l.OrderNumber, "orderId": strconv.FormatInt(l.OrderID, 10),
		"sellerOrderId": strconv.FormatInt(l.ID, 10), "store": l.StoreName,
	}
	if reason != "" {
		data["reason"] = string(reason)
	}
	if err := s.notify.Enqueue(ctx, domain.Notification{UserID: l.BuyerID, Kind: buyerKind[a], Data: data}); err != nil {
		return err
	}
	if a == domain.ActionExpire {
		return s.notify.Enqueue(ctx, domain.Notification{UserID: l.OwnerID, Kind: domain.NotifOrderCancelled, Data: data})
	}
	return nil
}

// storeOf returns the owner's store id or fails with 409 NO_STORE.
func (s *service) storeOf(ctx context.Context, ownerID int64) (int64, error) {
	ref, err := s.stores.GetRefByOwner(ctx, ownerID)
	if err != nil {
		return 0, err
	}
	if ref == nil {
		return 0, domain.NewError(domain.ErrConflict, "NO_STORE", "you do not have a store yet")
	}
	return ref.ID, nil
}

func orderNotFound() error {
	return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "order not found")
}

// presentStore turns the stored image keys of every item into URLs.
func (s *service) presentStore(o *domain.StoreOrder) {
	for j := range o.Items {
		if o.Items[j].ImageKeys != nil {
			o.Items[j].Image = s.imageSet(*o.Items[j].ImageKeys)
		}
	}
}
