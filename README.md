# ORDIN

Минималистичный Laravel-like фреймворк на Go: роутинг, группы, middleware, JSON context, простой ORM/query builder для PostgreSQL и SQL-миграции.

> Это учебный/стартовый каркас, а не production replacement для Laravel, Gin, Echo или GORM.

## Возможности

- `GET`, `POST`, `PUT`, `PATCH`, `DELETE`
- параметры маршрутов: `/users/{id}`
- группы маршрутов: callback-style `Group` и fluent-style `Route`
- global и route-level middleware
- `Context`: `Param`, `ParamInt`, `Query`, `BindJSON`, `Ctx`, `JSON`, `Text`
- короткие ответы: `OK`, `Created`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `NoContent`
- PostgreSQL через `pgx` stdlib
- query builder: `Table`, `Where`, `OrderBy`, `Limit`, `Get`, `First`, `Insert`, `Update`, `Delete`
- generic typed query: `ordin.Query[T](db, "table").All(ctx)`
- CRUD routes через `Resource`
- простые SQL-миграции из папки

## Подключение

```bash
go get github.com/savuerka/ordin
```

Новый короткий импорт:

```go
import "github.com/savuerka/ordin"
```

Старый импорт продолжает работать:

```go
import "github.com/savuerka/ordin/framework"
```

## Минимальный сервер

```go
package main

import "github.com/savuerka/ordin"

func main() {
    app := ordin.New(ordin.Dev())

    app.Get("/", ordin.Text("Hello"))

    app.Get("/users/{id}", func(c *ordin.Context) error {
        return c.OK(map[string]string{"id": c.Param("id")})
    })

    _ = app.Run()
}
```

`Run()` без аргументов слушает `:8080`. Можно передать адрес явно:

```go
_ = app.Run(":3000")
```

## Middleware

```go
func Auth() ordin.Middleware {
    return func(next ordin.HandlerFunc) ordin.HandlerFunc {
        return func(c *ordin.Context) error {
            if c.Header("Authorization") == "" {
                return c.Unauthorized("unauthorized")
            }
            return next(c)
        }
    }
}

admin := app.Route("/admin", Auth())
admin.Get("/dashboard", Dashboard)
```

Callback-style группы тоже поддерживаются:

```go
app.Group("/admin", func(r *ordin.Router) {
    r.Get("/dashboard", Dashboard)
}, Auth())
```

## Resource routes

```go
api := app.Route("/api")
api.Resource("/users", ordin.Resource{
    Index: users.Index,
    Show:  users.Show,
    Store: users.Store,
})
```

Это зарегистрирует:

```text
GET  /api/users
GET  /api/users/{id}
POST /api/users
```

Если указать `Update` и `Delete`, будут добавлены:

```text
PUT    /api/users/{id}
DELETE /api/users/{id}
```

## PostgreSQL ORM/query builder

```go
db, err := ordin.ConnectPostgres("postgres://postgres:postgres@localhost:5432/app?sslmode=disable")
if err != nil {
    panic(err)
}
defer db.Close()

type User struct {
    ID    int    `db:"id,omitempty" json:"id"`
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

## Typed query

```go
users, err := ordin.Query[User](db, "users").
    Where("email LIKE ?", "%@test.com").
    OrderBy("id DESC").
    All(ctx)

user, err := ordin.Query[User](db, "users").
    Where("id = ?", 1).
    First(ctx)
```

## Context helpers

```go
func Show(c *ordin.Context) error {
    id, err := c.ParamInt("id")
    if err != nil {
        return c.BadRequest("invalid id")
    }

    return c.OK(map[string]any{"id": id})
}
```

Generic bind:

```go
user, err := ordin.Bind[User](c)
if err != nil {
    return c.BadRequest(err.Error())
}
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
err := ordin.NewMigrator(db).Run(context.Background(), "migrations")
```

Или коротко:

```go
ordin.MustMigrate(db, "migrations")
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
