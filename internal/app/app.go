package app

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/router"
	categoryrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/category"
	"github.com/oemah/nusantara/core/internal/core/domain"
	catalogsvc "github.com/oemah/nusantara/core/internal/core/services/catalog"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
)

// Options overrides collaborators; tests inject fakes here. Later tasks add fields.
type Options struct {
	Clock domain.Clock
}

// App is the composition root: it builds repositories, services and handlers and owns their resources.
type App struct {
	pool    *pgxpool.Pool
	exec    *db.Executor
	clock   domain.Clock
	handler http.Handler
}

func New(ctx context.Context, cfg *config.Config, opts ...Options) (*App, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Clock == nil {
		opt.Clock = domain.SystemClock{}
	}

	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	exec := db.NewExecutor(pool)
	catalog := catalogsvc.NewService(categoryrepo.NewCategoryRepository(exec))

	h := router.Handlers{
		Category: category.NewHandler(catalog),
	}
	ping := func(r *http.Request) error { return pool.Ping(r.Context()) }

	return &App{pool: pool, exec: exec, clock: opt.Clock, handler: router.New(h, cfg.App.AllowedOrigins, ping)}, nil
}

func (a *App) Router() http.Handler { return a.handler }

func (a *App) Shutdown(context.Context) error {
	a.pool.Close()
	return nil
}
