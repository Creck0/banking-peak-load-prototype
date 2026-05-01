package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ahargunyllib/banking-peak-load-prototype/internal/domain/transaction"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/infrastructure/queue"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/logger"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrAccountNotFound = errors.New("account not found")

type CreateTransactionInput struct {
	SourceAccount int64
	DestAccount   int64
	Amount        float64
}

// TxMessage is the payload published to and consumed from the transactions queue.
type TxMessage struct {
	TXID          string  `json:"tx_id"`
	SourceAccount int64   `json:"source_account"`
	DestAccount   int64   `json:"dest_account"`
	Amount        float64 `json:"amount"`
}

type TransactionService interface {
	CreateTransaction(ctx context.Context, input CreateTransactionInput) (*transaction.Transaction, error)
	GetTransactionStatus(ctx context.Context, id string) (*transaction.Transaction, error)
}

type transactionService struct {
	repo  transaction.Repository
	db    *sqlx.DB      // nil → memory mode (no balance ops)
	queue *queue.Client // nil → sync path
	redis *redis.Client // nil → no cache invalidation
}

func NewTransactionService(
	repo transaction.Repository,
	db *sqlx.DB,
	q *queue.Client,
	rdb *redis.Client,
) TransactionService {
	return &transactionService{repo: repo, db: db, queue: q, redis: rdb}
}

func (s *transactionService) CreateTransaction(ctx context.Context, input CreateTransactionInput) (*transaction.Transaction, error) {
	logger.Set(ctx, "source_account", input.SourceAccount)
	logger.Set(ctx, "dest_account", input.DestAccount)
	logger.Set(ctx, "amount", input.Amount)

	now := time.Now()
	tx := &transaction.Transaction{
		ID:            fmt.Sprintf("tx_%d", now.UnixNano()),
		SourceAccount: input.SourceAccount,
		DestAccount:   input.DestAccount,
		Amount:        input.Amount,
		CreatedAt:     now,
	}

	var err error
	switch {
	case s.queue != nil:
		// Async path: save pending, publish to queue, worker handles debit/credit.
		err = s.createAsync(ctx, tx)
	case s.db != nil:
		// Sync path: atomic debit + credit + insert in one DB transaction.
		err = s.createSync(ctx, tx)
	default:
		// Memory/dev fallback: no balance ops.
		tx.Status = transaction.StatusCompleted
		err = s.repo.Save(ctx, tx)
	}

	if err != nil {
		logger.Set(ctx, "transaction_error", err.Error())
		return nil, err
	}

	logger.Set(ctx, "transaction_id", tx.ID)
	logger.Set(ctx, "transaction_status", tx.Status)
	return tx, nil
}

// createSync executes an atomic DB transaction: balance check → debit → credit → insert.
func (s *transactionService) createSync(ctx context.Context, tx *transaction.Transaction) error {
	dbTx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = dbTx.Rollback() }()

	// Lock source account row and read balance.
	var balance float64
	err = dbTx.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`,
		tx.SourceAccount).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("query source account: %w", err)
	}

	if balance < tx.Amount {
		return ErrInsufficientFunds
	}

	// Verify destination account exists.
	var exists bool
	err = dbTx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)`,
		tx.DestAccount).Scan(&exists)
	if err != nil {
		return fmt.Errorf("query dest account: %w", err)
	}
	if !exists {
		return ErrAccountNotFound
	}

	// Debit source.
	if _, err = dbTx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1, updated_at = NOW() WHERE id = $2`,
		tx.Amount, tx.SourceAccount); err != nil {
		return fmt.Errorf("debit source: %w", err)
	}

	// Credit destination.
	if _, err = dbTx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = NOW() WHERE id = $2`,
		tx.Amount, tx.DestAccount); err != nil {
		return fmt.Errorf("credit dest: %w", err)
	}

	// Insert transaction record.
	tx.Status = transaction.StatusCompleted
	if _, err = dbTx.ExecContext(ctx,
		`INSERT INTO transactions (id, source_account, dest_account, amount, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		tx.ID, tx.SourceAccount, tx.DestAccount, tx.Amount, string(tx.Status), tx.CreatedAt); err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	if err = dbTx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.invalidateBalanceCache(ctx, tx.SourceAccount, tx.DestAccount)
	return nil
}

// createAsync saves a pending transaction and enqueues it for worker processing.
func (s *transactionService) createAsync(ctx context.Context, tx *transaction.Transaction) error {
	tx.Status = transaction.StatusPending
	if err := s.repo.Save(ctx, tx); err != nil {
		return fmt.Errorf("save pending transaction: %w", err)
	}

	msg := TxMessage{
		TXID:          tx.ID,
		SourceAccount: tx.SourceAccount,
		DestAccount:   tx.DestAccount,
		Amount:        tx.Amount,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal queue message: %w", err)
	}

	if err = s.queue.Publish(ctx, "transactions", body); err != nil {
		// Best-effort: mark failed if we can't publish.
		_ = s.repo.UpdateStatus(ctx, tx.ID, transaction.StatusFailed)
		return fmt.Errorf("publish to queue: %w", err)
	}

	return nil
}

// invalidateBalanceCache removes cached balance entries for the given account IDs.
func (s *transactionService) invalidateBalanceCache(ctx context.Context, accountIDs ...int64) {
	if s.redis == nil {
		return
	}
	keys := make([]string, len(accountIDs))
	for i, id := range accountIDs {
		keys[i] = fmt.Sprintf("balance:%d", id)
	}
	s.redis.Del(ctx, keys...)
}

func (s *transactionService) GetTransactionStatus(ctx context.Context, id string) (*transaction.Transaction, error) {
	logger.Set(ctx, "transaction_id", id)

	tx, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.Set(ctx, "transaction_error", err.Error())
		return nil, err
	}

	logger.Set(ctx, "transaction_status", tx.Status)
	return tx, nil
}
