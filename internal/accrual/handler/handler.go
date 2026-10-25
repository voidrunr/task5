package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"gofermart/internal/accrual/repo"
	"gofermart/internal/accrual/service"
	"gofermart/internal/luno"
	"gofermart/internal/models"
	"gofermart/internal/ratelimit"

	"github.com/go-chi/chi/v5"
)

type Service interface {
	AddGoods(ctx context.Context, rule models.GoodsRule) error

	AddOrder(ctx context.Context, number string, items []models.OrderItem) error

	Order(ctx context.Context, number string) (repo.Order, error)
}

const maxBodySize = 1 << 20

type Handler struct {
	svc     Service
	limiter *ratelimit.Limiter
	log     *slog.Logger
}

func New(svc Service, limiter *ratelimit.Limiter, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	if limiter == nil {
		limiter = ratelimit.New(60)
	}
	return &Handler{svc: svc, limiter: limiter, log: log}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/api/goods", h.AddGoods)
	r.Post("/api/orders", h.AddOrder)
	r.Get("/api/orders/{number}", h.GetAccrual)
	return r
}

type goodsRequest struct {
	Match      string  `json:"match"`
	Reward     float64 `json:"reward"`
	RewardType string  `json:"reward_type"`
}

func (h *Handler) AddGoods(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var req goodsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := h.svc.AddGoods(r.Context(), models.GoodsRule{
		Match:      req.Match,
		Reward:     req.Reward,
		RewardType: models.RewardType(req.RewardType),
	})
	switch {
	case errors.Is(err, service.ErrInvalidGoods):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, repo.ErrDuplicate):
		http.Error(w, "match already registered", http.StatusConflict)
	case err != nil:
		h.log.Error("add goods", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

type orderRequest struct {
	Order string             `json:"order"`
	Goods []models.OrderItem `json:"goods"`
}

func (h *Handler) AddOrder(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var req orderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := h.svc.AddOrder(r.Context(), req.Order, req.Goods)
	switch {
	case errors.Is(err, service.ErrInvalidOrderNumber), errors.Is(err, service.ErrInvalidGoods):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, repo.ErrDuplicate):
		http.Error(w, "order already accepted", http.StatusConflict)
	case err != nil:
		h.log.Error("add order", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) GetAccrual(w http.ResponseWriter, r *http.Request) {
	number := chi.URLParam(r, "number")
	if !luno.Valid(number) {
		w.WriteHeader(http.StatusNoContent)

		return
	}

	ok, retryAfter := h.limiter.Allow()
	if !ok {
		secs := int(math.Ceil(retryAfter.Seconds()))
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "No more than %d requests per minute allowed\n", h.limiter.Limit())
		return
	}

	order, err := h.svc.Order(r.Context(), number)
	if errors.Is(err, repo.ErrOrderNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.log.Error("get accrual", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, models.AccrualInfo{
		Order:   order.Number,
		Status:  order.Status,
		Accrual: order.Accrual,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
