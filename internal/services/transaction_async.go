package services

import (
	"bank-prototype/internal/models"
	"bank-prototype/internal/utils"
	"bank-prototype/internal/worker"
	"context"
	"errors"
	"fmt"
)

func (s *TransactionService) CreateTransaction(userID string, req models.TransactionRequest) (*models.Transaction, error) {
	ctx := context.Background()

	utils.LogInfo("TransactionService", fmt.Sprintf("Создание транзакции: тип=%s, от=%s, к=%s, сумма=%.2f",
		req.Type, req.FromAccountID, req.ToAccountID, req.Amount))

	var transaction *models.Transaction
	var err error

	switch req.Type {
	case "transfer":
		transferReq := models.TransferRequest{
			FromAccountID: req.FromAccountID,
			ToAccountID:   req.ToAccountID,
			Amount:        req.Amount,
		}
		transaction, err = s.Transfer(ctx, userID, transferReq)

	case "payment":
		paymentReq := models.PaymentRequest{
			FromAccountID: req.FromAccountID,
			ToAccountID:   req.ToAccountID,
			Amount:        req.Amount,
		}
		transaction, err = s.Payment(ctx, userID, paymentReq)

	default:
		return nil, errors.New("неподдерживаемый тип транзакции")
	}

	if err != nil {
		utils.LogError("TransactionService", "Ошибка создания транзакции", err)
		return nil, err
	}

	utils.LogSuccess("TransactionService", fmt.Sprintf("Транзакция %s успешно создана", transaction.ID))
	return transaction, nil
}

// CreateTransactionAsync - Асинхронное создание транзакции через Worker Pool
func (s *TransactionService) CreateTransactionAsync(userID string, req models.TransactionRequest) error {
	if s.workerPool == nil {
		return errors.New("worker pool не инициализирован")
	}

	transactionID := fmt.Sprintf("tx-%s-%d", userID, worker.GetCurrentTimeMs())

	job := worker.Job{
		ID: transactionID,
		Task: func() error {
			_, err := s.CreateTransaction(userID, req)
			return err
		},
	}

	if err := s.workerPool.Submit(job); err != nil {
		utils.LogError("TransactionService", "Не удалось добавить транзакцию в очередь", err)
		return err
	}

	utils.LogInfo("TransactionService", fmt.Sprintf("Транзакция %s добавлена в очередь обработки", transactionID))
	return nil
}
