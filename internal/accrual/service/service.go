package service

import (
	"context"
	"errors"
	"strings"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/luno"
	"gofermart/internal/models"
)

var (
	ErrInvalidOrderNumber = errors.New("service: invalid order number")
	ErrInvalidGoods       = errors.New("service: invalid goods")
)

type Storage interface {
	AddGoods(ctx context.Context, rule models.GoodsRule) error
	AddOrder(ctx context.Context, number string, items []models.OrderItem) error
	OrderByNumber(ctx context.Context, number string) (repo.Order, error)
	GoodsRules(ctx context.Context) ([]models.GoodsRule, error)
}

type Service struct {
	store Storage
}

func New(store Storage) *Service {
	return &Service{store: store}
}

func (s *Service) AddGoods(ctx context.Context, rule models.GoodsRule) error {
	if rule.Match == "" || !rule.RewardType.Valid() || rule.Reward < 0 {
		return ErrInvalidGoods
	}
	return s.store.AddGoods(ctx, rule)
}

func (s *Service) AddOrder(ctx context.Context, number string, items []models.OrderItem) error {
	if !luno.Valid(number) {
		return ErrInvalidOrderNumber
	}
	if len(items) == 0 {
		return ErrInvalidGoods
	}
	for _, it := range items {
		if it.Description == "" || it.Price < 0 {
			return ErrInvalidGoods
		}
	}
	return s.store.AddOrder(ctx, number, items)
}

func (s *Service) Order(ctx context.Context, number string) (repo.Order, error) {
	return s.store.OrderByNumber(ctx, number)
}

func ComputeAccrual(items []models.OrderItem, rules []models.GoodsRule) float64 {
	var total float64
	for _, item := range items {
		for _, rule := range rules {
			if !strings.Contains(item.Description, rule.Match) {
				continue
			}
			switch rule.RewardType {
			case models.RewardPercent:
				total += item.Price * rule.Reward / 100
			case models.RewardPoints:
				total += rule.Reward
			}
		}
	}
	return round2(total)
}

func round2(v float64) float64 {
	if v == 0 {
		return 0
	}
	scaled := v * 100
	if scaled > 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}
