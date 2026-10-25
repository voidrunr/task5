package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"gofermart/internal/auth"
	"gofermart/internal/loyalty/repo"
	"gofermart/internal/loyalty/service"
	"gofermart/internal/models"

	"github.com/go-chi/chi/v5"
)

type Service interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
	AddOrder(ctx context.Context, userID int64, number string) (bool, error)
	Orders(ctx context.Context, userID int64) ([]models.UserOrder, error)
	Balance(ctx context.Context, userID int64) (models.Balance, error)
	Withdraw(ctx context.Context, userID int64, number string, sum float64) error
	Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

const maxBodySize = 1 << 20

type Handler struct {
	svc Service
	mw  func(http.Handler) http.Handler
	log *slog.Logger
}

func New(svc Service, authMiddleware func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, mw: authMiddleware, log: log}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/api/user/register", h.Register)
	r.Post("/api/user/login", h.Login)
	r.Group(func(r chi.Router) {
		r.Use(h.mw)
		r.Post("/api/user/orders", h.AddOrder)
		r.Get("/api/user/orders", h.Orders)
		r.Get("/api/user/balance", h.Balance)
		r.Post("/api/user/balance/withdraw", h.Withdraw)
		r.Get("/api/user/withdrawals", h.Withdrawals)
	})
	return r
}

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var cred credentials
	if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if cred.Login == "" || cred.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token, err := h.svc.Register(r.Context(), cred.Login, cred.Password)
	if errors.Is(err, repo.ErrLoginTaken) {
		http.Error(w, "login already taken", http.StatusConflict)
		return
	}
	if err != nil {
		h.log.Error("register", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	auth.SetCookie(w, token)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var cred credentials
	if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if cred.Login == "" || cred.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token, err := h.svc.Login(r.Context(), cred.Login, cred.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err != nil {
		h.log.Error("login", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	auth.SetCookie(w, token)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) AddOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	body, err := readBody(r)
	if err != nil || len(body) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	exists, err := h.svc.AddOrder(r.Context(), userID, body)
	switch {
	case errors.Is(err, service.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	case errors.Is(err, repo.ErrOrderOwnedByOther):
		http.Error(w, "order already uploaded by another user", http.StatusConflict)
	case err != nil:
		h.log.Error("add order", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	case exists:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) Orders(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	orders, err := h.svc.Orders(r.Context(), userID)
	if err != nil {
		h.log.Error("orders", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	b, err := h.svc.Balance(r.Context(), userID)
	if err != nil {
		h.log.Error("balance", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := h.svc.Withdraw(r.Context(), userID, req.Order, req.Sum)
	switch {
	case errors.Is(err, service.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	case errors.Is(err, service.ErrInsufficientFunds):
		http.Error(w, "not enough funds", http.StatusPaymentRequired)
	case errors.Is(err, service.ErrInvalidAmount):
		http.Error(w, "bad request", http.StatusBadRequest)
	case err != nil:
		h.log.Error("withdraw", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out, err := h.svc.Withdrawals(r.Context(), userID)
	if err != nil {
		h.log.Error("withdrawals", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(out) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func readBody(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
