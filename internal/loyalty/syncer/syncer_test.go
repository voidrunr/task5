package syncer

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"gofermart/internal/loyalty/repo"
	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	pending    []repo.PendingOrder
	pendingErr error
	applied    map[int64]models.OrderStatus
	applyErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{applied: map[int64]models.OrderStatus{}}
}

func (f *fakeStore) PendingOrders(ctx context.Context, limit int) ([]repo.PendingOrder, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	if len(f.pending) > limit {
		return f.pending[:limit], nil
	}
	return f.pending, nil
}

func (f *fakeStore) ApplyAccrual(ctx context.Context, orderID int64, status models.OrderStatus, accrual *float64) (bool, error) {
	if f.applyErr != nil {
		return false, f.applyErr
	}
	f.applied[orderID] = status
	return true, nil
}

type fakeAPI struct {
	infos map[string]models.AccrualInfo
	errs  map[string]error
}

func (f *fakeAPI) Accrual(ctx context.Context, number string) (models.AccrualInfo, error) {
	if err, ok := f.errs[number]; ok {
		return models.AccrualInfo{}, err
	}
	return f.infos[number], nil
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io_Discard{}, nil))
}

type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

func testOpts() Options {
	return Options{Interval: time.Millisecond, BatchSize: 10, RateLimitBackoff: time.Hour}
}

func TestNewDefaults(t *testing.T) {
	s := New(newFakeStore(), &fakeAPI{}, Options{}, nil)
	assert.Equal(t, DefaultOptions().Interval, s.opts.Interval)
	assert.Equal(t, DefaultOptions().BatchSize, s.opts.BatchSize)
	assert.Equal(t, DefaultOptions().RateLimitBackoff, s.opts.RateLimitBackoff)
}

func TestSyncOnceProcessed(t *testing.T) {
	accrual := 500.0
	store := newFakeStore()
	store.pending = []repo.PendingOrder{{ID: 1, Number: "12345678903"}}
	api := &fakeAPI{infos: map[string]models.AccrualInfo{
		"12345678903": {Status: models.AccrualProcessed, Accrual: &accrual},
	}}
	s := New(store, api, testOpts(), quietLog())
	s.SyncOnce(context.Background())

	assert.Equal(t, models.StatusProcessed, store.applied[1])
}

func TestSyncOnceMapsAllStatuses(t *testing.T) {
	accrual := 75.0
	cases := []struct {
		info models.AccrualInfo
		want models.OrderStatus
	}{
		{models.AccrualInfo{Status: models.AccrualInvalid}, models.StatusInvalid},
		{models.AccrualInfo{Status: models.AccrualProcessing}, models.StatusProcessing},
		{models.AccrualInfo{Status: models.AccrualRegistered}, models.StatusNew},
		{models.AccrualInfo{Status: models.AccrualProcessed, Accrual: &accrual}, models.StatusProcessed},
	}
	for i, c := range cases {
		store := newFakeStore()
		store.pending = []repo.PendingOrder{{ID: int64(i + 1), Number: "12345678903"}}
		api := &fakeAPI{infos: map[string]models.AccrualInfo{"12345678903": c.info}}
		s := New(store, api, testOpts(), quietLog())
		s.SyncOnce(context.Background())
		assert.Equal(t, c.want, store.applied[int64(i+1)])
	}
}

func TestSyncOnceNotRegistered(t *testing.T) {
	store := newFakeStore()
	store.pending = []repo.PendingOrder{{ID: 1, Number: "12345678903"}}
	api := &fakeAPI{errs: map[string]error{
		"12345678903": ErrNotRegisteredSentinel,
	}}
	s := New(store, api, testOpts(), quietLog())
	s.SyncOnce(context.Background())
	assert.Empty(t, store.applied)
}

