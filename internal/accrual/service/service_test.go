package service

import (
	"context"
	"errors"
	"testing"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStorage struct {
	rules       []models.GoodsRule
	addGoodsErr error
	addOrderErr error
	order       repo.Order
	orderErr    error
	addedOrders map[string][]models.OrderItem
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{addedOrders: map[string][]models.OrderItem{}}
}

func (f *fakeStorage) AddGoods(ctx context.Context, rule models.GoodsRule) error {
	if f.addGoodsErr != nil {
		return f.addGoodsErr
	}
	f.rules = append(f.rules, rule)
	return nil
}

func (f *fakeStorage) AddOrder(ctx context.Context, number string, items []models.OrderItem) error {
	if f.addOrderErr != nil {
		return f.addOrderErr
	}
	f.addedOrders[number] = items
	return nil
}

func (f *fakeStorage) OrderByNumber(ctx context.Context, number string) (repo.Order, error) {
	if f.orderErr != nil {
		return repo.Order{}, f.orderErr
	}
	return f.order, nil
}

func (f *fakeStorage) GoodsRules(ctx context.Context) ([]models.GoodsRule, error) {
	return f.rules, nil
}

func TestAddGoods(t *testing.T) {
	s := New(newFakeStorage())

	require.NoError(t, s.AddGoods(context.Background(), models.GoodsRule{
		Match: "Bork", Reward: 10, RewardType: models.RewardPercent,
	}))
	require.ErrorIs(t, s.AddGoods(context.Background(), models.GoodsRule{
		Match: "", Reward: 10, RewardType: models.RewardPercent,
	}), ErrInvalidGoods)
	require.ErrorIs(t, s.AddGoods(context.Background(), models.GoodsRule{
		Match: "x", Reward: -1, RewardType: models.RewardPercent,
	}), ErrInvalidGoods)
	require.ErrorIs(t, s.AddGoods(context.Background(), models.GoodsRule{
		Match: "x", Reward: 1, RewardType: "zzz",
	}), ErrInvalidGoods)
}

func TestAddOrderValidation(t *testing.T) {
	s := New(newFakeStorage())
	ctx := context.Background()

	require.ErrorIs(t, s.AddOrder(ctx, "1234", []models.OrderItem{{Description: "x", Price: 1}}), ErrInvalidOrderNumber)

	require.ErrorIs(t, s.AddOrder(ctx, "12345678903", nil), ErrInvalidGoods)

	require.ErrorIs(t, s.AddOrder(ctx, "12345678903",
		[]models.OrderItem{{Description: "", Price: 1}}), ErrInvalidGoods)

	require.ErrorIs(t, s.AddOrder(ctx, "12345678903",
		[]models.OrderItem{{Description: "x", Price: -1}}), ErrInvalidGoods)

	require.NoError(t, s.AddOrder(ctx, "12345678903",
		[]models.OrderItem{{Description: "Чайник Bork", Price: 7000}}))
}

func TestAddOrderDuplicate(t *testing.T) {
	f := newFakeStorage()
	f.addOrderErr = repo.ErrDuplicate
	s := New(f)
	err := s.AddOrder(context.Background(), "12345678903",
		[]models.OrderItem{{Description: "x", Price: 1}})
	require.ErrorIs(t, err, repo.ErrDuplicate)
}

func TestOrder(t *testing.T) {
	f := newFakeStorage()
	f.order = repo.Order{ID: 1, Number: "12345678903", Status: models.AccrualProcessed}
	s := New(f)
	o, err := s.Order(context.Background(), "12345678903")
	require.NoError(t, err)
	assert.Equal(t, "12345678903", o.Number)

	f.orderErr = repo.ErrOrderNotFound
	_, err = s.Order(context.Background(), "1")
	require.ErrorIs(t, err, repo.ErrOrderNotFound)
}

func TestComputeAccrual(t *testing.T) {
	rules := []models.GoodsRule{
		{Match: "Bork", Reward: 10, RewardType: models.RewardPercent},
		{Match: "Чайник", Reward: 50, RewardType: models.RewardPoints},
	}
	tests := []struct {
		name  string
		items []models.OrderItem
		want  float64
	}{
		{
			name:  "percent only",
			items: []models.OrderItem{{Description: "Чайник Bork", Price: 7000}},
			want:  700 + 50,
		},
		{
			name:  "no match",
			items: []models.OrderItem{{Description: "Обычный товар", Price: 1000}},
			want:  0,
		},
		{
			name: "multi item sum",
			items: []models.OrderItem{
				{Description: "Чайник Bork", Price: 7000},
				{Description: "Ковёр Bork", Price: 2000},
			},
			want: 700 + 50 + 200,
		},
		{
			name:  "fractional percent rounds to 2 decimals",
			items: []models.OrderItem{{Description: "Bork", Price: 199.99}},
			want:  20,
		},
		{
			name:  "negative-free fractional down",
			items: []models.OrderItem{{Description: "Bork", Price: 10.01}},
			want:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.want, ComputeAccrual(tt.items, rules), 0.001)
		})
	}
}

func TestComputeAccrualEmpty(t *testing.T) {
	if got := ComputeAccrual(nil, nil); got != 0 {
		t.Errorf("empty = %v", got)
	}
	if got := ComputeAccrual([]models.OrderItem{{Description: "x", Price: 1}}, nil); got != 0 {
		t.Errorf("no rules = %v", got)
	}
}

func TestRound2(t *testing.T) {
	assert.Equal(t, 0.0, round2(0))
	assert.Equal(t, 5.01, round2(5.005))
	assert.Equal(t, 5.0, round2(4.995))
	assert.Equal(t, -5.01, round2(-5.005))
}

func TestAddGoodsStorageError(t *testing.T) {
	f := newFakeStorage()
	f.addGoodsErr = errors.New("db down")
	s := New(f)
	require.Error(t, s.AddGoods(context.Background(), models.GoodsRule{
		Match: "x", Reward: 1, RewardType: models.RewardPoints,
	}))
}
