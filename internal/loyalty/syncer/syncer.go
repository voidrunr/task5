package syncer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gofermart/internal/loyalty/repo"
	"gofermart/internal/models"
)

type AccrualAPI interface {
	Accrual(ctx context.Context, number string) (models.AccrualInfo, error)
}

type Storage interface {
	PendingOrders(ctx context.Context, limit int) ([]repo.PendingOrder, error)
	ApplyAccrual(ctx context.Context, orderID int64, status models.OrderStatus, accrual *float64) (bool, error)
}

type Options struct {
	Interval         time.Duration
	BatchSize        int
	RateLimitBackoff time.Duration
}

func DefaultOptions() Options {
	return Options{
		Interval:         5 * time.Second,
		BatchSize:        100,
		RateLimitBackoff: time.Minute,
	}
}

type Syncer struct {
	store            Storage
	api              AccrualAPI
	opts             Options
	log              *slog.Logger
	rateLimitedUntil time.Time
}

func New(store Storage, api AccrualAPI, opts Options, log *slog.Logger) *Syncer {
	if opts.Interval <= 0 {
		opts.Interval = DefaultOptions().Interval
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = DefaultOptions().BatchSize
	}
	if opts.RateLimitBackoff <= 0 {
		opts.RateLimitBackoff = DefaultOptions().RateLimitBackoff
	}
	if log == nil {
		log = slog.Default()
	}
	return &Syncer{store: store, api: api, opts: opts, log: log}
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SyncOnce(ctx)
		}
	}
}

func (s *Syncer) SyncOnce(ctx context.Context) {
	if time.Now().Before(s.rateLimitedUntil) {
		return
	}
	orders, err := s.store.PendingOrders(ctx, s.opts.BatchSize)
	if err != nil {
		s.log.Error("syncer: pending orders", "error", err)
		return
	}
	for _, o := range orders {
		if ctx.Err() != nil {
			return
		}
		if !s.processOrder(ctx, o) {
			return
		}
	}
}

func (s *Syncer) processOrder(ctx context.Context, o repo.PendingOrder) bool {
	info, err := s.api.Accrual(ctx, o.Number)
	switch {
	case errors.Is(err, ErrRateLimitedSentinel):
		s.log.Warn("syncer: rate limited, backing off", "order", o.Number)
		s.rateLimitedUntil = time.Now().Add(s.opts.RateLimitBackoff)
		return false
	case errors.Is(err, ErrNotRegisteredSentinel):

		return true
	case err != nil:
		s.log.Error("syncer: accrual request", "order", o.Number, "error", err)
		return false
	}

	status, accrual := mapStatus(info)
	if _, err := s.store.ApplyAccrual(ctx, o.ID, status, accrual); err != nil {
		s.log.Error("syncer: apply accrual", "order", o.Number, "error", err)
	}
	return true
}

func mapStatus(info models.AccrualInfo) (models.OrderStatus, *float64) {
	switch info.Status {
	case models.AccrualInvalid:
		return models.StatusInvalid, nil
	case models.AccrualProcessed:
		return models.StatusProcessed, info.Accrual
	case models.AccrualProcessing:
		return models.StatusProcessing, nil
	default:
		return models.StatusNew, nil
	}
}

var (
	ErrNotRegisteredSentinel = errors.New("syncer: order not registered")
	ErrRateLimitedSentinel   = errors.New("syncer: rate limited")
)

type ErrorMapper struct {
	Inner         AccrualAPI
	NotRegistered error
	RateLimited   error
}

func (m ErrorMapper) Accrual(ctx context.Context, number string) (models.AccrualInfo, error) {
	info, err := m.Inner.Accrual(ctx, number)
	if err == nil {
		return info, nil
	}
	switch {
	case m.NotRegistered != nil && errors.Is(err, m.NotRegistered):
		return models.AccrualInfo{}, ErrNotRegisteredSentinel
	case m.RateLimited != nil && errors.Is(err, m.RateLimited):
		return models.AccrualInfo{}, ErrRateLimitedSentinel
	default:
		return models.AccrualInfo{}, err
	}
}
