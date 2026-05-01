package transaction

import (
	"context"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Transaction struct {
	ID            string    `db:"id"`
	SourceAccount int64     `db:"source_account"`
	DestAccount   int64     `db:"dest_account"`
	Amount        float64   `db:"amount"`
	Status        Status    `db:"status"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type Repository interface {
	Save(ctx context.Context, tx *Transaction) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
}
