package main

import (
	"bank-prototype/internal/cache"
	"bank-prototype/internal/handlers"
	"bank-prototype/internal/middleware"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/services"
	"bank-prototype/internal/utils"
	"bank-prototype/internal/worker"
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
)

func main() {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	utils.LogInfo("Server", "Запуск банковской системы...")

	if err := runMigrations(); err != nil {
		utils.LogError("Server", "Критическая ошибка миграций", err)
		os.Exit(1)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://user:pass@localhost:5435/bank?sslmode=disable"
	}
	utils.LogInfo("Database", "Подключение к PostgreSQL...")

	dbpool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		utils.LogError("Database", "Ошибка подключения к базе данных", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	utils.LogSuccess("Database", "Подключение к базе данных установлено")

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	utils.LogInfo("Redis", "Подключение к Redis: "+redisURL)

	redisCache := cache.NewRedisCache(redisURL)
	defer func() {
		_ = redisCache.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisCache.Ping(ctx); err != nil {
		utils.LogError("Redis", "Ошибка подключения к Redis", err)
		os.Exit(1)
	}
	utils.LogSuccess("Redis", "Подключение к Redis установлено")

	// Инициализация Worker Pool
	utils.LogInfo("WorkerPool", "Инициализация пула воркеров...")
	workerPool := worker.NewWorkerPool(10, 1000, 3) // 10 воркеров, очередь на 1000 задач, 3 повтора
	workerPool.Start()
	defer func() {
		utils.LogInfo("WorkerPool", "Остановка пула воркеров...")
		if err := workerPool.Shutdown(30 * time.Second); err != nil {
			utils.LogError("WorkerPool", "Ошибка остановки пула", err)
		}
	}()

	userRepo := repository.NewUserRepository(dbpool)
	accountRepo := repository.NewAccountRepository(dbpool)
	transactionRepo := repository.NewTransactionRepository(dbpool)

	authService := services.NewAuthService("your_jwt_secret_change_me_in_production", time.Hour*24)
	accountService := services.NewAccountServiceWithCache(accountRepo, redisCache)
	transactionService := services.NewTransactionServiceWithCache(transactionRepo, accountRepo, redisCache)
	transactionService.SetWorkerPool(workerPool) // Устанавливаем worker pool

	authMiddleware := middleware.NewAuthMiddleware(authService)

	authHandler := handlers.NewAuthHandler(authService, userRepo)
	accountHandler := handlers.NewAccountHandler(accountService)
	transactionHandler := handlers.NewTransactionHandler(transactionService)

	utils.LogInfo("Server", "Запуск HTTP сервера на порту :8080...")

	err = fasthttp.ListenAndServe(":8080", func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		switch {
		case method == "GET" && path == "/health":
			healthHandler(ctx)

		case method == "POST" && path == "/register":
			authHandler.RegisterHandler(ctx)

		case method == "POST" && path == "/login":
			authHandler.LoginHandler(ctx)

		case method == "DELETE" && path == "/users/me":
			authMiddleware.RequireAuth(authHandler.DeleteUserHandler)(ctx)

		case method == "POST" && path == "/accounts":
			authMiddleware.RequireAuth(accountHandler.CreateAccount)(ctx)

		case method == "GET" && path == "/accounts":
			authMiddleware.RequireAuth(accountHandler.GetAccounts)(ctx)

		case method == "GET" && len(path) > 10 && path[:10] == "/accounts/":
			accountID := path[10:]
			ctx.SetUserValue("id", accountID)
			authMiddleware.RequireAuth(accountHandler.GetAccountByID)(ctx)

		case method == "DELETE" && len(path) > 10 && path[:10] == "/accounts/":
			accountID := path[10:]
			ctx.SetUserValue("id", accountID)
			authMiddleware.RequireAuth(accountHandler.DeleteAccount)(ctx)

		case method == "POST" && path == "/transactions/transfer":
			authMiddleware.RequireAuth(transactionHandler.Transfer)(ctx)

		case method == "POST" && path == "/transactions/payment":
			authMiddleware.RequireAuth(transactionHandler.Payment)(ctx)

		case method == "GET" && path == "/transactions":
			authMiddleware.RequireAuth(transactionHandler.GetHistory)(ctx)

		case method == "GET" && len(path) > 14 && path[:14] == "/transactions/":
			transactionID := path[14:]
			ctx.SetUserValue("id", transactionID)
			authMiddleware.RequireAuth(transactionHandler.GetByID)(ctx)

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
		"message": "Bank Prototype API is running",
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
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://user:pass@localhost:5435/bank?sslmode=disable"
	}

	utils.LogInfo("Migration", "Запуск миграций базы данных")

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

	utils.LogSuccess("Migration", "Миграции выполнены успешно")
	return nil
}
