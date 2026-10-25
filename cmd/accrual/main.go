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

	"gofermart/internal/accrual/handler"
	"gofermart/internal/accrual/repo"
	"gofermart/internal/accrual/service"
	"gofermart/internal/accrual/worker"
	"gofermart/internal/config"
	"gofermart/internal/gzip"
	"gofermart/internal/migrate"
	"gofermart/internal/ratelimit"

	"github.com/jackc/pgx/v5/pgxpool"
)

const shutdownTimeout = 5 * time.Second

func main() {
	log := slog.Default()

	cfg, err := config.ParseAccrual(os.Args[1:])
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
	svc := service.New(store)
	limiter := ratelimit.New(cfg.RateLimit)
	h := handler.New(svc, limiter, log)

	w := worker.New(store, time.Duration(cfg.WorkerInterval)*time.Millisecond, log)
	go w.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           gzip.Middleware(h.Router()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("accrual service listening", "addr", cfg.RunAddress)
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
