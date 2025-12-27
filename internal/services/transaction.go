package services

import (
	"context"
	"errors"
	"fmt"

	"bank-prototype/internal/cache"
	"bank-prototype/internal/models"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/utils"
	"bank-prototype/internal/worker"
)

var (
	ErrInvalidAmount = errors.New("сумма должна быть больше 0")
	ErrSelfTransfer  = errors.New("нельзя переводить на свой же счёт")
)

type TransactionService struct {
	transactionRepo *repository.TransactionRepository
	accountRepo     *repository.AccountRepository
	cache           *cache.RedisCache
	workerPool      *worker.WorkerPool
}

func NewTransactionService(
	transactionRepo *repository.TransactionRepository,
	accountRepo *repository.AccountRepository,
) *TransactionService {
	return &TransactionService{
		transactionRepo: transactionRepo,
		accountRepo:     accountRepo,
		cache:           nil,
	}
}

func NewTransactionServiceWithCache(
	transactionRepo *repository.TransactionRepository,
	accountRepo *repository.AccountRepository,
	cache *cache.RedisCache,
) *TransactionService {
	return &TransactionService{
		transactionRepo: transactionRepo,
		accountRepo:     accountRepo,
		cache:           cache,
	}
}

// SetWorkerPool устанавливает пул воркеров для асинхронной обработки
func (s *TransactionService) SetWorkerPool(pool *worker.WorkerPool) {
	s.workerPool = pool
	utils.LogSuccess("TransactionService", "Worker Pool подключен к сервису транзакций")
}

func (s *TransactionService) Transfer(ctx context.Context, userID string, req models.TransferRequest) (*models.Transaction, error) {
	utils.LogInfo("TransactionService", fmt.Sprintf("Перевод от пользователя %s: %s → %s (сумма: %.2f)",
		userID, req.FromAccountID, req.ToAccountID, req.Amount))

	if err := s.validateTransfer(ctx, userID, req.FromAccountID, req.ToAccountID, req.Amount); err != nil {
		utils.LogError("TransactionService", "Ошибка валидации перевода", err)
		return nil, err
	}

	feeAmount := req.Amount * 0.01
	totalDebit := req.Amount + feeAmount

	utils.LogInfo("TransactionService", fmt.Sprintf("Расчёт: сумма %.2f + комиссия %.2f (1%%) = %.2f",
		req.Amount, feeAmount, totalDebit))

	transaction, err := s.transactionRepo.ExecuteTransfer(
		ctx,
		req.FromAccountID,
		req.ToAccountID,
		req.Amount,
		feeAmount,
		1,
		"transfer",
	)

	if err != nil {
		utils.LogError("TransactionService", "Ошибка выполнения перевода", err)
		return nil, err
	}

	s.invalidateCacheAsync(ctx, req.FromAccountID, req.ToAccountID, transaction.ID)

	utils.LogSuccess("TransactionService", fmt.Sprintf("Перевод %s успешно выполнен", transaction.ID))

	return transaction, nil
}

func (s *TransactionService) Payment(ctx context.Context, userID string, req models.PaymentRequest) (*models.Transaction, error) {
	utils.LogInfo("TransactionService", fmt.Sprintf("Платёж от пользователя %s: %s → %s (сумма: %.2f)",
		userID, req.FromAccountID, req.ToAccountID, req.Amount))

	if err := s.validateTransfer(ctx, userID, req.FromAccountID, req.ToAccountID, req.Amount); err != nil {
		utils.LogError("TransactionService", "Ошибка валидации платежа", err)
		return nil, err
	}

	feeAmount := req.Amount * 0.03
	totalDebit := req.Amount + feeAmount

	utils.LogInfo("TransactionService", fmt.Sprintf("Расчёт: сумма %.2f + комиссия %.2f (3%%) = %.2f",
		req.Amount, feeAmount, totalDebit))

	transaction, err := s.transactionRepo.ExecuteTransfer(
		ctx,
		req.FromAccountID,
		req.ToAccountID,
		req.Amount,
		feeAmount,
		3,
		"payment",
	)

	if err != nil {
		utils.LogError("TransactionService", "Ошибка выполнения платежа", err)
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.Delete(ctx,
			cache.AccountBalanceKey(req.FromAccountID),
			cache.AccountBalanceKey(req.ToAccountID),
			cache.AccountBalanceKey(repository.SystemBankAccountID),
		)
		utils.LogInfo("Cache", fmt.Sprintf("Инвалидирован кеш балансов счетов: %s, %s, system", req.FromAccountID, req.ToAccountID))
	}

	utils.LogSuccess("TransactionService", fmt.Sprintf("Платёж %s успешно выполнен", transaction.ID))

	return transaction, nil
}

