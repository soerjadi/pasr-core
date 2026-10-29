package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/auth"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/router"
	"github.com/oemah/nusantara/core/internal/adapters/secondary/otp/logsender"
	categoryrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/category"
	otprepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/otp"
	sessionrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/session"
	uploadrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/upload"
	userrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/user"
	localstorage "github.com/oemah/nusantara/core/internal/adapters/secondary/storage/local"
	s3storage "github.com/oemah/nusantara/core/internal/adapters/secondary/storage/s3"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/otp"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	catalogsvc "github.com/oemah/nusantara/core/internal/core/services/catalog"
	identitysvc "github.com/oemah/nusantara/core/internal/core/services/identity"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
)

// Options overrides collaborators; tests inject fakes here. Later tasks add fields.
type Options struct {
	Clock     domain.Clock
	OTPSender otp.Sender
	Storage   storage.ObjectStorage // defaults to the adapter chosen by cfg.Storage.Driver
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

	if opt.OTPSender == nil {
		if cfg.OTP.Sender != "log" {
			return nil, fmt.Errorf("app: otp sender %q is not available; use \"log\" until a provider adapter exists", cfg.OTP.Sender)
		}
		opt.OTPSender = logsender.New(cfg.App.Env)
	}

	if opt.Storage == nil {
		st, err := newStorage(cfg.Storage)
		if err != nil {
			return nil, err
		}
		opt.Storage = st
	}

	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	exec := db.NewExecutor(pool)
	catalog := catalogsvc.NewService(
		categoryrepo.NewCategoryRepository(exec),
		uploadrepo.NewUploadRepository(exec),
		opt.Storage,
		catalogsvc.Config{MaxImageBytes: cfg.Images.MaxBytes, MaxImagePixels: cfg.Images.MaxPixels},
		opt.Clock,
	)
	identity, err := identitysvc.NewService(
		userrepo.NewUserRepository(exec),
		otprepo.NewOTPRepository(exec),
		sessionrepo.NewSessionRepository(exec),
		exec, opt.OTPSender, opt.Clock,
		identitysvc.Config{
			AccessSecret:   cfg.JWT.AccessSecret,
			AccessTTL:      time.Duration(cfg.JWT.AccessExpiry) * time.Minute,
			RefreshSecret:  cfg.JWT.RefreshSecret,
			RefreshTTL:     time.Duration(cfg.JWT.RefreshExpiry) * time.Minute,
			OTPSecret:      cfg.OTP.Secret,
			OTPTTL:         time.Duration(cfg.OTP.TTLSeconds) * time.Second,
			OTPMaxAttempts: cfg.OTP.MaxAttempts,
			OTPPhoneLimit:  cfg.OTP.PhoneLimit,
			OTPIPLimit:     cfg.OTP.IPLimit,
			OTPWindow:      time.Duration(cfg.OTP.WindowMinutes) * time.Minute,
			OperatorPhones: cfg.App.OperatorPhones,
		},
	)
	if err != nil {
		pool.Close()
		return nil, err
	}

	h := router.Handlers{
		Auth:     auth.NewHandler(identity),
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

func newStorage(cfg config.Storage) (storage.ObjectStorage, error) {
	switch cfg.Driver {
	case "local", "":
		return localstorage.New(cfg.LocalDir, cfg.PublicBaseURL)
	case "s3":
		return s3storage.New(s3storage.Config{
			Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey, PublicBaseURL: cfg.PublicBaseURL,
		})
	}
	return nil, fmt.Errorf("app: unknown storage driver %q", cfg.Driver)
}
