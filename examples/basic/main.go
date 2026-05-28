package main

import (
	"context"
	"log"
	"os"
	"time"

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	if getenv("REDIS_ENABLED", "false") == "true" {
		cache := ordin.MustRedisCache(ordin.RedisConfigFromEnv("REDIS"))
		defer cache.Close()
		options = append(options, ordin.WithRedis(cache))
	}

	if getenv("SFTP_ENABLED", "false") == "true" {
		sftpClient := ordin.MustSFTPClient(ordin.SFTPConfigFromEnv("SFTP"))
		defer sftpClient.Close()
		options = append(options, ordin.WithSFTP(sftpClient))
	}

	if getenv("SCHEDULER_ENABLED", "false") == "true" {
		scheduler := ordin.NewScheduler()
		scheduler.Every("heartbeat", time.Minute, func(ctx context.Context) error {
			log.Println("scheduler heartbeat")
			return nil
		}, ordin.RunImmediately())
		go func() {
			if err := scheduler.Start(ctx); err != nil && err != context.Canceled {
				log.Printf("scheduler stopped: %v", err)
			}
		}()
		options = append(options, ordin.WithScheduler(scheduler))
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
