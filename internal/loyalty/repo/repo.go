package repo

import (
	"context"
	"errors"
	"fmt"

	"gofermart/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLoginTaken        = errors.New("repo: login already taken")
	ErrUserNotFound      = errors.New("repo: user not found")
	ErrOrderOwnedByOther = errors.New("repo: order owned by another user")
	ErrInsufficientFunds = errors.New("repo: insufficient funds")
)

const uniqueViolation = "23505"

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

type PendingOrder struct {
	ID     int64
	UserID int64
	Number string
	Status models.OrderStatus
}

type CreateOrder struct {
	ID         int64
	UploadedAt string
	Exists     bool
}

func (r *Repo) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("repo: begin create user: %w", err)
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrLoginTaken
		}
		return 0, fmt.Errorf("repo: insert user: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_balances (user_id) VALUES ($1)`, id,
	); err != nil {
		return 0, fmt.Errorf("repo: create balance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("repo: commit create user: %w", err)
	}
	return id, nil
}

func (r *Repo) UserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash FROM users WHERE login = $1`, login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("repo: select user by login: %w", err)
	}
	return u, nil
}

func (r *Repo) AddOrder(ctx context.Context, userID int64, number string) (CreateOrder, error) {
	var res CreateOrder
	err := r.pool.QueryRow(ctx,
		`INSERT INTO orders (user_id, number, status)
		 VALUES ($1, $2, 'NEW')
		 ON CONFLICT (number) DO NOTHING
		 RETURNING id, uploaded_at::text`,
		userID, number,
	).Scan(&res.ID, &res.UploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var owner int64
		if err := r.pool.QueryRow(ctx,
			`SELECT user_id FROM orders WHERE number = $1`, number,
		).Scan(&owner); err != nil {
			return CreateOrder{}, fmt.Errorf("repo: select order owner: %w", err)
		}
		if owner != userID {
			return CreateOrder{}, ErrOrderOwnedByOther
		}
		return CreateOrder{Exists: true}, nil
	}
	if err != nil {
		return CreateOrder{}, fmt.Errorf("repo: insert order: %w", err)
	}
	return res, nil
}

func (r *Repo) OrdersByUser(ctx context.Context, userID int64) ([]models.UserOrder, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT number, status, accrual, uploaded_at
		 FROM orders WHERE user_id = $1 ORDER BY uploaded_at ASC, id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: select orders: %w", err)
	}
	defer rows.Close()

	var orders []models.UserOrder
	for rows.Next() {
		var o models.UserOrder
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, fmt.Errorf("repo: scan order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: iterate orders: %w", err)
	}
	return orders, nil
}

func (r *Repo) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	var b models.Balance
	err := r.pool.QueryRow(ctx,
		`SELECT current, withdrawn FROM user_balances WHERE user_id = $1`, userID,
	).Scan(&b.Current, &b.Withdrawn)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Balance{}, nil
	}
	if err != nil {
		return models.Balance{}, fmt.Errorf("repo: select balance: %w", err)
	}
	return b, nil
}

func (r *Repo) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: begin withdraw: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE user_balances
		 SET current = current - $2, withdrawn = withdrawn + $2
		 WHERE user_id = $1 AND current >= $2`,
		userID, sum,
	)
	if err != nil {
		return fmt.Errorf("repo: update balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientFunds
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`,
		userID, order, sum,
	); err != nil {
		return fmt.Errorf("repo: insert withdrawal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repo: commit withdraw: %w", err)
	}
	return nil
}

func (r *Repo) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT order_number, sum, processed_at
		 FROM withdrawals WHERE user_id = $1 ORDER BY processed_at ASC, id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: select withdrawals: %w", err)
	}
	defer rows.Close()

	var out []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("repo: scan withdrawal: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: iterate withdrawals: %w", err)
	}
	return out, nil
}

func (r *Repo) PendingOrders(ctx context.Context, limit int) ([]PendingOrder, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, number, status
		 FROM orders
		 WHERE status IN ('NEW', 'PROCESSING')
		 ORDER BY id ASC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: select pending orders: %w", err)
	}
	defer rows.Close()

	var out []PendingOrder
	for rows.Next() {
		var o PendingOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.Number, &o.Status); err != nil {
			return nil, fmt.Errorf("repo: scan pending order: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: iterate pending orders: %w", err)
	}
	return out, nil
}

func (r *Repo) ApplyAccrual(ctx context.Context, orderID int64, status models.OrderStatus, accrual *float64) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("repo: begin apply accrual: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID int64
	err = tx.QueryRow(ctx,
		`UPDATE orders
		 SET status = $2, accrual = $3
		 WHERE id = $1 AND status IN ('NEW', 'PROCESSING')
		 RETURNING user_id`,
		orderID, status, accrual,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("repo: update order status: %w", err)
	}

	if status == models.StatusProcessed && accrual != nil && *accrual > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE user_balances SET current = current + $2 WHERE user_id = $1`,
			userID, *accrual,
		); err != nil {
			return false, fmt.Errorf("repo: credit balance: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("repo: commit apply accrual: %w", err)
	}
	return true, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
