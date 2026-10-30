package app

import (
	"context"
	"fmt"
	"net/http"
	"time"
	_ "time/tzdata" // the business time zone must load even on images without zoneinfo

	"github.com/jackc/pgx/v5/pgxpool"

	adminrest "github.com/oemah/nusantara/core/internal/adapters/primary/rest/admin"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/ads"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/auth"
	catalogrest "github.com/oemah/nusantara/core/internal/adapters/primary/rest/catalog"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/me"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/media"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/middleware"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/mystore"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/order"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/router"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/social"
	"github.com/oemah/nusantara/core/internal/adapters/secondary/otp/logsender"
	"github.com/oemah/nusantara/core/internal/adapters/secondary/push/fcm"
	"github.com/oemah/nusantara/core/internal/adapters/secondary/push/logpusher"
	adrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/ad"
	cartrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/cart"
	categoryrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/category"
	devicerepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/device"
	idemrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/idempotency"
	notificationrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/notification"
	orderrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/order"
	otprepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/otp"
	productrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/product"
	reviewrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/review"
	sessionrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/session"
	socialrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/social"
	statsrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/stats"
	storerepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/store"
	storefrontrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/storefront"
	uploadrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/upload"
	userrepo "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/user"
	localstorage "github.com/oemah/nusantara/core/internal/adapters/secondary/storage/local"
	s3storage "github.com/oemah/nusantara/core/internal/adapters/secondary/storage/s3"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/otp"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/push"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	accountsvc "github.com/oemah/nusantara/core/internal/core/services/account"
	adminsvc "github.com/oemah/nusantara/core/internal/core/services/admin"
	adssvc "github.com/oemah/nusantara/core/internal/core/services/ads"
	catalogsvc "github.com/oemah/nusantara/core/internal/core/services/catalog"
	identitysvc "github.com/oemah/nusantara/core/internal/core/services/identity"
	notifysvc "github.com/oemah/nusantara/core/internal/core/services/notify"
	ordersvc "github.com/oemah/nusantara/core/internal/core/services/ordering"
	storemgmtsvc "github.com/oemah/nusantara/core/internal/core/services/storemgmt"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/infrastructure/jobs"
)

// Options overrides collaborators; tests inject fakes here. Later tasks add fields.
type Options struct {
	Clock     domain.Clock
	OTPSender otp.Sender
	Storage   storage.ObjectStorage // defaults to the adapter chosen by cfg.Storage.Driver
	Pusher    push.Pusher           // defaults to the adapter chosen by cfg.Push.Driver
}

