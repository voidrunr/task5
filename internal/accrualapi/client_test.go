package accrualapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gofermart/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccrualProcessed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/orders/12345678903", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"12345678903","status":"PROCESSED","accrual":500}`))
	}))
	defer srv.Close()

	c := New(srv.URL + "/")
	info, err := c.Accrual(context.Background(), "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.AccrualProcessed, info.Status)
	require.NotNil(t, info.Accrual)
	assert.InDelta(t, 500.0, *info.Accrual, 0.001)
	assert.Equal(t, "12345678903", info.Order)
}

func TestAccrualWithoutAccrualField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"order":"1","status":"PROCESSING"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	info, err := c.Accrual(context.Background(), "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.AccrualProcessing, info.Status)
	assert.Nil(t, info.Accrual)
}

func TestAccrualNotRegistered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Accrual(context.Background(), "12345678903")
	require.ErrorIs(t, err, ErrNotRegistered)
}

func TestAccrualRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("No more than N requests per minute allowed"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Accrual(context.Background(), "12345678903")
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, 30*time.Second, RetryAfterDuration(err, time.Minute))
}

func TestAccrualServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Accrual(context.Background(), "12345678903")
	require.ErrorIs(t, err, ErrServer)
}

func TestAccrualUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Accrual(context.Background(), "12345678903")
	require.Error(t, err)
}

func TestAccrualBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Accrual(context.Background(), "12345678903")
	require.Error(t, err)
}

func TestAccrualUnreachable(t *testing.T) {
	c := New("http://127.0.0.1:1")
	_, err := c.Accrual(context.Background(), "12345678903")
	require.Error(t, err)
}

func TestRetryAfter(t *testing.T) {
	assert.Equal(t, time.Minute, RetryAfter(""))
	assert.Equal(t, time.Minute, RetryAfter("junk"))
	assert.Equal(t, time.Minute, RetryAfter("0"))
	assert.Equal(t, 42*time.Second, RetryAfter("42"))
	assert.Equal(t, time.Minute, RetryAfterDuration(nil, time.Minute))
	assert.Equal(t, time.Minute, RetryAfterDuration(ErrServer, time.Minute))
}
