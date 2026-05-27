package main

import (
	"context"
	"log"
	"os"

	"basic-example/app/controllers"
	"basic-example/routes"

	"github.com/savuerka/ordin/framework"
)

func main() {
	dsn := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/larago?sslmode=disable")
	db, err := framework.ConnectPostgres(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := framework.NewMigrator(db).Run(context.Background(), "migrations"); err != nil {
		log.Fatal(err)
	}

	app := framework.New()
	app.Use(framework.Recover(), framework.Logger())

	routes.Register(app, controllers.UserController{DB: db})

	addr := getenv("APP_ADDR", ":8080")
	log.Printf("listening on %s", addr)
	log.Fatal(app.Listen(addr))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
