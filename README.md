# Mini Larago

Минималистичный Laravel-like фреймворк на Go: роутинг, группы, middleware, JSON context, простой ORM/query builder для PostgreSQL и SQL-миграции.

> Это учебный/стартовый каркас, а не production replacement для Laravel, Gin, Echo или GORM.

## Возможности

- `GET`, `POST`, `PUT`, `PATCH`, `DELETE`
- параметры маршрутов: `/users/{id}`
- группы маршрутов: `/api`, `/admin` и т.д.
- global и route-level middleware
- `Context`: `Param`, `Query`, `BindJSON`, `JSON`, `Text`
- PostgreSQL через `pgx` stdlib
- query builder: `Table`, `Where`, `OrderBy`, `Limit`, `Get`, `First`, `Insert`, `Update`, `Delete`
- простые SQL-миграции из папки



## Подключение к своему приложению

```bash
go get github.com/savuerka/ordin
```

И импорт:

```go
import "github.com/savuerka/ordin/framework"
```

## Минимальный сервер

```go
package main

import "github.com/savuerka/ordin/framework"

func main() {
    app := framework.New()
    app.Use(framework.Recover(), framework.Logger())

    app.Get("/", func(c *framework.Context) error {
        return c.Text(200, "Hello")
    })

    app.Get("/users/{id}", func(c *framework.Context) error {
        return c.JSON(200, map[string]string{"id": c.Param("id")})
    })

    _ = app.Listen(":8080")
}
```

## Middleware

```go
func Auth() framework.Middleware {
    return func(next framework.HandlerFunc) framework.HandlerFunc {
        return func(c *framework.Context) error {
            if c.Header("Authorization") == "" {
                return c.JSON(401, map[string]string{"error": "unauthorized"})
            }
            return next(c)
        }
    }
}

app.Group("/admin", func(r *framework.Router) {
    r.Get("/dashboard", Dashboard)
}, Auth())
```

## PostgreSQL ORM/query builder

```go
db, err := framework.ConnectPostgres("postgres://postgres:postgres@localhost:5432/app?sslmode=disable")
if err != nil {
    panic(err)
}
defer db.Close()

type User struct {
    ID    int    `db:"id" json:"id"`
    Name  string `db:"name" json:"name"`
    Email string `db:"email" json:"email"`
}

var users []User
err = db.Table("users").Where("email LIKE ?", "%@test.com").OrderBy("id DESC").Get(ctx, &users)

var user User
err = db.Table("users").Where("id = ?", 1).First(ctx, &user)

err = db.Table("users").Insert(ctx, User{Name: "Alex", Email: "alex@test.com"})

err = db.Table("users").Where("id = ?", 1).Update(ctx, map[string]any{"name": "Alex Updated"})

err = db.Table("users").Where("id = ?", 1).Delete(ctx)
```

## Миграции

Файлы миграций лежат в папке, например:

```text
migrations/
  001_create_users.sql
  002_create_posts.sql
```

Запуск:

```go
err := framework.NewMigrator(db).Run(context.Background(), "migrations")
```

## Запуск примера

```bash
cd examples/basic
docker compose up -d
go mod tidy
go run .
```

Проверка:

```bash
curl http://localhost:8080/

curl -X POST http://localhost:8080/api/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alex","email":"alex@test.com"}'

curl http://localhost:8080/api/users
```
