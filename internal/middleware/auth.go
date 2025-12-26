package middleware

import (
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

type AuthMiddleware struct {
	authService *services.AuthService
}

func NewAuthMiddleware(authService *services.AuthService) *AuthMiddleware {
	utils.LogSuccess("Middleware", "Инициализирован middleware авторизации")
	return &AuthMiddleware{
		authService: authService,
	}
}

func (m *AuthMiddleware) RequireAuth(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		startTime := time.Now()

		authHeader := string(ctx.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			utils.LogWarning("Middleware", "Отсутствует заголовок Authorization")
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			ctx.SetContentType("application/json")
			json.NewEncoder(ctx).Encode(map[string]string{
				"error": "Требуется авторизация",
			})
			utils.LogResponse("RequireAuth", fasthttp.StatusUnauthorized, time.Since(startTime))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.LogWarning("Middleware", "Неверный формат заголовка Authorization")
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			ctx.SetContentType("application/json")
			json.NewEncoder(ctx).Encode(map[string]string{
				"error": "Неверный формат токена",
			})
			utils.LogResponse("RequireAuth", fasthttp.StatusUnauthorized, time.Since(startTime))
			return
		}

		token := parts[1]

		claims, err := m.authService.ValidateToken(token)
		if err != nil {
			utils.LogWarning("Middleware", fmt.Sprintf("Невалидный токен: %v", err))
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			ctx.SetContentType("application/json")
			json.NewEncoder(ctx).Encode(map[string]string{
				"error": "Невалидный или истёкший токен",
			})
			utils.LogResponse("RequireAuth", fasthttp.StatusUnauthorized, time.Since(startTime))
			return
		}

		ctx.SetUserValue("user_id", claims.UserID)
		utils.LogDebug("Middleware", fmt.Sprintf("Аутентифицирован пользователь: %s", claims.UserID))

		next(ctx)
	}
}
