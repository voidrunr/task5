package repo

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsUniqueViolation(t *testing.T) {
	assert.True(t, isUniqueViolation(&pgconn.PgError{Code: uniqueViolation}))
	assert.False(t, isUniqueViolation(&pgconn.PgError{Code: "23503"}))
	assert.False(t, isUniqueViolation(errors.New("other")))
	assert.False(t, isUniqueViolation(nil))
	assert.False(t, isUniqueViolation(errors.Join(&pgconn.PgError{Code: "25006"})))
	assert.True(t, isUniqueViolation(errors.Join(errors.New("wrap"), &pgconn.PgError{Code: uniqueViolation})))
}
