package store

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func sp(s string) *string { return &s }

func TestFindByOwner_NoStoreIsNil(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewStoreRepository(db.NewExecutor(pool))
	uid := seed.User(t, pool, "+6281000000011", "A")
	got, err := r.FindByOwner(context.Background(), uid)
	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestUpdate_ConcurrentPatchesOfDifferentFieldsBothSurvive(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewStoreRepository(db.NewExecutor(pool))
	id, owner := seed.Store(t, pool, seed.StoreOpts{Photo: true})
	ctx := context.Background()

	var wg sync.WaitGroup
	for _, u := range []domain.StoreUpdate{{Name: sp("Baru")}, {Address: sp("Jl. Baru 1")}, {Description: sp("Desc")}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Update(ctx, id, u); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := r.FindByOwner(ctx, owner)
	if err != nil || got == nil {
		t.Fatalf("%v %v", got, err)
	}
	if got.Name != "Baru" || got.Address != "Jl. Baru 1" || got.Description != "Desc" || got.PhotoKeys == nil || got.OwnerName != "Pemilik" {
		t.Fatalf("got %+v", got)
	}
}

func TestUpdate_UnknownCategoryIsValidation(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewStoreRepository(db.NewExecutor(pool))
	id, _ := seed.Store(t, pool, seed.StoreOpts{})
	cat := int64(999999)
	if err := r.Update(context.Background(), id, domain.StoreUpdate{CategoryID: &cat}); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestSetOpen_StoresFlag(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewStoreRepository(db.NewExecutor(pool))
	id, owner := seed.Store(t, pool, seed.StoreOpts{})
	if err := r.SetOpen(context.Background(), id, false); err != nil {
		t.Fatal(err)
	}
	got, _ := r.FindByOwner(context.Background(), owner)
	if got == nil || got.IsOpen {
		t.Fatalf("got %+v", got)
	}
}
