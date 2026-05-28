package main

import (
	"log"
	"os"

	"basic-example/app/controllers"
	"basic-example/routes"

	"github.com/savuerka/ordin"
)

func main() {
	db := ordin.MustPostgresEnv(
		"DATABASE_URL",
		"postgres://postgres:postgres@localhost:5432/larago?sslmode=disable",
	)
	defer db.Close()

	ordin.MustMigrate(db, "migrations")

	options := []ordin.Option{
		ordin.Dev(),
		ordin.WithViews("resources/views"),
	}

	if getenv("S3_ENABLED", "false") == "true" {
		storage := ordin.MustS3Storage(ordin.S3ConfigFromEnv("S3"))
		options = append(options, ordin.WithStorage(storage))
	}

	if getenv("RABBITMQ_ENABLED", "false") == "true" {
		queue := ordin.MustRabbitQueue(ordin.RabbitMQConfigFromEnv("RABBITMQ"))
		defer queue.Close()
		options = append(options, ordin.WithQueue(queue))
	}

	app := ordin.New(options...)
	routes.Register(app, controllers.UserController{DB: db}, controllers.ServiceController{})

	addr := getenv("APP_ADDR", ":8080")
	log.Printf("listening on %s", addr)
	log.Fatal(app.Run(addr))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