// App is the composition root: it builds repositories, services and handlers and owns their resources.
type App struct {
	pool    *pgxpool.Pool
	exec    *db.Executor
	clock   domain.Clock
	handler http.Handler
	jobs    *jobs.Runner
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

	if opt.Pusher == nil {
		p, err := newPusher(ctx, cfg.Push)
		if err != nil {
			return nil, err
		}
		opt.Pusher = p
	}

	tz := cfg.Orders.Timezone
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("app: orders.timezone: %w", err)
	}

	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	exec := db.NewExecutor(pool)
	catalog := catalogsvc.NewService(
		categoryrepo.NewCategoryRepository(exec),
		productrepo.NewProductRepository(exec),
		storefrontrepo.NewStorefrontRepository(exec),
		reviewrepo.NewReviewRepository(exec),
		socialrepo.NewSocialRepository(exec),
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

	account := accountsvc.NewService(
		userrepo.NewUserRepository(exec),
		storerepo.NewStoreRepository(exec),
		devicerepo.NewDeviceRepository(exec),
		uploadrepo.NewUploadRepository(exec),
		exec,
	)

	storeMgmt := storemgmtsvc.NewService(
		productrepo.NewProductRepository(exec),
		storerepo.NewStoreRepository(exec),
		uploadrepo.NewUploadRepository(exec),
		opt.Storage,
		exec,
	)

	notify := notifysvc.NewService(
		notificationrepo.NewNotificationRepository(exec),
		devicerepo.NewDeviceRepository(exec),
		opt.Pusher,
		exec,
	)

	ordering := ordersvc.NewService(
		cartrepo.NewCartRepository(exec),
		orderrepo.NewOrderRepository(exec),
		reviewrepo.NewReviewRepository(exec),
		idemrepo.NewIdempotencyRepository(exec),
		storerepo.NewStoreRepository(exec),
		statsrepo.NewStatsRepository(exec),
		exec, notify, opt.Storage, opt.Clock,
		ordersvc.Config{AcceptWindow: time.Duration(cfg.Orders.AcceptWindowMinutes) * time.Minute, Location: loc},
	)

	adsService := adssvc.NewService(adrepo.NewAdRepository(exec), opt.Clock)

	adminService := adminsvc.NewService(adminsvc.Deps{
		Stores:     storerepo.NewStoreRepository(exec),
		Products:   productrepo.NewProductRepository(exec),
		Categories: categoryrepo.NewCategoryRepository(exec),
		Ads:        adrepo.NewAdRepository(exec),
		Reviews:    reviewrepo.NewReviewRepository(exec),
		Ordering:   ordering,
		Notify:     notify,
		Tx:         exec,
	})

	files, _ := opt.Storage.(media.FileServer) // only the local driver serves its own files
	h := router.Handlers{
		Admin:        adminrest.NewHandler(adminService),
		Ads:          ads.NewHandler(adsService),
		Auth:         auth.NewHandler(identity),
		Category:     category.NewHandler(catalog),
		Catalog:      catalogrest.NewHandler(catalog),
		Media:        media.NewHandler(catalog, cfg.Images.MaxBytes, files),
		Me:           me.NewHandler(account),
		MyStore:      mystore.NewHandler(storeMgmt, ordering),
		Order:        order.NewHandler(ordering),
		Social:       social.NewHandler(catalog),
		Authenticate: middleware.Auth(identity.ParseAccessToken),
		OptionalAuth: middleware.OptionalAuth(identity.ParseAccessToken),
	}
	ping := func(r *http.Request) error { return pool.Ping(r.Context()) }

	runner := jobs.NewRunner()
	runner.Add("orphan-uploads-gc", time.Hour, func(ctx context.Context) error {
		_, err := catalog.PurgeOrphanUploads(ctx)
		return err
	})

	runner.Add("notifications-dispatch", 5*time.Second, notify.Dispatch)

	// Seller-orders not accepted within the accept window are cancelled and their stock restored.
	runner.Add("orders-auto-cancel", 30*time.Second, func(ctx context.Context) error {
		_, err := ordering.ExpireOverdue(ctx)
		return err
	})

	return &App{pool: pool, exec: exec, clock: opt.Clock, handler: router.New(h, cfg.App.AllowedOrigins, ping), jobs: runner}, nil
}

func (a *App) Router() http.Handler { return a.handler }

// Start launches the background jobs; they run until Shutdown. ctx should outlive startup.
func (a *App) Start(ctx context.Context) { a.jobs.Start(ctx) }

// Shutdown stops the background jobs (waiting for running ones until ctx expires), then releases resources.
func (a *App) Shutdown(ctx context.Context) error {
	err := a.jobs.Stop(ctx)
	a.pool.Close()
	return err
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

func newPusher(ctx context.Context, cfg config.Push) (push.Pusher, error) {
	switch cfg.Driver {
	case "log", "":
		return logpusher.New(), nil
	case "fcm":
		if cfg.FCMCredentialsFile == "" {
			return nil, fmt.Errorf("app: push driver fcm requires push.fcm_credentials_file")
		}
		return fcm.NewFromFile(ctx, cfg.FCMCredentialsFile)
	}
	return nil, fmt.Errorf("app: unknown push driver %q", cfg.Driver)
}
