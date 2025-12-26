package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bank-prototype/internal/models"
	"bank-prototype/internal/utils"
)

var (
	ErrTransactionFailed = errors.New("транзакция не выполнена")
)

type TransactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) ExecuteTransfer(
	ctx context.Context,
	fromAccountID, toAccountID string,
	amount, feeAmount float64,
	feePercent int,
	txType string,
) (*models.Transaction, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	totalDebit := amount + feeAmount

	var fromBalance float64
	err = tx.QueryRow(ctx,
		"SELECT balance FROM accounts WHERE id = $1 AND status = 'active' FOR UPDATE",
		fromAccountID,
	).Scan(&fromBalance)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("ошибка получения баланса отправителя: %w", err)
	}

	if fromBalance < totalDebit {
		return nil, ErrInsufficientBalance
	}

	var toStatus string
	err = tx.QueryRow(ctx,
		"SELECT status FROM accounts WHERE id = $1 FOR UPDATE",
		toAccountID,
	).Scan(&toStatus)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("ошибка проверки счёта получателя: %w", err)
	}

	if toStatus != "active" {
		return nil, ErrAccountClosed
	}

	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance - $1 WHERE id = $2",
		totalDebit, fromAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка списания со счёта отправителя: %w", err)
	}

	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance + $1 WHERE id = $2",
		amount, toAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка зачисления на счёт получателя: %w", err)
	}

	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance + $1 WHERE id = $2",
		feeAmount, SystemBankAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка начисления комиссии: %w", err)
	}

	transactionID := uuid.New().String()
	query := `
		INSERT INTO transactions (
			id, type, from_account_id, to_account_id,
			amount, fee_percent, fee_amount, total_debit,
			fee_account_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'completed', NOW())
		RETURNING id, type, from_account_id, to_account_id, amount,
		          fee_percent, fee_amount, total_debit, fee_account_id,
		          status, created_at
	`

	var transaction models.Transaction
	err = tx.QueryRow(ctx, query,
		transactionID, txType, fromAccountID, toAccountID,
		amount, feePercent, feeAmount, totalDebit,
		SystemBankAccountID,
	).Scan(
		&transaction.ID,
		&transaction.Type,
		&transaction.FromAccountID,
		&transaction.ToAccountID,
		&transaction.Amount,
		&transaction.FeePercent,
		&transaction.FeeAmount,
		&transaction.TotalDebit,
		&transaction.FeeAccountID,
		&transaction.Status,
		&transaction.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка записи транзакции: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	utils.LogSuccess("TransactionRepo", fmt.Sprintf(" Транзакция %s выполнена: %s → %s (%.2f + %.2f комиссии)",
		transactionID, fromAccountID, toAccountID, amount, feeAmount))

	return &transaction, nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	query := `
		SELECT id, type, from_account_id, to_account_id, amount,
		       fee_percent, fee_amount, total_debit, fee_account_id,
		       status, created_at
		FROM transactions
		WHERE id = $1
	`

	var transaction models.Transaction
	err := r.db.QueryRow(ctx, query, transactionID).Scan(
		&transaction.ID,
		&transaction.Type,
		&transaction.FromAccountID,
		&transaction.ToAccountID,
		&transaction.Amount,
		&transaction.FeePercent,
		&transaction.FeeAmount,
		&transaction.TotalDebit,
		&transaction.FeeAccountID,
		&transaction.Status,
		&transaction.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("транзакция не найдена")
		}
		return nil, fmt.Errorf("ошибка получения транзакции: %w", err)
	}

	return &transaction, nil
}

func (r *TransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]models.Transaction, error) {
	query := `
		SELECT id, type, from_account_id, to_account_id, amount,
		       fee_percent, fee_amount, total_debit, fee_account_id,
		       status, created_at
		FROM transactions
		WHERE from_account_id = $1 OR to_account_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения транзакций: %w", err)
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		err := rows.Scan(
			&tx.ID,
			&tx.Type,
			&tx.FromAccountID,
			&tx.ToAccountID,
			&tx.Amount,
			&tx.FeePercent,
			&tx.FeeAmount,
			&tx.TotalDebit,
			&tx.FeeAccountID,
			&tx.Status,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования транзакции: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func (r *TransactionRepository) GetByUserID(ctx context.Context, userID string) ([]models.Transaction, error) {
	query := `
		SELECT t.id, t.type, t.from_account_id, t.to_account_id, t.amount,
		       t.fee_percent, t.fee_amount, t.total_debit, t.fee_account_id,
		       t.status, t.created_at
		FROM transactions t
		INNER JOIN accounts a1 ON t.from_account_id = a1.id
		WHERE a1.user_id = $1
		UNION
		SELECT t.id, t.type, t.from_account_id, t.to_account_id, t.amount,
		       t.fee_percent, t.fee_amount, t.total_debit, t.fee_account_id,
		       t.status, t.created_at
		FROM transactions t
		INNER JOIN accounts a2 ON t.to_account_id = a2.id
		WHERE a2.user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения транзакций пользователя: %w", err)
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		err := rows.Scan(
			&tx.ID,
			&tx.Type,
			&tx.FromAccountID,
			&tx.ToAccountID,
			&tx.Amount,
			&tx.FeePercent,
			&tx.FeeAmount,
			&tx.TotalDebit,
			&tx.FeeAccountID,
			&tx.Status,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования транзакции: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}
