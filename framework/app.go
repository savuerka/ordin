package framework

import (
	"html/template"
	"net/http"
)

// Option configures an App during creation.
type Option func(*App)

type App struct {
	*Router
}

func New(options ...Option) *App {
	app := &App{Router: NewRouter()}
	for _, option := range options {
		if option != nil {
			option(app)
		}
	}
	return app
}

func (a *App) Listen(addr string) error {
	return http.ListenAndServe(addr, a.Router)
}

// Run starts the HTTP server. If addr is omitted, :8080 is used.
func (a *App) Run(addrs ...string) error {
	addr := ":8080"
	if len(addrs) > 0 && addrs[0] != "" {
		addr = addrs[0]
	}
	return a.Listen(addr)
}

// WithMiddleware registers global middleware during app creation.
func WithMiddleware(middlewares ...Middleware) Option {
	return func(app *App) {
		app.Use(middlewares...)
	}
}

// Dev enables a small development preset: panic recovery + request logging.
func Dev() Option {
	return WithMiddleware(Recover(), Logger())
}

// WithRenderer sets a custom HTML view renderer.
func WithRenderer(renderer Renderer) Option {
	return func(app *App) {
		app.Router.config.renderer = renderer
	}
}

// WithViews configures the default ORDIN Blade-like view engine.
func WithViews(dir string, funcs ...template.FuncMap) Option {
	return func(app *App) {
		app.Router.config.renderer = MustViewEngine(dir, funcs...)
	}
}

// WithStorage sets the default storage service available through Context.Storage().
func WithStorage(storage Storage) Option {
	return func(app *App) {
		app.Router.config.storage = storage
	}
}

// WithQueue sets the default queue service available through Context.Queue().
func WithQueue(queue Queue) Option {
	return func(app *App) {
		app.Router.config.queue = queue
	}
}

// WithCache sets the default cache service available through Context.Cache().
func WithCache(cache Cache) Option {
	return func(app *App) {
		app.Router.config.cache = cache
	}
}

// WithRedis is an alias for WithCache for Redis-backed applications.
func WithRedis(cache Cache) Option {
	return WithCache(cache)
}

// WithSFTP sets the default SFTP transport available through Context.SFTP().
func WithSFTP(transport FileTransport) Option {
	return func(app *App) {
		app.Router.config.sftp = transport
	}
}

// WithScheduler sets the default scheduler available through Context.Scheduler().
func WithScheduler(scheduler *Scheduler) Option {
	return func(app *App) {
		app.Router.config.scheduler = scheduler
	}
}
