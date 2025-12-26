package handlers

import (
	"bank-prototype/internal/models"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
)

type AuthHandler struct {
	authService *services.AuthService
	userRepo    *repository.UserRepository
}

func NewAuthHandler(authService *services.AuthService, userRepo *repository.UserRepository) *AuthHandler {
	utils.LogSuccess("AuthHandler", "Инициализирован обработчик аутентификации")
	return &AuthHandler{
		authService: authService,
		userRepo:    userRepo,
	}
}

func (h *AuthHandler) RegisterHandler(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	utils.LogRequest("POST", "/register", "anonymous")

	var req models.RegisterRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		utils.LogError("AuthHandler", "Ошибка парсинга JSON", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Неверный формат данных",
		})
		utils.LogResponse("/register", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	if req.Name == "" || req.Password == "" {
		utils.LogWarning("AuthHandler", "Отсутствуют обязательные поля")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Имя и пароль обязательны",
		})
		utils.LogResponse("/register", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	if len(req.Password) < 6 {
		utils.LogWarning("AuthHandler", "Пароль слишком короткий")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Пароль должен быть не менее 6 символов",
		})
		utils.LogResponse("/register", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogInfo("AuthHandler", fmt.Sprintf("Регистрация пользователя: %s", req.Name))

	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		utils.LogError("AuthHandler", "Ошибка хеширования пароля", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Внутренняя ошибка сервера",
		})
		utils.LogResponse("/register", fasthttp.StatusInternalServerError, time.Since(startTime))
		return
	}

	user := &models.User{
		Name:         req.Name,
		PasswordHash: passwordHash,
	}

	if err := h.userRepo.Create(ctx, user); err != nil {
		utils.LogError("AuthHandler", fmt.Sprintf("Ошибка создания пользователя %s", req.Name), err)
		ctx.SetStatusCode(fasthttp.StatusConflict)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Пользователь с таким именем уже существует",
		})
		utils.LogResponse("/register", fasthttp.StatusConflict, time.Since(startTime))
		return
	}

	utils.LogSuccess("AuthHandler", fmt.Sprintf("Пользователь зарегистрирован: %s", user.Name))

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"message":    "Пользователь успешно зарегистрирован",
		"user_id":    user.ID,
		"name":       user.Name,
		"created_at": user.CreatedAt,
	})

	utils.LogResponse("/register", fasthttp.StatusCreated, time.Since(startTime))
}

// LoginHandler - вход пользователя
func (h *AuthHandler) LoginHandler(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	utils.LogRequest("POST", "/login", "anonymous")

	var req models.LoginRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		utils.LogError("AuthHandler", "Ошибка парсинга JSON", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Неверный формат данных",
		})
		utils.LogResponse("/login", fasthttp.StatusBadRequest, time.Since(startTime))
		return
	}

	utils.LogInfo("AuthHandler", fmt.Sprintf("Попытка входа пользователя: %s", req.Name))

	// Получение пользователя
	user, err := h.userRepo.GetByName(ctx, req.Name)
	if err != nil {
		utils.LogWarning("AuthHandler", fmt.Sprintf("Пользователь не найден: %s", req.Name))
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Неверное имя пользователя или пароль",
		})
		utils.LogResponse("/login", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	// Проверка пароля
	if err := h.authService.CheckPasswordHash(req.Password, user.PasswordHash); err != nil {
		utils.LogWarning("AuthHandler", fmt.Sprintf("Неверный пароль для пользователя: %s", req.Name))
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Неверное имя пользователя или пароль",
		})
		utils.LogResponse("/login", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	// Генерация токена
	token, err := h.authService.GenerateToken(user.ID)
	if err != nil {
		utils.LogError("AuthHandler", "Ошибка генерации токена", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Внутренняя ошибка сервера",
		})
		utils.LogResponse("/login", fasthttp.StatusInternalServerError, time.Since(startTime))
		return
	}

	utils.LogSuccess("AuthHandler", fmt.Sprintf("Пользователь вошёл: %s (ID: %s)", user.Name, user.ID))

	// Ответ
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"message":    "Вход выполнен успешно",
		"token":      token,
		"user_id":    user.ID,
		"name":       user.Name,
		"expires_in": "24h",
	})

	utils.LogResponse("/login", fasthttp.StatusOK, time.Since(startTime))
}

func (h *AuthHandler) DeleteUserHandler(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()

	userID, ok := ctx.UserValue("user_id").(string)
	if !ok || userID == "" {
		utils.LogError("AuthHandler", "user_id не найден в контексте", nil)
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Требуется авторизация",
		})
		utils.LogResponse("/users/me", fasthttp.StatusUnauthorized, time.Since(startTime))
		return
	}

	utils.LogRequest("DELETE", "/users/me", userID)
	utils.LogInfo("AuthHandler", fmt.Sprintf("Попытка удаления пользователя: %s", userID))

	if err := h.userRepo.Delete(ctx, userID); err != nil {
		utils.LogError("AuthHandler", fmt.Sprintf("Ошибка удаления пользователя %s", userID), err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(map[string]string{
			"error": "Ошибка удаления пользователя",
		})
		utils.LogResponse("/users/me", fasthttp.StatusInternalServerError, time.Since(startTime))
		return
	}

	utils.LogSuccess("AuthHandler", fmt.Sprintf("Пользователь удалён: %s", userID))

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"message": "Пользователь успешно удалён",
		"user_id": userID,
	})

	utils.LogResponse("/users/me", fasthttp.StatusOK, time.Since(startTime))
}