func TestSyncOnceRateLimited(t *testing.T) {
	store := newFakeStore()
	store.pending = []repo.PendingOrder{
		{ID: 1, Number: "12345678903"},
		{ID: 2, Number: "9278923470"},
	}
	api := &fakeAPI{errs: map[string]error{
		"12345678903": ErrRateLimitedSentinel,
		"9278923470":  ErrRateLimitedSentinel,
	}}
	s := New(store, api, testOpts(), quietLog())
	s.SyncOnce(context.Background())

	assert.Empty(t, store.applied)

	s.SyncOnce(context.Background())
	assert.Empty(t, store.applied)
}

func TestSyncOnceAPIError(t *testing.T) {
	store := newFakeStore()
	store.pending = []repo.PendingOrder{{ID: 1, Number: "12345678903"}}
	api := &fakeAPI{errs: map[string]error{"12345678903": assert.AnError}}
	s := New(store, api, testOpts(), quietLog())
	s.SyncOnce(context.Background())
	assert.Empty(t, store.applied)
}

func TestSyncOnceStorageErrors(t *testing.T) {
	store := newFakeStore()
	store.pendingErr = errors.New("db down")
	s := New(store, &fakeAPI{}, testOpts(), quietLog())
	s.SyncOnce(context.Background())

	store = newFakeStore()
	store.pending = []repo.PendingOrder{{ID: 1, Number: "12345678903"}}
	store.applyErr = errors.New("db down")
	api := &fakeAPI{infos: map[string]models.AccrualInfo{
		"12345678903": {Status: models.AccrualProcessed},
	}}
	s = New(store, api, testOpts(), quietLog())
	s.SyncOnce(context.Background())
	assert.Empty(t, store.applied)
}

func TestSyncOnceEmptyPending(t *testing.T) {
	s := New(newFakeStore(), &fakeAPI{}, testOpts(), quietLog())
	s.SyncOnce(context.Background())
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New(newFakeStore(), &fakeAPI{}, testOpts(), quietLog())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestMapStatus(t *testing.T) {
	accrual := 10.0
	st, ac := mapStatus(models.AccrualInfo{Status: models.AccrualInvalid})
	require.Equal(t, models.StatusInvalid, st)
	assert.Nil(t, ac)

	st, ac = mapStatus(models.AccrualInfo{Status: models.AccrualProcessed, Accrual: &accrual})
	require.Equal(t, models.StatusProcessed, st)
	require.NotNil(t, ac)
	assert.Equal(t, 10.0, *ac)

	st, ac = mapStatus(models.AccrualInfo{Status: models.AccrualProcessing})
	require.Equal(t, models.StatusProcessing, st)
	assert.Nil(t, ac)

	st, ac = mapStatus(models.AccrualInfo{Status: models.AccrualRegistered})
	require.Equal(t, models.StatusNew, st)
	assert.Nil(t, ac)
}

func TestErrorMapper(t *testing.T) {
	innerErr := errors.New("inner: not registered")
	m := ErrorMapper{
		Inner:         &fakeAPI{errs: map[string]error{"1": innerErr}},
		NotRegistered: innerErr,
	}
	_, err := m.Accrual(context.Background(), "1")
	require.ErrorIs(t, err, ErrNotRegisteredSentinel)

	rateErr := errors.New("inner: rate limited")
	m = ErrorMapper{
		Inner:       &fakeAPI{errs: map[string]error{"1": rateErr}},
		RateLimited: rateErr,
	}
	_, err = m.Accrual(context.Background(), "1")
	require.ErrorIs(t, err, ErrRateLimitedSentinel)

	other := errors.New("boom")
	m = ErrorMapper{Inner: &fakeAPI{errs: map[string]error{"1": other}}}
	_, err = m.Accrual(context.Background(), "1")
	require.ErrorIs(t, err, other)

	m = ErrorMapper{Inner: &fakeAPI{infos: map[string]models.AccrualInfo{
		"1": {Status: models.AccrualProcessing},
	}}}
	info, err := m.Accrual(context.Background(), "1")
	require.NoError(t, err)
	assert.Equal(t, models.AccrualProcessing, info.Status)
}
