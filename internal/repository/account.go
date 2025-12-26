package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgxpool"

	"bank-prototype/internal/models"
	"bank-prototype/internal/utils"
)

var (
	ErrAccountNotFound     = errors.New("счёт не найден")
	ErrAccountClosed       = errors.New("счёт закрыт")
	ErrInsufficientBalance = errors.New("недостаточно средств")
	SystemBankAccountID    = "00000000000001"
)

type AccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) generateAccountID(ctx context.Context) (string, error) {
	const maxAttempts = 10

	for attempt := 0; attempt < maxAttempts; attempt++ {

		maxValue := big.NewInt(1_000_000_000_000) // 10^12
		n, err := rand.Int(rand.Reader, maxValue)
		if err != nil {
			return "", fmt.Errorf("ошибка генерации случайного числа: %w", err)
		}

		accountID := fmt.Sprintf("13%012d", n.Int64())

		// Проверяем уникальность
		var exists bool
		err = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)", accountID).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("ошибка проверки уникальности: %w", err)
		}

		if !exists {
			return accountID, nil
		}

		utils.LogWarning("AccountRepo", fmt.Sprintf("Коллизия ID счёта %s, попытка %d/%d", accountID, attempt+1, maxAttempts))
	}

	return "", errors.New("не удалось сгенерировать уникальный ID счёта после нескольких попыток")
}

func (r *AccountRepository) Create(ctx context.Context, userID string) (*models.Account, error) {
	accountID, err := r.generateAccountID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO accounts (id, user_id, balance, status, created_at)
		VALUES ($1, $2, 100.00, 'active', NOW())
		RETURNING id, user_id, balance, status, created_at
	`

	var account models.Account
	err = r.db.QueryRow(ctx, query, accountID, userID).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.Status,
		&account.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка создания счёта: %w", err)
	}

	return &account, nil
}

func (r *AccountRepository) GetByID(ctx context.Context, accountID string) (*models.Account, error) {
	query := `
		SELECT id, user_id, balance, status, created_at
		FROM accounts
		WHERE id = $1
	`

	var account models.Account
	err := r.db.QueryRow(ctx, query, accountID).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.Status,
		&account.CreatedAt,
	)

	if err != nil {
		return nil, ErrAccountNotFound
	}

	return &account, nil
}

func (r *AccountRepository) GetByUserID(ctx context.Context, userID string) ([]models.Account, error) {
	query := `
		SELECT id, user_id, balance, status, created_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка счетов: %w", err)
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		err := rows.Scan(
			&account.ID,
			&account.UserID,
			&account.Balance,
			&account.Status,
			&account.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования счёта: %w", err)
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

// CountActiveAccountsByUserID возвращает количество активных счетов пользователя
func (r *AccountRepository) CountActiveAccountsByUserID(ctx context.Context, userID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM accounts WHERE user_id = $1 AND status = 'active'`

	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчёта активных счетов: %w", err)
	}

	return count, nil
}

// UpdateStatus изменяет статус счёта (используется при закрытии)
func (r *AccountRepository) UpdateStatus(ctx context.Context, accountID, status string) error {
	query := `
		UPDATE accounts
		SET status = $1
		WHERE id = $2
	`

	result, err := r.db.Exec(ctx, query, status, accountID)
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса счёта: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAccountNotFound
	}

	return nil
}

func (r *AccountRepository) GetBalance(ctx context.Context, accountID string) (float64, error) {
	var balance float64
	query := `SELECT balance FROM accounts WHERE id = $1 AND status = 'active'`

	err := r.db.QueryRow(ctx, query, accountID).Scan(&balance)
	if err != nil {
		return 0, ErrAccountNotFound
	}

	return balance, nil
}

func (r *AccountRepository) UpdateBalance(ctx context.Context, accountID string, newBalance float64) error {
	query := `
		UPDATE accounts
		SET balance = $1
		WHERE id = $2 AND status = 'active'
	`

	result, err := r.db.Exec(ctx, query, newBalance, accountID)
	if err != nil {
		return fmt.Errorf("ошибка обновления баланса: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAccountNotFound
	}

	return nil
}
