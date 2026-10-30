package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListStores(ctx context.Context, status, cursor string, limit int) (domain.Page[domain.AdminStore], error) {
	ctx, span := tracer.Start(ctx, "service.admin.ListStores")
	defer span.End()

	st := domain.StoreStatus(status)
	if st != "" && !domain.ValidStoreStatus(st) {
		err := invalid("status must be PENDING, VERIFIED or SUSPENDED")
		tracer.RecordError(span, err)
		return domain.Page[domain.AdminStore]{}, err
	}
	before, err := domain.DecodeCursor(cursor)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.AdminStore]{}, err
	}
	limit = domain.ClampLimit(limit)
	rows, err := s.Stores.ListAdmin(ctx, domain.AdminStoreQuery{Status: st, BeforeID: before, Limit: limit + 1})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.AdminStore]{}, err
	}
	return domain.BuildPage(rows, limit, func(a domain.AdminStore) int64 { return a.ID }), nil
}

// storeMove is a verification status change: the statuses it may start from and its target.
type storeMove struct {
	from   []domain.StoreStatus
	to     domain.StoreStatus
	notify bool // queue STORE_VERIFIED to the owner
}

var (
	verifyMove    = storeMove{from: []domain.StoreStatus{domain.StorePending}, to: domain.StoreVerified, notify: true}
	suspendMove   = storeMove{from: []domain.StoreStatus{domain.StorePending, domain.StoreVerified}, to: domain.StoreSuspended}
	reinstateMove = storeMove{from: []domain.StoreStatus{domain.StoreSuspended}, to: domain.StoreVerified}
)

func (s *service) VerifyStore(ctx context.Context, id int64) (domain.AdminStore, error) {
	ctx, span := tracer.Start(ctx, "service.admin.VerifyStore")
	defer span.End()
	out, err := s.moveStore(ctx, id, verifyMove)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) SuspendStore(ctx context.Context, id int64) (domain.AdminStore, error) {
	ctx, span := tracer.Start(ctx, "service.admin.SuspendStore")
	defer span.End()
	out, err := s.moveStore(ctx, id, suspendMove)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (s *service) ReinstateStore(ctx context.Context, id int64) (domain.AdminStore, error) {
	ctx, span := tracer.Start(ctx, "service.admin.ReinstateStore")
	defer span.End()
	out, err := s.moveStore(ctx, id, reinstateMove)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

// moveStore applies m under the store's row lock. A store already in the target status is
// returned unchanged; one in a status m does not start from is 409 STORE_STATE_INVALID.
func (s *service) moveStore(ctx context.Context, id int64, m storeMove) (domain.AdminStore, error) {
	var out domain.AdminStore
	err := s.Tx.WithTx(ctx, func(ctx context.Context) error {
		st, err := s.Stores.GetAdmin(ctx, id, true)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "store not found")
		}
		out = *st
		if st.Status == m.to {
			return nil
		}
		if !allowed(m.from, st.Status) {
			return domain.NewError(domain.ErrConflict, "STORE_STATE_INVALID",
				"store cannot become "+string(m.to)+" from "+string(st.Status))
		}
		if err := s.Stores.SetStatus(ctx, id, m.to); err != nil {
			return err
		}
		out.Status = m.to
		if m.notify {
			return s.Notify.Enqueue(ctx, domain.Notification{
				UserID: st.OwnerID, Kind: domain.NotifStoreVerified, Data: map[string]string{"store": st.Name},
			})
		}
		return nil
	})
	if err != nil {
		return domain.AdminStore{}, err
	}
	return out, nil
}

func allowed(from []domain.StoreStatus, s domain.StoreStatus) bool {
	for _, f := range from {
		if f == s {
			return true
		}
	}
	return false
}
