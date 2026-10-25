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
	ErrDuplicate     = errors.New("repo: duplicate key")
	ErrOrderNotFound = errors.New("repo: order not found")
)

const uniqueViolation = "23505"

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

type Order struct {
	ID      int64
	Number  string
	Status  models.AccrualStatus
	Accrual *float64
	Items   []models.OrderItem
}

func (r *Repo) AddGoods(ctx context.Context, rule models.GoodsRule) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO goods (match, reward, reward_type) VALUES ($1, $2, $3)`,
		rule.Match, rule.Reward, string(rule.RewardType),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("repo: insert goods: %w", err)
	}
	return nil
}

func (r *Repo) AddOrder(ctx context.Context, number string, items []models.OrderItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: begin add order: %w", err)
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO accrual_orders (number, status) VALUES ($1, 'REGISTERED')
		 RETURNING id`, number,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("repo: insert order: %w", err)
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO accrual_order_items (order_id, description, price) VALUES ($1, $2, $3)`,
			id, item.Description, item.Price,
		); err != nil {
			return fmt.Errorf("repo: insert order item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repo: commit add order: %w", err)
	}
	return nil
}

func (r *Repo) OrderByNumber(ctx context.Context, number string) (Order, error) {
	var o Order
	err := r.pool.QueryRow(ctx,
		`SELECT id, number, status, accrual FROM accrual_orders WHERE number = $1`,
		number,
	).Scan(&o.ID, &o.Number, &o.Status, &o.Accrual)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("repo: select order: %w", err)
	}
	items, err := r.orderItems(ctx, o.ID)
	if err != nil {
		return Order{}, err
	}
	o.Items = items
	return o, nil
}

func (r *Repo) orderItems(ctx context.Context, orderID int64) ([]models.OrderItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT description, price FROM accrual_order_items WHERE order_id = $1 ORDER BY id`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: select order items: %w", err)
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var it models.OrderItem
		if err := rows.Scan(&it.Description, &it.Price); err != nil {
			return nil, fmt.Errorf("repo: scan order item: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: iterate order items: %w", err)
	}
	return items, nil
}

func (r *Repo) GoodsRules(ctx context.Context) ([]models.GoodsRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT match, reward, reward_type FROM goods ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: select goods: %w", err)
	}
	defer rows.Close()

	var rules []models.GoodsRule
	for rows.Next() {
		var g models.GoodsRule
		var rewardType string
		if err := rows.Scan(&g.Match, &g.Reward, &rewardType); err != nil {
			return nil, fmt.Errorf("repo: scan goods: %w", err)
		}
		g.RewardType = models.RewardType(rewardType)
		rules = append(rules, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: iterate goods: %w", err)
	}
	return rules, nil
}

func (r *Repo) TakeOrderForProcessing(ctx context.Context) (Order, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, false, fmt.Errorf("repo: begin take order: %w", err)
	}
	defer tx.Rollback(ctx)

	var o Order
	err = tx.QueryRow(ctx,
		`UPDATE accrual_orders SET status = 'PROCESSING'
		 WHERE id = (
		     SELECT id FROM accrual_orders
		     WHERE status IN ('REGISTERED', 'PROCESSING')
		     ORDER BY id
		     FOR UPDATE SKIP LOCKED
		     LIMIT 1
		 )
		 RETURNING id, number, status`,
	).Scan(&o.ID, &o.Number, &o.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("repo: take order: %w", err)
	}
	items, err := r.orderItems(ctx, o.ID)
	if err != nil {
		return Order{}, false, err
	}
	o.Items = items
	if err := tx.Commit(ctx); err != nil {
		return Order{}, false, fmt.Errorf("repo: commit take order: %w", err)
	}
	return o, true, nil
}

func (r *Repo) FinishOrder(ctx context.Context, orderID int64, status models.AccrualStatus, accrual *float64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE accrual_orders
		 SET status = $2, accrual = $3, processed_at = now()
		 WHERE id = $1 AND status NOT IN ('PROCESSED', 'INVALID')`,
		orderID, status, accrual,
	)
	if err != nil {
		return fmt.Errorf("repo: finish order: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
