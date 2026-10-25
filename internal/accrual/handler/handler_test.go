package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/accrual/service"
	"gofermart/internal/models"
	"gofermart/internal/ratelimit"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	goodsErr   error
	orderErr   error
	order      repo.Order
	addedGoods []models.GoodsRule
	addedOrder string
}

func (f *fakeService) AddGoods(ctx context.Context, rule models.GoodsRule) error {
	if f.goodsErr != nil {
		return f.goodsErr
	}
	f.addedGoods = append(f.addedGoods, rule)
	return nil
}

func (f *fakeService) AddOrder(ctx context.Context, number string, items []models.OrderItem) error {
	if f.orderErr != nil {
		return f.orderErr
	}
	f.addedOrder = number
	return nil
}

func (f *fakeService) Order(ctx context.Context, number string) (repo.Order, error) {
	if f.orderErr != nil {
		return repo.Order{}, f.orderErr
	}
	return f.order, nil
}

func newTestRouter(svc Service, limiter *ratelimit.Limiter) http.Handler {
	h := New(svc, limiter, slog.New(slog.NewTextHandler(io_Discard{}, nil)))
	return h.Router()
}

type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

func route(t *testing.T, h http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getAccrual(t *testing.T, h http.Handler, number string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+number, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("number", number)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAddGoods(t *testing.T) {
	svc := &fakeService{}
	h := newTestRouter(svc, nil)

	body := []byte(`{"match":"Bork","reward":10,"reward_type":"%"}`)
	rec := route(t, h, http.MethodPost, "/api/goods", body)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, svc.addedGoods, 1)
	assert.Equal(t, "Bork", svc.addedGoods[0].Match)

	rec = route(t, h, http.MethodPost, "/api/goods", []byte(`{`))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.goodsErr = service.ErrInvalidGoods
	rec = route(t, h, http.MethodPost, "/api/goods", []byte(`{"match":"","reward":1,"reward_type":"pt"}`))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	svc.goodsErr = nil

	svc.goodsErr = repo.ErrDuplicate
	rec = route(t, h, http.MethodPost, "/api/goods", body)
	assert.Equal(t, http.StatusConflict, rec.Code)

	svc.goodsErr = assert.AnError
	rec = route(t, h, http.MethodPost, "/api/goods", body)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAddOrder(t *testing.T) {
	svc := &fakeService{}
	h := newTestRouter(svc, nil)

	body := []byte(`{"order":"12345678903","goods":[{"description":"x","price":1}]}`)
	rec := route(t, h, http.MethodPost, "/api/orders", body)
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "12345678903", svc.addedOrder)

	rec = route(t, h, http.MethodPost, "/api/orders", []byte(`{`))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.orderErr = service.ErrInvalidOrderNumber
	rec = route(t, h, http.MethodPost, "/api/orders", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.orderErr = repo.ErrDuplicate
	rec = route(t, h, http.MethodPost, "/api/orders", body)
	assert.Equal(t, http.StatusConflict, rec.Code)

	svc.orderErr = assert.AnError
	rec = route(t, h, http.MethodPost, "/api/orders", body)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetAccrual(t *testing.T) {
	svc := &fakeService{order: repo.Order{
		Number: "12345678903",
		Status: models.AccrualProcessed,
	}}
	h := newTestRouter(svc, ratelimit.New(100))

	rec := getAccrual(t, h, "12345678903")
	require.Equal(t, http.StatusOK, rec.Code)
	var info models.AccrualInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Equal(t, "12345678903", info.Order)
	assert.Equal(t, models.AccrualProcessed, info.Status)
	assert.Nil(t, info.Accrual)

	svc.orderErr = repo.ErrOrderNotFound
	rec = getAccrual(t, h, "12345678903")
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = getAccrual(t, h, "abc")
	assert.Equal(t, http.StatusNoContent, rec.Code)

	svc.orderErr = assert.AnError
	rec = getAccrual(t, h, "12345678903")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetAccrualRateLimited(t *testing.T) {
	svc := &fakeService{order: repo.Order{Number: "12345678903", Status: models.AccrualRegistered}}
	h := newTestRouter(svc, ratelimit.New(1))

	rec := getAccrual(t, h, "12345678903")
	require.Equal(t, http.StatusOK, rec.Code)

	rec = getAccrual(t, h, "12345678903")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "No more than 1 requests per minute allowed")
}
