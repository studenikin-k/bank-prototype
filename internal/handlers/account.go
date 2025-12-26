package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/valyala/fasthttp"

	"bank-prototype/internal/models"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
)

type AccountHandler struct {
	accountService *services.AccountService
}

func NewAccountHandler(accountService *services.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

// CreateAccount обрабатывает POST /accounts - создание нового счёта
func (h *AccountHandler) CreateAccount(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("AccountHandler", "user_id не найден в контексте", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	utils.LogInfo("AccountHandler", " Запрос на создание счёта от пользователя: "+userID)

	// Создаём счёт
	account, err := h.accountService.CreateAccount(ctx, userID)
	if err != nil {
		if err == services.ErrAccountLimitReached {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Достигнут лимит активных счетов (максимум 5)"})
		} else {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Ошибка создания счёта"})
		}
		utils.LogError("AccountHandler", "Ошибка создания счёта", err)
		return
	}

	// Формируем ответ
	response := models.AccountResponse{
		ID:        account.ID,
		Balance:   account.Balance,
		Status:    account.Status,
		CreatedAt: account.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(response)

	utils.LogSuccess("AccountHandler", " Счёт успешно создан: "+account.ID)
}

// GetAccounts обрабатывает GET /accounts - список всех активных счетов пользователя
func (h *AccountHandler) GetAccounts(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("AccountHandler", "user_id не найден в контексте", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	utils.LogInfo("AccountHandler", " Запрос списка счетов от пользователя: "+userID)

	accounts, err := h.accountService.GetUserAccounts(ctx, userID)
	if err != nil {
		utils.LogError("AccountHandler", "Ошибка получения счетов", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Ошибка получения счетов"})
		return
	}

	// Формируем список ответов и подсчитываем статистику
	var accountResponses []models.AccountResponse
	activeCount := 0
	closedCount := 0

	for _, acc := range accounts {
		accountResponses = append(accountResponses, models.AccountResponse{
			ID:        acc.ID,
			Balance:   acc.Balance,
			Status:    acc.Status,
			CreatedAt: acc.CreatedAt.Format("2006-01-02 15:04:05"),
		})

		if acc.Status == "active" {
			activeCount++
		} else {
			closedCount++
		}
	}

	response := models.AccountListResponse{
		Accounts:      accountResponses,
		Total:         len(accountResponses),
		ActiveCount:   activeCount,
		ClosedCount:   closedCount,
		MaxAccounts:   5,
		CanCreateMore: activeCount < 5,
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(response)

	utils.LogSuccess("AccountHandler", fmt.Sprintf("✅ Отправлен список счетов: %d шт. (активных: %d, закрытых: %d)", len(accounts), activeCount, closedCount))
}

// GetAccountByID обрабатывает GET /accounts/{id} - информация о конкретном счёте
func (h *AccountHandler) GetAccountByID(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("AccountHandler", "user_id не найден в контексте", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	accountID := ctx.UserValue("id").(string)
	utils.LogInfo("AccountHandler", "📥 Запрос информации о счёте: "+accountID)

	account, err := h.accountService.GetAccount(ctx, accountID, userID)
	if err != nil {
		if err == repository.ErrAccountNotFound {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Счёт не найден"})
		} else if err == services.ErrUnauthorizedAccess {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Нет доступа к данному счёту"})
		} else if err == repository.ErrAccountClosed {
			ctx.SetStatusCode(fasthttp.StatusGone)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Счёт закрыт"})
		} else {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Ошибка получения счёта"})
		}
		utils.LogError("AccountHandler", "Ошибка получения счёта", err)
		return
	}

	response := models.AccountResponse{
		ID:        account.ID,
		Balance:   account.Balance,
		Status:    account.Status,
		CreatedAt: account.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(response)

	utils.LogSuccess("AccountHandler", "✅ Информация о счёте отправлена: "+accountID)
}

func (h *AccountHandler) DeleteAccount(ctx *fasthttp.RequestCtx) {
	userID, ok := ctx.UserValue("user_id").(string)
	if !ok {
		utils.LogError("AccountHandler", "user_id не найден в контексте", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	accountID := ctx.UserValue("id").(string)
	utils.LogInfo("AccountHandler", "Запрос на закрытие счёта: "+accountID)

	err := h.accountService.DeleteAccount(ctx, accountID, userID)
	if err != nil {
		if err == repository.ErrAccountNotFound {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Счёт не найден"})
		} else if err == services.ErrUnauthorizedAccess {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Нет доступа к данному счёту"})
		} else if err == services.ErrAccountAlreadyClosed {
			ctx.SetStatusCode(fasthttp.StatusGone)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Счёт уже закрыт"})
		} else {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			_ = json.NewEncoder(ctx).Encode(map[string]string{"error": "Ошибка закрытия счёта"})
		}
		utils.LogError("AccountHandler", "Ошибка закрытия счёта", err)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]string{
		"message":    "Счёт успешно закрыт",
		"account_id": accountID,
	})

	utils.LogSuccess("AccountHandler", "Счёт успешно закрыт: "+accountID)
}
