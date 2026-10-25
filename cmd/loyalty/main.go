package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gofermart/internal/accrualapi"
	"gofermart/internal/auth"
	"gofermart/internal/config"
	"gofermart/internal/gzip"
	"gofermart/internal/loyalty/handler"
	"gofermart/internal/loyalty/repo"
	"gofermart/internal/loyalty/service"
	"gofermart/internal/loyalty/syncer"
	"gofermart/internal/migrate"

	"github.com/jackc/pgx/v5/pgxpool"
)

const shutdownTimeout = 5 * time.Second

func main() {
	log := slog.Default()

	cfg, err := config.ParseLoyalty(os.Args[1:])
	if err != nil {
		log.Error("parse config", "error", err)
		os.Exit(1)
	}
	if cfg.DatabaseURI == "" {
		log.Error("database DSN is required (-d flag or DATABASE_URI)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURI)
	if err != nil {
		log.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Error("ping database", "error", err)
		os.Exit(1)
	}
	if err := migrate.RunEmbedded(ctx, pool); err != nil {
		log.Error("run migrations", "error", err)
		os.Exit(1)
	}

	store := repo.New(pool)
	manager := auth.NewManager(cfg.JWTSecret)
	svc := service.New(store, manager)
	h := handler.New(svc, manager.Middleware, log)

	accrualClient := accrualapi.New(cfg.AccrualSystemAddress)
	syn := syncer.New(store, syncer.ErrorMapper{
		Inner:         accrualClient,
		NotRegistered: accrualapi.ErrNotRegistered,
		RateLimited:   accrualapi.ErrRateLimited,
	}, syncer.Options{
		Interval: time.Duration(cfg.SyncInterval) * time.Second,
	}, log)
	go syn.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           gzip.Middleware(h.Router()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("loyalty service listening", "addr", cfg.RunAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "error", err)
	}
}
