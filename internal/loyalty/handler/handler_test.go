package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gofermart/internal/auth"
	"gofermart/internal/loyalty/repo"
	"gofermart/internal/loyalty/service"
	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeService struct {
	registerToken  string
	registerErr    error
	loginToken     string
	loginErr       error
	addExists      bool
	addErr         error
	orders         []models.UserOrder
	ordersErr      error
	balance        models.Balance
	balanceErr     error
	withdrawErr    error
	withdrawals    []models.Withdrawal
	withdrawalsErr error
	lastOrder      string
	lastSum        float64
}

func (f *fakeService) Register(ctx context.Context, login, password string) (string, error) {
	return f.registerToken, f.registerErr
}

func (f *fakeService) Login(ctx context.Context, login, password string) (string, error) {
	return f.loginToken, f.loginErr
}

func (f *fakeService) AddOrder(ctx context.Context, userID int64, number string) (bool, error) {
	f.lastOrder = number
	return f.addExists, f.addErr
}

func (f *fakeService) Orders(ctx context.Context, userID int64) ([]models.UserOrder, error) {
	return f.orders, f.ordersErr
}

func (f *fakeService) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return f.balance, f.balanceErr
}

func (f *fakeService) Withdraw(ctx context.Context, userID int64, number string, sum float64) error {
	f.lastOrder = number
	f.lastSum = sum
	return f.withdrawErr
}

func (f *fakeService) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return f.withdrawals, f.withdrawalsErr
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io_Discard{}, nil))
}

type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

func newTestRouter(svc Service) (http.Handler, *auth.Manager) {
	m := auth.NewManager("test-secret")
	h := New(svc, m.Middleware, quietLog())
	return h.Router(), m
}

func authedRequest(t *testing.T, m *auth.Manager, method, path string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	token, err := m.IssueToken(1, "tester")
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterHandler(t *testing.T) {
	svc := &fakeService{registerToken: "tok"}
	h, _ := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodPost, "/api/user/register",
		strings.NewReader(`{"login":"gopher","password":"secret"}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, auth.CookieName, cookies[0].Name)

	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/register",
		strings.NewReader(`{"login":"","password":""}`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.registerErr = repo.ErrLoginTaken
	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/register",
		strings.NewReader(`{"login":"gopher","password":"secret"}`)))
	assert.Equal(t, http.StatusConflict, rec.Code)

	svc.registerErr = assert.AnError
	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/register",
		strings.NewReader(`{"login":"gopher","password":"secret"}`)))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLoginHandler(t *testing.T) {
	svc := &fakeService{loginToken: "tok"}
	h, _ := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodPost, "/api/user/login",
		strings.NewReader(`{"login":"gopher","password":"secret"}`)))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/login",
		strings.NewReader(`{"login":"","password":""}`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.loginErr = service.ErrInvalidCredentials
	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/login",
		strings.NewReader(`{"login":"gopher","password":"bad"}`)))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	svc.loginErr = assert.AnError
	rec = do(h, httptest.NewRequest(http.MethodPost, "/api/user/login",
		strings.NewReader(`{"login":"gopher","password":"secret"}`)))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAddOrderHandler(t *testing.T) {
	svc := &fakeService{}
	h, m := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodPost, "/api/user/orders",
		strings.NewReader("12345678903")))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders",
		[]byte("12345678903")))
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "12345678903", svc.lastOrder)

	svc.addExists = true
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders",
		[]byte("12345678903")))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.addErr = service.ErrInvalidOrderNumber
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders",
		[]byte("1234")))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	svc.addErr = repo.ErrOrderOwnedByOther
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders",
		[]byte("12345678903")))
	assert.Equal(t, http.StatusConflict, rec.Code)

	svc.addErr = assert.AnError
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/orders",
		[]byte("12345678903")))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOrdersHandler(t *testing.T) {
	svc := &fakeService{}
	h, m := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodGet, "/api/user/orders", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/orders", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)

	accrual := 500.0
	svc.orders = []models.UserOrder{
		{Number: "12345678903", Status: models.StatusProcessed, Accrual: &accrual},
		{Number: "9278923470", Status: models.StatusNew},
	}
	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/orders", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var out []models.UserOrder
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 2)
	assert.Equal(t, "12345678903", out[0].Number)
	assert.NotNil(t, out[0].Accrual)
	assert.Nil(t, out[1].Accrual)

	assert.Contains(t, rec.Body.String(), "uploaded_at")

	svc.ordersErr = assert.AnError
	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/orders", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestBalanceHandler(t *testing.T) {
	svc := &fakeService{balance: models.Balance{Current: 500.5, Withdrawn: 42}}
	h, m := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodGet, "/api/user/balance", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/balance", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var b models.Balance
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, 500.5, b.Current)
	assert.Equal(t, 42.0, b.Withdrawn)

	svc.balanceErr = assert.AnError
	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/balance", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWithdrawHandler(t *testing.T) {
	svc := &fakeService{}
	h, m := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw",
		strings.NewReader(`{"order":"12345678903","sum":10}`)))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{"order":"12345678903","sum":751}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "12345678903", svc.lastOrder)
	assert.Equal(t, 751.0, svc.lastSum)

	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.withdrawErr = service.ErrInvalidOrderNumber
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{"order":"1234","sum":10}`)))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	svc.withdrawErr = service.ErrInsufficientFunds
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{"order":"12345678903","sum":10}`)))
	assert.Equal(t, http.StatusPaymentRequired, rec.Code)

	svc.withdrawErr = service.ErrInvalidAmount
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{"order":"12345678903","sum":-1}`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	svc.withdrawErr = assert.AnError
	rec = do(h, authedRequest(t, m, http.MethodPost, "/api/user/balance/withdraw",
		[]byte(`{"order":"12345678903","sum":10}`)))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWithdrawalsHandler(t *testing.T) {
	svc := &fakeService{}
	h, m := newTestRouter(svc)

	rec := do(h, httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/withdrawals", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)

	svc.withdrawals = []models.Withdrawal{{Order: "2377225624", Sum: 500}}
	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/withdrawals", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var out []models.Withdrawal
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out, 1)
	assert.Equal(t, "2377225624", out[0].Order)

	svc.withdrawalsErr = assert.AnError
	rec = do(h, authedRequest(t, m, http.MethodGet, "/api/user/withdrawals", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
