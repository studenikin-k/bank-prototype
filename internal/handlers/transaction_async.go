package handlers

import (
	"bank-prototype/internal/models"
	"bank-prototype/internal/services"
	"encoding/json"
	"log"

	"github.com/valyala/fasthttp"
)

type TransactionAsyncHandler struct {
	transactionService *services.TransactionService
}

func NewTransactionAsyncHandler(transactionService *services.TransactionService) *TransactionAsyncHandler {
	return &TransactionAsyncHandler{
		transactionService: transactionService,
	}
}

// CreateTransactionAsync - Создание транзакции асинхронно
func (h *TransactionAsyncHandler) CreateTransactionAsync(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		log.Println("[ERROR] [TransactionAsyncHandler]  Не удалось получить user_id из контекста")
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req models.TransferRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		log.Printf("[ERROR] [TransactionAsyncHandler]  Ошибка парсинга запроса: %v\n", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		json.NewEncoder(ctx).Encode(map[string]string{"error": "Invalid request format"})
		return
	}

	log.Printf("[INFO] [TransactionAsyncHandler]  Асинхронный запрос на создание транзакции от пользователя %s: %+v\n", userID, req)

	// Запускаем обработку транзакции асинхронно
	go func() {
		transaction, err := h.transactionService.Transfer(ctx, userID, req)
		if err != nil {
			log.Printf("[ERROR] [TransactionAsyncHandler]  Ошибка создания транзакции: %v\n", err)
			return
		}
		log.Printf("[SUCCESS] [TransactionAsyncHandler]  Транзакция создана асинхронно: ID=%s, Amount=%.2f\n",
			transaction.ID, transaction.Amount)
	}()

	// Сразу возвращаем ответ клиенту
	ctx.SetStatusCode(fasthttp.StatusAccepted)
	json.NewEncoder(ctx).Encode(map[string]string{
		"status":  "accepted",
		"message": "Transaction is being processed",
	})
	log.Printf("[SUCCESS] [TransactionAsyncHandler]  Транзакция принята в обработку\n")
}

// GetTransactionsAsync - Получение транзакций с использованием горутин для параллельной обработки
func (h *TransactionAsyncHandler) GetTransactionsAsync(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		log.Println("[ERROR] [TransactionAsyncHandler]  Не удалось получить user_id из контекста")
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	accountID := string(ctx.QueryArgs().Peek("account_id"))
	if accountID == "" {
		log.Println("[ERROR] [TransactionAsyncHandler]  Отсутствует account_id")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		json.NewEncoder(ctx).Encode(map[string]string{"error": "account_id is required"})
		return
	}

	log.Printf("[INFO] [TransactionAsyncHandler]  Асинхронный запрос на получение транзакций: UserID=%s, AccountID=%s\n", userID, accountID)

	// Используем канал для получения результата
	resultChan := make(chan struct {
		transactions []models.Transaction
		err          error
	}, 1)

	// Запускаем получение транзакций в горутине
	go func() {
		transactions, err := h.transactionService.GetTransactionHistory(ctx, userID, &accountID)
		resultChan <- struct {
			transactions []models.Transaction
			err          error
		}{transactions, err}
	}()

	// Ждем результата
	result := <-resultChan

	if result.err != nil {
		log.Printf("[ERROR] [TransactionAsyncHandler]  Ошибка получения транзакций: %v\n", result.err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		json.NewEncoder(ctx).Encode(map[string]string{"error": result.err.Error()})
		return
	}

	log.Printf("[SUCCESS] [TransactionAsyncHandler]  Получено транзакций: %d\n", len(result.transactions))
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"transactions": result.transactions,
		"count":        len(result.transactions),
	})
}
