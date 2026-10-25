package accrualapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gofermart/internal/models"
)

var (
	ErrNotRegistered = errors.New("accrualapi: order not registered")
	ErrRateLimited   = errors.New("accrualapi: rate limited")
	ErrServer        = errors.New("accrualapi: accrual system server error")
)

const DefaultTimeout = 5 * time.Second

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: DefaultTimeout},
	}
}

func (c *Client) Accrual(ctx context.Context, number string) (models.AccrualInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/orders/"+number, nil)
	if err != nil {
		return models.AccrualInfo{}, fmt.Errorf("accrualapi: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return models.AccrualInfo{}, fmt.Errorf("accrualapi: do request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.AccrualInfo{}, fmt.Errorf("accrualapi: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var info models.AccrualInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return models.AccrualInfo{}, fmt.Errorf("accrualapi: decode body: %w", err)
		}
		info.Order = number
		return info, nil
	case http.StatusNoContent:
		return models.AccrualInfo{}, ErrNotRegistered
	case http.StatusTooManyRequests:
		return models.AccrualInfo{}, fmt.Errorf("%w: retry after %d",
			ErrRateLimited, int(RetryAfter(resp.Header.Get("Retry-After")).Seconds()))
	case http.StatusInternalServerError:
		return models.AccrualInfo{}, ErrServer
	default:
		return models.AccrualInfo{}, fmt.Errorf("accrualapi: unexpected status %d", resp.StatusCode)
	}
}

func RetryAfter(header string) time.Duration {
	sec, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || sec <= 0 {
		return time.Minute
	}
	return time.Duration(sec) * time.Second
}

func RetryAfterDuration(err error, defaultDur time.Duration) time.Duration {
	if err == nil {
		return defaultDur
	}
	msg := err.Error()
	idx := strings.LastIndex(msg, "retry after ")
	if idx < 0 {
		return defaultDur
	}
	return RetryAfter(msg[idx+len("retry after "):])
}