func (s *TransactionService) GetTransactionHistory(ctx context.Context, userID string, accountID *string) ([]models.Transaction, error) {
	if accountID != nil {
		utils.LogInfo("TransactionService", fmt.Sprintf("Получение истории транзакций по счёту %s", *accountID))

		account, err := s.accountRepo.GetByID(ctx, *accountID)
		if err != nil {
			return nil, repository.ErrAccountNotFound
		}

		if account.UserID != userID {
			utils.LogWarning("TransactionService", fmt.Sprintf("Попытка доступа к чужому счёту %s пользователем %s", *accountID, userID))
			return nil, ErrUnauthorizedAccess
		}

		transactions, err := s.transactionRepo.GetByAccountID(ctx, *accountID)
		if err != nil {
			utils.LogError("TransactionService", "Ошибка получения транзакций", err)
			return nil, err
		}

		utils.LogSuccess("TransactionService", fmt.Sprintf("Найдено %d транзакций по счёту %s", len(transactions), *accountID))
		return transactions, nil
	}

	utils.LogInfo("TransactionService", fmt.Sprintf("Получение всех транзакций пользователя %s", userID))

	transactions, err := s.transactionRepo.GetByUserID(ctx, userID)
	if err != nil {
		utils.LogError("TransactionService", "Ошибка получения транзакций пользователя", err)
		return nil, err
	}

	utils.LogSuccess("TransactionService", fmt.Sprintf("Найдено %d транзакций для пользователя %s", len(transactions), userID))
	return transactions, nil
}

func (s *TransactionService) GetTransactionByID(ctx context.Context, userID, transactionID string) (*models.Transaction, error) {
	utils.LogInfo("TransactionService", fmt.Sprintf("Получение транзакции %s пользователем %s", transactionID, userID))

	transaction, err := s.transactionRepo.GetByID(ctx, transactionID)
	if err != nil {
		utils.LogError("TransactionService", "Транзакция не найдена", err)
		return nil, err
	}

	fromAccount, err1 := s.accountRepo.GetByID(ctx, transaction.FromAccountID)
	toAccount, err2 := s.accountRepo.GetByID(ctx, transaction.ToAccountID)

	if err1 != nil && err2 != nil {
		return nil, errors.New("ошибка проверки доступа к транзакции")
	}

	hasAccess := false
	if err1 == nil && fromAccount.UserID == userID {
		hasAccess = true
	}
	if err2 == nil && toAccount.UserID == userID {
		hasAccess = true
	}

	if !hasAccess {
		utils.LogWarning("TransactionService", fmt.Sprintf("Попытка доступа к чужой транзакции %s пользователем %s", transactionID, userID))
		return nil, ErrUnauthorizedAccess
	}

	utils.LogSuccess("TransactionService", fmt.Sprintf("Транзакция %s получена", transactionID))
	return transaction, nil
}

func (s *TransactionService) validateTransfer(ctx context.Context, userID, fromAccountID, toAccountID string, amount float64) error {

	if amount <= 0 {
		return ErrInvalidAmount
	}

	if fromAccountID == toAccountID {
		return ErrSelfTransfer
	}

	fromAccount, err := s.accountRepo.GetByID(ctx, fromAccountID)
	if err != nil {
		return repository.ErrAccountNotFound
	}

	if fromAccount.UserID != userID {
		return ErrUnauthorizedAccess
	}

	if fromAccount.Status != "active" {
		return repository.ErrAccountClosed
	}

	toAccount, err := s.accountRepo.GetByID(ctx, toAccountID)
	if err != nil {
		return repository.ErrAccountNotFound
	}

	if toAccount.Status != "active" {
		return repository.ErrAccountClosed
	}

	return nil
}

// invalidateCacheAsync - асинхронная инвалидация кеша через Worker Pool
func (s *TransactionService) invalidateCacheAsync(ctx context.Context, fromAccountID, toAccountID, transactionID string) {
	if s.cache == nil {
		return
	}

	// Если Worker Pool доступен, используем его для асинхронной обработки
	if s.workerPool != nil {
		job := worker.Job{
			ID: fmt.Sprintf("cache-invalidate-%s", transactionID),
			Task: func() error {
				return s.cache.Delete(ctx,
					cache.AccountBalanceKey(fromAccountID),
					cache.AccountBalanceKey(toAccountID),
					cache.AccountBalanceKey(repository.SystemBankAccountID),
				)
			},
		}

		// Пытаемся добавить задачу в очередь (неблокирующая операция)
		if err := s.workerPool.Submit(job); err != nil {
			// Если очередь переполнена, выполняем синхронно
			utils.LogWarning("TransactionService", "Worker Pool переполнен, инвалидация кеша выполняется синхронно")
			_ = s.cache.Delete(ctx,
				cache.AccountBalanceKey(fromAccountID),
				cache.AccountBalanceKey(toAccountID),
				cache.AccountBalanceKey(repository.SystemBankAccountID),
			)
		} else {
			utils.LogDebug("TransactionService", "Инвалидация кеша добавлена в Worker Pool для транзакции %s", transactionID)
		}
	} else {
		// Если Worker Pool недоступен, выполняем синхронно
		_ = s.cache.Delete(ctx,
			cache.AccountBalanceKey(fromAccountID),
			cache.AccountBalanceKey(toAccountID),
			cache.AccountBalanceKey(repository.SystemBankAccountID),
		)
		utils.LogInfo("Cache", fmt.Sprintf("Инвалидирован кеш балансов счетов: %s, %s, system", fromAccountID, toAccountID))
	}
}
