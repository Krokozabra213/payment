package pgRepo

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

const (
	ctxTimeout = 4 * time.Second
)

type Repository struct {
	*pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		Pool: pool,
	}
}
