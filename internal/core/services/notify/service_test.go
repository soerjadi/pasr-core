package notify_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/adapters/secondary/repository/device"
	"github.com/oemah/nusantara/core/internal/adapters/secondary/repository/notification"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/services/notify"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

type fx struct {
	pool *pgxpool.Pool
	exec *db.Executor
	push *testapp.FakePusher
	svc  func() interface {
		Enqueue(context.Context, domain.Notification) error
		Dispatch(context.Context) error
	}
}

func setup(t *testing.T) *fx {
	t.Helper()
	pool := pgtest.Pool(t)
	exec := db.NewExecutor(pool)
	push := testapp.NewFakePusher()
	f := &fx{pool: pool, exec: exec, push: push}
	f.svc = func() interface {
		Enqueue(context.Context, domain.Notification) error
		Dispatch(context.Context) error
	} {
		return notify.NewService(notification.NewNotificationRepository(exec), device.NewDeviceRepository(exec), push, exec)
	}
	return f
}

func (f *fx) device(t *testing.T, uid int64, token string) {
	t.Helper()
	if err := device.NewDeviceRepository(f.exec).Upsert(context.Background(), uid, token); err != nil {
		t.Fatal(err)
	}
}

func (f *fx) enqueue(t *testing.T, uid int64) {
	t.Helper()
	err := f.svc().Enqueue(context.Background(), domain.Notification{UserID: uid, Kind: domain.NotifOrderReady,
		Data: map[string]string{"orderNumber": "NM-1", "store": "Toko"}})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fx) count(t *testing.T, where string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE `+where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEnqueue_RollsBackWithCallerTransaction(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	boom := errors.New("boom")
	err := f.exec.WithTx(context.Background(), func(ctx context.Context) error {
		if err := f.svc().Enqueue(ctx, domain.Notification{UserID: uid, Kind: domain.NotifNewOrder}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(t, "true"); n != 0 {
		t.Fatalf("rows after rollback = %d", n)
	}
	if err := f.exec.WithTx(context.Background(), func(ctx context.Context) error {
		return f.svc().Enqueue(ctx, domain.Notification{UserID: uid, Kind: domain.NotifNewOrder})
	}); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "true"); n != 1 {
		t.Fatalf("rows after commit = %d", n)
	}
}

func TestDispatch_SendsToAllUserDevicesAndMarksSent(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	other := seed.User(t, f.pool, "", "B")
	f.device(t, uid, "tok-1")
	f.device(t, uid, "tok-2")
	f.device(t, other, "tok-other")
	f.enqueue(t, uid)
	if err := f.svc().Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	sends := f.push.Sends()
	if len(sends) != 2 {
		t.Fatalf("sends = %d", len(sends))
	}
	got := map[string]bool{sends[0].Token: true, sends[1].Token: true}
	if !got["tok-1"] || !got["tok-2"] {
		t.Fatalf("tokens = %v", got)
	}
	if sends[0].Notification.Title != "Pesanan siap diambil" || sends[0].Notification.ID == 0 {
		t.Fatalf("notification = %+v", sends[0].Notification)
	}
	if f.count(t, "sent_at IS NOT NULL") != 1 {
		t.Fatal("not marked sent")
	}
}

func TestDispatch_NoDevicesStillMarksSent(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	f.enqueue(t, uid)
	if err := f.svc().Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.push.Sends()) != 0 || f.count(t, "sent_at IS NOT NULL") != 1 {
		t.Fatal("expected no sends and sent_at set")
	}
}

func TestDispatch_FailureRetriesThenAbandons(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	f.device(t, uid, "tok-1")
	f.enqueue(t, uid)
	f.push.FailWith(errors.New("fcm down"))
	for i := 0; i < 8; i++ {
		if err := f.svc().Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(f.push.Sends()); n != 5 {
		t.Fatalf("send attempts = %d, want 5", n)
	}
	var attempts int
	var lastErr *string
	if err := f.pool.QueryRow(context.Background(), `SELECT attempts, last_error FROM notifications`).Scan(&attempts, &lastErr); err != nil {
		t.Fatal(err)
	}
	if attempts != 5 || lastErr == nil || *lastErr == "" {
		t.Fatalf("attempts=%d lastErr=%v", attempts, lastErr)
	}
	if f.count(t, "sent_at IS NOT NULL") != 0 {
		t.Fatal("abandoned notification marked sent")
	}
}

func TestDispatch_UnregisteredTokenPruned(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	f.device(t, uid, "tok-good")
	f.device(t, uid, "tok-dead")
	f.push.MarkUnregistered("tok-dead")
	f.enqueue(t, uid)
	if err := f.svc().Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	toks, err := device.NewDeviceRepository(f.exec).ListTokens(context.Background(), uid)
	if err != nil || len(toks) != 1 || toks[0] != "tok-good" {
		t.Fatalf("tokens = %v err=%v", toks, err)
	}
	if f.count(t, "sent_at IS NOT NULL") != 1 {
		t.Fatal("not marked sent")
	}
}

func TestDispatch_TwoDispatchersDoNotDoubleSend(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	f.device(t, uid, "tok-1")
	const batch, total = 10, 20 // two full batches: each dispatcher must end up with exactly one of them
	for i := 0; i < total; i++ {
		f.enqueue(t, uid)
	}

	// Each dispatcher blocks inside its first Send (rows still locked, transaction open) until the other
	// dispatcher has also reached Send. With SKIP LOCKED both claim disjoint batches and proceed; with a plain
	// FOR UPDATE the second one waits on the first one's locks and the rendezvous times out.
	p := [2]*testapp.FakePusher{testapp.NewFakePusher(), testapp.NewFakePusher()}
	reached := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	var once [2]sync.Once
	for d := 0; d < 2; d++ {
		d := d
		p[d].SetDelay(func(context.Context) {
			once[d].Do(func() { close(reached[d]) })
			select {
			case <-reached[1-d]:
			case <-time.After(5 * time.Second):
				t.Errorf("dispatcher %d: the other dispatcher never claimed rows (claims are not disjoint)", d)
			}
		})
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for d := 0; d < 2; d++ {
		svc := notify.NewService(notification.NewNotificationRepository(f.exec), device.NewDeviceRepository(f.exec), p[d], f.exec)
		notify.Tune(svc, batch, 0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := svc.Dispatch(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()

	seen := map[int64]int{}
	for d := 0; d < 2; d++ {
		sends := p[d].Sends()
		if len(sends) != batch {
			t.Errorf("dispatcher %d sent %d notifications, want %d", d, len(sends), batch)
		}
		for _, s := range sends {
			seen[s.Notification.ID]++
		}
	}
	if len(seen) != total {
		t.Fatalf("distinct notifications sent = %d, want %d", len(seen), total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("notification %d sent %d times", id, n)
		}
	}
	if f.count(t, "sent_at IS NOT NULL") != total {
		t.Fatal("not all notifications marked sent")
	}
}

func TestDispatch_StuckPusherIsBoundedBySendTimeout(t *testing.T) {
	f := setup(t)
	uid := seed.User(t, f.pool, "", "A")
	f.device(t, uid, "tok-1")
	f.enqueue(t, uid)
	f.push.SetDelay(func(ctx context.Context) { <-ctx.Done() })
	svc := notify.NewService(notification.NewNotificationRepository(f.exec), device.NewDeviceRepository(f.exec), f.push, f.exec)
	notify.Tune(svc, 0, 100*time.Millisecond)
	start := time.Now()
	if err := svc.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("dispatch took %v", time.Since(start))
	}
	if f.count(t, "attempts = 1 AND sent_at IS NULL") != 1 {
		t.Fatal("timed-out send should be recorded as a failed attempt")
	}
}
