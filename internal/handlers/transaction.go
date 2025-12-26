package handlers

import (
	"bank-prototype/internal/models"
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
)

type TransactionHandler struct {
	service *services.TransactionService
}

func NewTransactionHandler(service *services.TransactionService) *TransactionHandler {
	utils.LogSuccess("TransactionHandler", "Инициализирован обработчик транзакций")
	return &TransactionHandler{service: service}
}

// Transfer обрабатывает POST /transactions/transfer
func (h *TransactionHandler) Transfer(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()

	// Получаем user_id из контекста (добавлено middleware)
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("TransactionHandler", "Не удалось получить user_id из контекста", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "unauthorized"})
		utils.LogResponse("/transactions/transfer", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	utils.LogRequest("POST", "/transactions/transfer", userID)

	var req models.TransferRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		utils.LogError("TransactionHandler", "Ошибка парсинга JSON", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "invalid request body"})
		utils.LogResponse("/transactions/transfer", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	transaction, err := h.service.Transfer(ctx, userID, req)
	if err != nil {
		utils.LogError("TransactionHandler", "Ошибка выполнения перевода", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": err.Error()})
		utils.LogResponse("/transactions/transfer", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogSuccess("TransactionHandler", fmt.Sprintf("Перевод выполнен: %s", transaction.ID))

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"message":     "transfer successful",
		"transaction": transaction,
	})

	utils.LogResponse("/transactions/transfer", fasthttp.StatusCreated, time.Since(startTime))
}

// Payment обрабатывает POST /transactions/payment
func (h *TransactionHandler) Payment(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()

	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("TransactionHandler", "Не удалось получить user_id из контекста", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "unauthorized"})
		utils.LogResponse("/transactions/payment", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	utils.LogRequest("POST", "/transactions/payment", userID)

	var req models.PaymentRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		utils.LogError("TransactionHandler", "Ошибка парсинга JSON", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "invalid request body"})
		utils.LogResponse("/transactions/payment", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	transaction, err := h.service.Payment(ctx, userID, req)
	if err != nil {
		utils.LogError("TransactionHandler", "Ошибка выполнения платежа", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": err.Error()})
		utils.LogResponse("/transactions/payment", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogSuccess("TransactionHandler", fmt.Sprintf("Платёж выполнен: %s", transaction.ID))

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"message":     "payment successful",
		"transaction": transaction,
	})

	utils.LogResponse("/transactions/payment", fasthttp.StatusCreated, time.Since(startTime))
}

// GetHistory обрабатывает GET /transactions или GET /transactions?account_id=xxx
func (h *TransactionHandler) GetHistory(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()

	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("TransactionHandler", "Не удалось получить user_id из контекста", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "unauthorized"})
		utils.LogResponse("/transactions", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	utils.LogRequest("GET", "/transactions", userID)

	// Получаем опциональный параметр account_id
	var accountID *string
	if accountIDBytes := ctx.QueryArgs().Peek("account_id"); len(accountIDBytes) > 0 {
		accountIDStr := string(accountIDBytes)
		accountID = &accountIDStr
		utils.LogInfo("TransactionHandler", fmt.Sprintf("Фильтр по счёту: %s", accountIDStr))
	}

	transactions, err := h.service.GetTransactionHistory(ctx, userID, accountID)
	if err != nil {
		utils.LogError("TransactionHandler", "Ошибка получения истории", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": err.Error()})
		utils.LogResponse("/transactions", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogSuccess("TransactionHandler", fmt.Sprintf("История получена: %d транзакций", len(transactions)))

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"transactions": transactions,
		"total":        len(transactions),
	})

	utils.LogResponse("/transactions", fasthttp.StatusOK, time.Since(startTime))
}

// GetByID обрабатывает GET /transactions/:id
func (h *TransactionHandler) GetByID(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()

	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("TransactionHandler", "Не удалось получить user_id из контекста", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "unauthorized"})
		utils.LogResponse("/transactions/:id", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	transactionID, ok := ctx.UserValue("id").(string)
	if !ok {
		utils.LogError("TransactionHandler", "Не удалось получить transaction_id из URL", nil)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": "invalid transaction id"})
		utils.LogResponse("/transactions/:id", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogRequest("GET", fmt.Sprintf("/transactions/%s", transactionID), userID)

	transaction, err := h.service.GetTransactionByID(ctx, userID, transactionID)
	if err != nil {
		utils.LogError("TransactionHandler", "Ошибка получения транзакции", err)
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{"error": err.Error()})
		utils.LogResponse("/transactions/:id", fasthttp.StatusNotFound, time.Since(startTime))
		return
	}

	utils.LogSuccess("TransactionHandler", fmt.Sprintf("Транзакция получена: %s", transactionID))

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(transaction)

	utils.LogResponse("/transactions/:id", fasthttp.StatusOK, time.Since(startTime))
}
