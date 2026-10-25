package service

import (
	"context"
	"testing"

	"gofermart/internal/auth"
	"gofermart/internal/loyalty/repo"
	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	users       map[string]repo.User
	nextID      int64
	orders      map[int64][]models.UserOrder
	owner       map[string]int64
	balances    map[int64]models.Balance
	withdrawals map[int64][]models.Withdrawal
	createErr   error
	userErr     error
	addOrderErr error
	withdrawErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:       map[string]repo.User{},
		orders:      map[int64][]models.UserOrder{},
		owner:       map[string]int64{},
		balances:    map[int64]models.Balance{},
		withdrawals: map[int64][]models.Withdrawal{},
	}
}

func (f *fakeStore) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	if _, ok := f.users[login]; ok {
		return 0, repo.ErrLoginTaken
	}
	f.nextID++
	f.users[login] = repo.User{ID: f.nextID, Login: login, PasswordHash: passwordHash}
	return f.nextID, nil
}

func (f *fakeStore) UserByLogin(ctx context.Context, login string) (repo.User, error) {
	if f.userErr != nil {
		return repo.User{}, f.userErr
	}
	u, ok := f.users[login]
	if !ok {
		return repo.User{}, repo.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeStore) AddOrder(ctx context.Context, userID int64, number string) (repo.CreateOrder, error) {
	if f.addOrderErr != nil {
		return repo.CreateOrder{}, f.addOrderErr
	}
	if owner, ok := f.owner[number]; ok {
		if owner != userID {
			return repo.CreateOrder{}, repo.ErrOrderOwnedByOther
		}
		return repo.CreateOrder{Exists: true}, nil
	}
	f.owner[number] = userID
	return repo.CreateOrder{ID: f.nextID + 1, UploadedAt: "now"}, nil
}

func (f *fakeStore) OrdersByUser(ctx context.Context, userID int64) ([]models.UserOrder, error) {
	return f.orders[userID], nil
}

func (f *fakeStore) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return f.balances[userID], nil
}

func (f *fakeStore) Withdraw(ctx context.Context, userID int64, number string, sum float64) error {
	if f.withdrawErr != nil {
		return f.withdrawErr
	}
	b := f.balances[userID]
	if b.Current < sum {
		return repo.ErrInsufficientFunds
	}
	b.Current -= sum
	b.Withdrawn += sum
	f.balances[userID] = b
	return nil
}

func (f *fakeStore) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return f.withdrawals[userID], nil
}

func newTestService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	return New(store, auth.NewManager("test-secret")), store
}

func TestRegisterLogin(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	token, err := svc.Register(ctx, "gopher", "pwd12345")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Contains(t, store.users, "gopher")

	assert.NotEqual(t, "pwd12345", store.users["gopher"].PasswordHash)

	_, err = svc.Register(ctx, "gopher", "pwd12345")
	require.ErrorIs(t, err, repo.ErrLoginTaken)

	token, err = svc.Login(ctx, "gopher", "pwd12345")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	_, err = svc.Login(ctx, "gopher", "wrong")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.Login(ctx, "nobody", "pwd12345")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestRegisterStorageError(t *testing.T) {
	svc, store := newTestService(t)
	store.createErr = repo.ErrLoginTaken
	_, err := svc.Register(context.Background(), "x", "y")
	require.ErrorIs(t, err, repo.ErrLoginTaken)

	store.createErr = assert.AnError
	_, err = svc.Register(context.Background(), "x", "y")
	require.Error(t, err)
}

func TestAddOrder(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	token, err := svc.Register(ctx, "u", "password1")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	_, err = svc.AddOrder(ctx, 1, "1234")
	require.ErrorIs(t, err, ErrInvalidOrderNumber)

	exists, err := svc.AddOrder(ctx, 1, "12345678903")
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = svc.AddOrder(ctx, 1, "12345678903")
	require.NoError(t, err)
	assert.True(t, exists)

	_, err = svc.AddOrder(ctx, 2, "12345678903")
	require.ErrorIs(t, err, repo.ErrOrderOwnedByOther)

	storeErrStore := newFakeStore()
	storeErrStore.addOrderErr = assert.AnError
	svc2 := New(storeErrStore, auth.NewManager("s"))
	_, err = svc2.AddOrder(ctx, 1, "12345678903")
	require.Error(t, err)
}

func TestBalanceAndWithdraw(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	store.balances[1] = models.Balance{Current: 500.5, Withdrawn: 42}

	b, err := svc.Balance(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 500.5, b.Current)
	assert.Equal(t, 42.0, b.Withdrawn)

	err = svc.Withdraw(ctx, 1, "1234", 10)
	require.ErrorIs(t, err, ErrInvalidOrderNumber)

	err = svc.Withdraw(ctx, 1, "12345678903", 0)
	require.ErrorIs(t, err, ErrInvalidAmount)
	err = svc.Withdraw(ctx, 1, "12345678903", -5)
	require.ErrorIs(t, err, ErrInvalidAmount)

	err = svc.Withdraw(ctx, 1, "12345678903", 500.5)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, store.balances[1].Current, 0.001)
	assert.InDelta(t, 42.0+500.5, store.balances[1].Withdrawn, 0.001)

	err = svc.Withdraw(ctx, 1, "12345678903", 1e9)
	require.ErrorIs(t, err, ErrInsufficientFunds)

	store.withdrawErr = assert.AnError
	err = svc.Withdraw(ctx, 1, "12345678903", 1)
	require.Error(t, err)
}

func TestOrdersList(t *testing.T) {
	svc, store := newTestService(t)
	store.orders[1] = []models.UserOrder{
		{Number: "12345678903", Status: models.StatusNew},
	}
	orders, err := svc.Orders(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, "12345678903", orders[0].Number)
}

func TestWithdrawalsList(t *testing.T) {
	svc, store := newTestService(t)
	store.withdrawals[1] = []models.Withdrawal{{Order: "12345678903", Sum: 500}}
	out, err := svc.Withdrawals(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, 500.0, out[0].Sum)
}
