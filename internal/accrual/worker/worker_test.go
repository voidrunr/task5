package worker

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	orders    []repo.Order
	rules     []models.GoodsRule
	finished  map[int64]finish
	takeErr   error
	finishErr error
}

type finish struct {
	status  models.AccrualStatus
	accrual *float64
}

func newFakeStore() *fakeStore {
	return &fakeStore{finished: map[int64]finish{}}
}

func (f *fakeStore) TakeOrderForProcessing(ctx context.Context) (repo.Order, bool, error) {
	if f.takeErr != nil {
		return repo.Order{}, false, f.takeErr
	}
	if len(f.orders) == 0 {
		return repo.Order{}, false, nil
	}
	o := f.orders[0]
	f.orders = f.orders[1:]
	o.Status = models.AccrualProcessing
	return o, true, nil
}

func (f *fakeStore) GoodsRules(ctx context.Context) ([]models.GoodsRule, error) {
	return f.rules, nil
}

func (f *fakeStore) FinishOrder(ctx context.Context, orderID int64, status models.AccrualStatus, accrual *float64) error {
	if f.finishErr != nil {
		return f.finishErr
	}
	f.finished[orderID] = finish{status: status, accrual: accrual}
	return nil
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io_Discard{}, nil))
}

type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

func TestProcessOnceEmptyQueue(t *testing.T) {
	w := New(newFakeStore(), 0, quietLog())
	w.ProcessOnce(context.Background())
}

func TestProcessOnceAccrual(t *testing.T) {
	f := newFakeStore()
	f.rules = []models.GoodsRule{{Match: "Bork", Reward: 10, RewardType: models.RewardPercent}}
	f.orders = []repo.Order{{
		ID:     1,
		Number: "12345678903",
		Items:  []models.OrderItem{{Description: "Чайник Bork", Price: 7000}},
	}}
	w := New(f, 0, quietLog())
	w.ProcessOnce(context.Background())

	got := f.finished[1]
	require.Equal(t, models.AccrualProcessed, got.status)
	require.NotNil(t, got.accrual)
	assert.InDelta(t, 700.0, *got.accrual, 0.001)
}

func TestProcessOnceNoAccrual(t *testing.T) {
	f := newFakeStore()
	f.orders = []repo.Order{{
		ID:     2,
		Number: "12345678903",
		Items:  []models.OrderItem{{Description: "Обычный", Price: 100}},
	}}
	w := New(f, 0, quietLog())
	w.ProcessOnce(context.Background())

	got := f.finished[2]
	require.Equal(t, models.AccrualProcessed, got.status)
	assert.Nil(t, got.accrual)
}

func TestProcessOnceErrors(t *testing.T) {
	f := newFakeStore()
	f.takeErr = errors.New("db down")
	w := New(f, 0, quietLog())
	w.ProcessOnce(context.Background())

	f = newFakeStore()
	f.finishErr = errors.New("db down")
	f.orders = []repo.Order{{ID: 3, Number: "12345678903",
		Items: []models.OrderItem{{Description: "Bork", Price: 100}}}}
	w = New(f, 0, quietLog())
	w.ProcessOnce(context.Background())
	assert.Empty(t, f.finished)
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := New(newFakeStore(), 0, quietLog())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-context.Background().Done():
		t.Fatal("Run did not stop")
	}
}
