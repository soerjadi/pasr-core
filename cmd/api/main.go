package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/app"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func main() {
	if err := run(); err != nil {
		log.Error("fatal", log.Field("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfgPath := os.Getenv("CONFIG_FILE")
	if cfgPath == "" {
		cfgPath = "config.toml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	log.Init(log.LogConfig{
		Level:      cfg.Log.Level,
		FilePath:   cfg.Log.FilePath,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
	})

	startCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	tp, err := tracer.Init(startCtx, cfg.Tracing)
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	a, err := app.New(startCtx, cfg)
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      a.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", log.Field("addr", srv.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-quit:
	}

	ctx, stop := context.WithTimeout(context.Background(), time.Duration(cfg.App.ShutdownTimeout)*time.Second)
	defer stop()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("http shutdown", log.Field("error", err))
	}
	if err := a.Shutdown(ctx); err != nil {
		log.Error("app shutdown", log.Field("error", err))
	}
	return tp.Shutdown(ctx) // flush pending spans last
}
