package worker

import (
	"context"
	"log/slog"
	"time"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/accrual/service"
	"gofermart/internal/models"
)

type Storage interface {
	TakeOrderForProcessing(ctx context.Context) (repo.Order, bool, error)
	GoodsRules(ctx context.Context) ([]models.GoodsRule, error)
	FinishOrder(ctx context.Context, orderID int64, status models.AccrualStatus, accrual *float64) error
}

type Worker struct {
	store    Storage
	interval time.Duration
	log      *slog.Logger
}

func New(store Storage, interval time.Duration, log *slog.Logger) *Worker {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker{store: store, interval: interval, log: log}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ProcessOnce(ctx)
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.processNext(ctx)
		if err != nil {
			w.log.Error("worker: process order", "error", err)
			return
		}
		if !processed {
			return
		}
	}
}

func (w *Worker) processNext(ctx context.Context) (bool, error) {
	order, found, err := w.store.TakeOrderForProcessing(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	rules, err := w.store.GoodsRules(ctx)
	if err != nil {
		return false, err
	}

	total := service.ComputeAccrual(order.Items, rules)
	if total > 0 {
		accrual := total
		if err := w.store.FinishOrder(ctx, order.ID, models.AccrualProcessed, &accrual); err != nil {
			return true, err
		}
		w.log.Info("worker: order processed",
			"order", order.Number, "accrual", accrual)
		return true, nil
	}

	if err := w.store.FinishOrder(ctx, order.ID, models.AccrualProcessed, nil); err != nil {
		return true, err
	}
	w.log.Info("worker: order processed without accrual", "order", order.Number)
	return true, nil
}
