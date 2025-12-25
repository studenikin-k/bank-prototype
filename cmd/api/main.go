package main

import (
	"bank-prototype/internal/handlers"
	"bank-prototype/internal/middleware"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
)

func main() {
	utils.LogInfo("Server", "🚀 Запуск банковской системы...")

	// Миграции
	if err := runMigrations(); err != nil {
		utils.LogError("Server", "Критическая ошибка миграций", err)
		os.Exit(1)
	}

	// Подключение к БД
	dbURL := "postgres://user:pass@localhost:5435/bank?sslmode=disable"
	utils.LogInfo("Database", "Подключение к PostgreSQL...")

	dbpool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		utils.LogError("Database", "Ошибка подключения к базе данных", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	utils.LogSuccess("Database", "✓ Подключение к базе данных установлено")

	// Инициализация сервисов
	authService := services.NewAuthService("your_jwt_secret_change_me_in_production", time.Hour*24)
	userRepo := repository.NewUserRepository(dbpool)

	// Инициализация middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

	// Инициализация handlers
	authHandler := handlers.NewAuthHandler(authService, userRepo)

	// HTTP-сервер
	utils.LogInfo("Server", "Запуск HTTP сервера на порту :8080...")

	err = fasthttp.ListenAndServe(":8080", func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		// Роутинг
		switch {
		// Публичные эндпоинты (без авторизации)
		case method == "GET" && path == "/health":
			healthHandler(ctx)

		case method == "POST" && path == "/register":
			authHandler.RegisterHandler(ctx)

		case method == "POST" && path == "/login":
			authHandler.LoginHandler(ctx)

		// Защищённые эндпоинты (с авторизацией)
		case method == "DELETE" && path == "/users/me":
			authMiddleware.RequireAuth(authHandler.DeleteUserHandler)(ctx)

		default:
			utils.LogWarning("Router", "Неизвестный маршрут: "+method+" "+path)
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			ctx.SetContentType("application/json")
			json.NewEncoder(ctx).Encode(map[string]string{
				"error": "Маршрут не найден",
			})
		}
	})

	if err != nil {
		utils.LogError("Server", "Ошибка запуска сервера", err)
		os.Exit(1)
	}
}

func healthHandler(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	utils.LogRequest("GET", "/health", "system")

	ctx.SetContentType("application/json")
	response := map[string]interface{}{
		"status":  "OK",
		"time":    time.Now().Format(time.RFC1123),
		"message": "Всё чики пуки братишка! 🏦",
		"service": "Bank Prototype API",
		"version": "0.1.0",
	}

	if jsonEncode, err := json.Marshal(response); err == nil {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Write(jsonEncode)
	} else {
		utils.LogError("HealthCheck", "Ошибка кодирования JSON", err)
		ctx.Error("Ошибка кодирования JSON", fasthttp.StatusInternalServerError)
	}

	utils.LogResponse("/health", fasthttp.StatusOK, time.Since(startTime))
}

func runMigrations() error {
	dbURL := "postgres://user:pass@localhost:5435/bank?sslmode=disable"

	utils.LogInfo("Migration", "📋 Запуск миграций базы данных...")

	migration, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		utils.LogError("Migration", "Ошибка создания миграции", err)
		return err
	}
	defer migration.Close()

	time.Sleep(2 * time.Second)

	if err := migration.Up(); err != nil && err != migrate.ErrNoChange {
		utils.LogError("Migration", "Ошибка применения миграций", err)
		return err
	}

	utils.LogSuccess("Migration", "✓ Миграции выполнены успешно")
	return nil
}
