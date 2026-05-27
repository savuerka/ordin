package framework

import "net/http"

type App struct{ *Router }

func New() *App { return &App{Router: NewRouter()} }

func (a *App) Listen(addr string) error { return http.ListenAndServe(addr, a.Router) }
