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
- HTML views через `c.View(...)`
- Blade-like шаблоны `.ordin.html`: `@extends`, `@section`, `@yield`, `@include`, `@if`, `@foreach`, `{{ value }}`
- S3-compatible storage: MinIO, SeaweedFS S3, AWS S3 и похожие backend-ы
- RabbitMQ queue backend через AMQP 0.9.1

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


## Views / Blade-like templates

ORDIN умеет рендерить обычные Go `html/template` файлы и Blade-like файлы с расширением `.ordin.html`.

Подключение:

```go
app := ordin.New(
    ordin.Dev(),
    ordin.WithViews("resources/views"),
)
```

Роут:

```go
app.Get("/", func(c *ordin.Context) error {
    return c.View("welcome", ordin.Data{
        "title": "ORDIN",
        "user": user,
    })
})
```

Шаблон `resources/views/welcome.ordin.html`:

```blade
@extends("layouts.app")

@section("title")
    {{ title }}
@endsection

@section("content")
    <h1>Hello, {{ user.Name }}</h1>

    @if user.IsAdmin
        <p>Admin mode</p>
    @else
        <p>User mode</p>
    @endif
@endsection
```

Layout `resources/views/layouts/app.ordin.html`:

```blade
<!doctype html>
<html>
<head>
    <title>@yield("title")</title>
</head>
<body>
    @include("partials.nav")

    <main>
        @yield("content")
    </main>
</body>
</html>
```

Поддерживается:

```blade
{{ value }}              # escaped output
{!! trustedHTML !!}      # raw HTML, использовать аккуратно
@extends("layouts.app")
@section("content") ... @endsection
@yield("content")
@include("partials.nav")
@if condition ... @else ... @endif
@foreach items as item ... @endforeach
```

Внутри это компилируется в `html/template`, поэтому обычный `{{ value }}` экранируется безопасно по умолчанию.

## Storage: MinIO / SeaweedFS / S3

ORDIN содержит небольшую абстракцию `Storage` и S3-compatible реализацию. Она подходит для MinIO, SeaweedFS S3 API, AWS S3, Garage и других совместимых backend-ов.

```go
storage := ordin.MustS3Storage(ordin.S3Config{
    Endpoint:        "localhost:9000",
    AccessKeyID:     "minioadmin",
    SecretAccessKey: "minioadmin",
    Bucket:          "ordin",
    Region:          "us-east-1",
    Secure:          false,
    CreateBucket:    true,
})

app := ordin.New(
    ordin.Dev(),
    ordin.WithStorage(storage),
)
```

В handler-е:

```go
app.Post("/upload", func(c *ordin.Context) error {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        return c.BadRequest(err.Error())
    }
    defer file.Close()

    key := "uploads/" + header.Filename
    if err := c.MustStorage().Put(c.Ctx(), key, file, header.Size, ordin.WithContentType(header.Header.Get("Content-Type"))); err != nil {
        return err
    }

    url, err := c.MustStorage().URL(c.Ctx(), key, 15*time.Minute)
    if err != nil {
        return err
    }

    return c.Created(ordin.Data{
        "key": key,
        "url": url,
    })
})
```

Можно читать конфигурацию из окружения:

```go
storage := ordin.MustS3Storage(ordin.S3ConfigFromEnv("S3"))
```

Для префикса `S3` используются переменные:

```text
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY_ID=minioadmin
S3_SECRET_ACCESS_KEY=minioadmin
S3_BUCKET=ordin
S3_REGION=us-east-1
S3_SECURE=false
S3_CREATE_BUCKET=true
```

Для SeaweedFS обычно достаточно поменять endpoint, например:

```text
S3_ENDPOINT=localhost:8333
```

В `examples/basic` storage включается явно через `S3_ENABLED=true`. Без этого demo-route `/demo/upload` вернёт `503`, чтобы приложение не падало, если MinIO/SeaweedFS не запущен.

## Queues: RabbitMQ

ORDIN содержит небольшую абстракцию `Queue` и RabbitMQ backend.

```go
queue := ordin.MustRabbitQueue(ordin.RabbitMQConfig{
    URL: "amqp://guest:guest@localhost:5672/",
})
defer queue.Close()

app := ordin.New(
    ordin.Dev(),
    ordin.WithQueue(queue),
)
```

Публикация job/message из handler-а:

```go
app.Post("/emails/welcome", func(c *ordin.Context) error {
    err := c.MustQueue().PublishJSON(c.Ctx(), "emails", ordin.Data{
        "type":  "welcome",
        "email": "user@example.com",
    })
    if err != nil {
        return err
    }

    return c.Created(ordin.Data{"queued": true})
})
```

Worker:

```go
func main() {
    queue := ordin.MustRabbitQueue(ordin.RabbitMQConfigFromEnv("RABBITMQ"))
    defer queue.Close()

    err := queue.Consume(context.Background(), "emails", func(ctx context.Context, job ordin.Job) error {
        var payload struct {
            Type  string `json:"type"`
            Email string `json:"email"`
        }

        if err := job.DecodeJSON(&payload); err != nil {
            return err
        }

        // send email, generate report, resize image, etc.
        return nil
    }, ordin.WithPrefetch(5))

    if err != nil {
        panic(err)
    }
}
```

Переменная окружения по умолчанию:

```text
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
```

В `examples/basic` очередь включается явно через `RABBITMQ_ENABLED=true`. Без этого demo-route `/demo/jobs/welcome` вернёт `503`, чтобы базовый пример запускался без RabbitMQ.

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
