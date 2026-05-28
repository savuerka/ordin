package framework

import "net/http"

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
