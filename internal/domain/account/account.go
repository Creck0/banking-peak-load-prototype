package account

import (
	"context"
	"time"
)

type Account struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Balance   float64   `db:"balance"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Repository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}
