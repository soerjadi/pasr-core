package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
)

// fakeCategories implements only GetAll; the embedded interface panics on any other method.
type fakeCategories struct {
	repository.CategoryRepository
	items []domain.Category
	err   error
}

func (f fakeCategories) GetAll(context.Context) ([]domain.Category, error) { return f.items, f.err }

func TestListCategories(t *testing.T) {
	want := []domain.Category{{ID: 1, Name: "Makanan", IconKey: "food"}}
	got, err := NewService(fakeCategories{items: want}, nil, nil, nil, nil, nil, nil, Config{}, nil).ListCategories(context.Background())
	if err != nil || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v, %v", got, err)
	}

	boom := errors.New("boom")
	if _, err := NewService(fakeCategories{err: boom}, nil, nil, nil, nil, nil, nil, Config{}, nil).ListCategories(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("want %v, got %v", boom, err)
	}
}
