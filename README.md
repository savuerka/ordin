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
- Redis cache/client abstraction
- SFTP upload transport с SHA-256 checksum verification после загрузки
- in-process scheduler для периодических задач
- sequential pipelines для задач отгрузки файлов, ETL и фоновых workflow

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



## Redis cache

ORDIN содержит небольшую `Cache`-абстракцию и Redis backend через `github.com/redis/go-redis/v9`.

```go
cache := ordin.MustRedisCache(ordin.RedisConfig{
    Addr:   "localhost:6379",
    DB:     0,
    Prefix: "ordin:",
})
defer cache.Close()

app := ordin.New(
    ordin.Dev(),
    ordin.WithRedis(cache),
)
```

В handler-е:

```go
app.Post("/cache", func(c *ordin.Context) error {
    if err := c.MustCache().Set(c.Ctx(), "demo:key", "value", 5*time.Minute); err != nil {
        return err
    }

    value, err := c.MustCache().Get(c.Ctx(), "demo:key")
    if err != nil {
        return err
    }

    return c.OK(ordin.Data{"value": value})
})
```

Можно получить низкоуровневый go-redis client:

```go
client := c.MustRedis().Client()
```

Переменные окружения:

```text
REDIS_ADDR=localhost:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0
REDIS_PREFIX=ordin:
REDIS_TLS=false
```

В `examples/basic` Redis включается явно через `REDIS_ENABLED=true`.

## SFTP file transport с checksum verification

SFTP-слой нужен для отгрузки файлов на удалённый хост. После загрузки ORDIN по умолчанию перечитывает удалённый файл и сравнивает SHA-256 контрольную сумму.

```go
sftpClient := ordin.MustSFTPClient(ordin.SFTPConfig{
    Host:                  "localhost",
    Port:                  2222,
    Username:              "ordin",
    Password:              "ordin",
    InsecureIgnoreHostKey: true, // только для локальной разработки
})
defer sftpClient.Close()

app := ordin.New(
    ordin.Dev(),
    ordin.WithSFTP(sftpClient),
)
```

Загрузка файла:

```go
result, err := c.MustSFTP().Upload(
    c.Ctx(),
    "/tmp/report.csv",
    "/upload/reports/report.csv",
    ordin.WithSFTPMkdirAll(),
)
if err != nil {
    return err
}

return c.Created(result)
```

`result.Verified == true` означает, что remote SHA-256 совпал с локальным SHA-256.

Для production лучше использовать `SFTP_KNOWN_HOSTS_PATH`, а не `SFTP_INSECURE_IGNORE_HOST_KEY=true`.

Переменные окружения:

```text
SFTP_HOST=localhost
SFTP_PORT=2222
SFTP_USERNAME=ordin
SFTP_PASSWORD=ordin
SFTP_PRIVATE_KEY_PATH=
SFTP_PRIVATE_KEY_PASSPHRASE=
SFTP_KNOWN_HOSTS_PATH=
SFTP_INSECURE_IGNORE_HOST_KEY=false
SFTP_TIMEOUT=15s
```

В `examples/basic` SFTP включается явно через `SFTP_ENABLED=true`.

## Scheduler

Scheduler — это in-process планировщик. Он хорош для одного worker-процесса, cron-like задач разработки, регулярных отгрузок и maintenance job-ов. В нескольких репликах лучше запускать scheduler только в отдельном worker-е или защищать задачи distributed lock-ом через Redis.

```go
scheduler := ordin.NewScheduler()

scheduler.Every("ship-files", 5*time.Minute, func(ctx context.Context) error {
    // найти файлы, собрать pipeline, отгрузить
    return nil
}, ordin.RunImmediately(), ordin.WithScheduleTimeout(2*time.Minute))

_, err := scheduler.DailyAt("daily-report", "02:30", func(ctx context.Context) error {
    // daily task
    return nil
})
if err != nil {
    panic(err)
}

go func() {
    _ = scheduler.Start(context.Background())
}()

app := ordin.New(
    ordin.Dev(),
    ordin.WithScheduler(scheduler),
)
```

В handler-е можно посмотреть зарегистрированные задачи:

```go
for _, job := range c.MustScheduler().Jobs() {
    fmt.Println(job.Name, job.LastError())
}
```

## Pipelines

Pipeline — это последовательное выполнение шагов с общим `PipelineContext`, retry, timeout и возможностью продолжить выполнение при ошибке отдельного шага.

```go
pipeline := ordin.NewPipeline("file-shipment").
    Use("prepare", func(pc *ordin.PipelineContext) error {
        pc.Set("local_path", "/tmp/report.csv")
        pc.Set("remote_path", "/upload/reports/report.csv")
        return nil
    }).
    Use("upload-sftp", func(pc *ordin.PipelineContext) error {
        result, err := sftpClient.Upload(pc, pc.String("local_path"), pc.String("remote_path"))
        if err != nil {
            return err
        }
        pc.Set("upload", result)
        return nil
    }, ordin.WithStepRetries(2, time.Second), ordin.WithStepTimeout(30*time.Second)).
    Use("publish-event", func(pc *ordin.PipelineContext) error {
        return queue.PublishJSON(pc, "shipments", ordin.Data{
            "type":        "file.shipped",
            "remote_path": pc.String("remote_path"),
        })
    }, ordin.ContinueOnStepError())

result, err := pipeline.Run(context.Background(), ordin.Data{})
if err != nil {
    panic(err)
}

fmt.Println(result.Events)
```

Это основной механизм, через который удобно собирать задачи отгрузки файлов: validate → prepare → upload SFTP/S3 → verify checksum → publish event → cleanup.


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


Дополнительные demo endpoints при включённых сервисах:

```bash
curl -X POST 'http://localhost:8080/demo/cache?key=hello&value=world'

curl -F file=@README.md http://localhost:8080/demo/sftp/upload

curl http://localhost:8080/demo/scheduler/jobs

curl -F file=@README.md http://localhost:8080/demo/pipelines/shipment
```
