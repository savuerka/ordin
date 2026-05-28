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

	app := ordin.New(
		ordin.Dev(),
		ordin.WithViews("resources/views"),
	)
	routes.Register(app, controllers.UserController{DB: db})

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
