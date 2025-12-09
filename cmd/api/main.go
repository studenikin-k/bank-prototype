package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/valyala/fasthttp"
)

func main() {

	log.Println("Сервер запускается...")

	if err := runMigrations(); err != nil {
		log.Fatalf("Ошибка миграций: %v", err)
	}

	err := fasthttp.ListenAndServe(":8080", func(ctx *fasthttp.RequestCtx) {

		if string(ctx.Path()) == "/health" {
			ctx.SetContentType("application/json")
			response := map[string]interface{}{
				"status":  http.StatusOK,
				"time":    time.Now().Format(time.RFC1123),
				"message": "Всё чики пуки братишка!",
			}

			jsonEncode, _ := json.Marshal(response)
			ctx.Write(jsonEncode)

		} else {
			ctx.WriteString("Неизвестный запрос")
		}
	})

	if err != nil {
		log.Fatal("Ошибка:", err)
	}
}

func runMigrations() error {
	dbURL := "postgres://user:pass@localhost:5435/bank?sslmode=disable"

	log.Println("Запуск миграции...")
	log.Println("URL подключения:", dbURL)

	migration, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Printf("Ошибка создания миграции: %v", err)
		return err
	}
	defer migration.Close()

	time.Sleep(2 * time.Second)

	if err := migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Printf("Ошибка применения миграций: %v", err)
		return err
	}

	log.Println("Миграции выполнены успешно")
	return nil
}
