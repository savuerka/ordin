package ordin

import (
	"html/template"

	"github.com/savuerka/ordin/framework"
)

type App = framework.App
type Router = framework.Router
type Context = framework.Context
type DB = framework.DB
type QueryBuilder = framework.QueryBuilder
type Migrator = framework.Migrator
type HandlerFunc = framework.HandlerFunc
type Middleware = framework.Middleware
type Option = framework.Option
type Resource = framework.Resource
type Data = framework.Data
type Renderer = framework.Renderer
type ViewEngine = framework.ViewEngine

func New(options ...Option) *App {
	return framework.New(options...)
}

func NewRouter() *Router {
	return framework.NewRouter()
}

func WithMiddleware(middlewares ...Middleware) Option {
	return framework.WithMiddleware(middlewares...)
}

func WithRenderer(renderer Renderer) Option {
	return framework.WithRenderer(renderer)
}

func WithViews(dir string, funcs ...template.FuncMap) Option {
	return framework.WithViews(dir, funcs...)
}

func NewViewEngine(dir string, funcs ...template.FuncMap) (*ViewEngine, error) {
	return framework.NewViewEngine(dir, funcs...)
}

func MustViewEngine(dir string, funcs ...template.FuncMap) *ViewEngine {
	return framework.MustViewEngine(dir, funcs...)
}

func Dev() Option {
	return framework.Dev()
}

func Logger() Middleware {
	return framework.Logger()
}

func Recover() Middleware {
	return framework.Recover()
}

func Text(text string) HandlerFunc {
	return framework.Text(text)
}

func JSON(data any) HandlerFunc {
	return framework.JSON(data)
}

func Bind[T any](c *Context) (T, error) {
	return framework.Bind[T](c)
}

func ConnectPostgres(dsn string) (*DB, error) {
	return framework.ConnectPostgres(dsn)
}

func MustPostgres(dsn string) *DB {
	return framework.MustPostgres(dsn)
}

func MustPostgresEnv(key, fallback string) *DB {
	return framework.MustPostgresEnv(key, fallback)
}

func NewMigrator(db *DB) *Migrator {
	return framework.NewMigrator(db)
}

func MustMigrate(db *DB, dir string) {
	framework.MustMigrate(db, dir)
}

func Query[T any](db *DB, table string) *framework.TypedQuery[T] {
	return framework.Query[T](db, table)
}

func Repo[T any](db *DB, table string) *framework.TypedQuery[T] {
	return framework.Repo[T](db, table)
}
