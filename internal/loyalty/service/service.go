package service

import (
	"context"
	"errors"
	"fmt"

	"gofermart/internal/auth"
	"gofermart/internal/loyalty/repo"
	"gofermart/internal/luno"
	"gofermart/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("service: invalid login or password")
	ErrInvalidOrderNumber = errors.New("service: invalid order number")
	ErrInsufficientFunds  = errors.New("service: insufficient funds")
	ErrInvalidAmount      = errors.New("service: invalid amount")
)

const maxOrderNumberLength = 64

type Storage interface {
	CreateUser(ctx context.Context, login, passwordHash string) (int64, error)
	UserByLogin(ctx context.Context, login string) (repo.User, error)
	AddOrder(ctx context.Context, userID int64, number string) (repo.CreateOrder, error)
	OrdersByUser(ctx context.Context, userID int64) ([]models.UserOrder, error)
	Balance(ctx context.Context, userID int64) (models.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

type Service struct {
	store Storage
	auth  *auth.Manager
}

func New(store Storage, manager *auth.Manager) *Service {
	return &Service{store: store, auth: manager}
}

func (s *Service) Register(ctx context.Context, login, password string) (string, error) {
	hash, err := s.auth.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("service: register: %w", err)
	}
	id, err := s.store.CreateUser(ctx, login, hash)
	if err != nil {
		return "", err
	}
	return s.auth.IssueToken(id, login)
}

func (s *Service) Login(ctx context.Context, login, password string) (string, error) {
	user, err := s.store.UserByLogin(ctx, login)
	if errors.Is(err, repo.ErrUserNotFound) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("service: login: %w", err)
	}
	if !s.auth.CheckPassword(user.PasswordHash, password) {
		return "", ErrInvalidCredentials
	}
	return s.auth.IssueToken(user.ID, user.Login)
}

func (s *Service) AddOrder(ctx context.Context, userID int64, number string) (exists bool, err error) {
	if !luno.Valid(number) || len(number) > maxOrderNumberLength {
		return false, ErrInvalidOrderNumber
	}
	res, err := s.store.AddOrder(ctx, userID, number)
	if errors.Is(err, repo.ErrOrderOwnedByOther) {
		return false, err
	}
	if err != nil {
		return false, fmt.Errorf("service: add order: %w", err)
	}
	return res.Exists, nil
}

func (s *Service) Orders(ctx context.Context, userID int64) ([]models.UserOrder, error) {
	orders, err := s.store.OrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service: orders: %w", err)
	}
	return orders, nil
}

func (s *Service) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	b, err := s.store.Balance(ctx, userID)
	if err != nil {
		return models.Balance{}, fmt.Errorf("service: balance: %w", err)
	}
	return b, nil
}

func (s *Service) Withdraw(ctx context.Context, userID int64, number string, sum float64) error {
	if !luno.Valid(number) {
		return ErrInvalidOrderNumber
	}
	if sum <= 0 {
		return ErrInvalidAmount
	}
	err := s.store.Withdraw(ctx, userID, number, sum)
	if errors.Is(err, repo.ErrInsufficientFunds) {
		return ErrInsufficientFunds
	}
	if err != nil {
		return fmt.Errorf("service: withdraw: %w", err)
	}
	return nil
}

func (s *Service) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	out, err := s.store.Withdrawals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service: withdrawals: %w", err)
	}
	return out, nil
}
