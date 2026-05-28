package routes

import (
	"basic-example/app/controllers"

	"github.com/savuerka/ordin"
)

type Endpoint struct {
	Method string
	Path   string
}

func Register(app *ordin.App, users controllers.UserController, services controllers.ServiceController) {
	app.Get("/", func(c *ordin.Context) error {
		return c.View("welcome", ordin.Data{
			"title":         "ORDIN",
			"heading":       "ORDIN is running",
			"description":   "Теперь с Blade-like шаблонами поверх html/template.",
			"showEndpoints": true,
			"endpoints": []Endpoint{
				{Method: "GET", Path: "/api/users"},
				{Method: "GET", Path: "/api/users/{id}"},
				{Method: "POST", Path: "/api/users"},
				{Method: "POST", Path: "/demo/upload"},
				{Method: "POST", Path: "/demo/jobs/welcome"},
			},
		})
	})

	api := app.Route("/api")
	api.Resource("/users", ordin.Resource{
		Index: users.Index,
		Show:  users.Show,
		Store: users.Store,
	})

	RegisterServiceRoutes(app, services)
}

func RegisterServiceRoutes(app *ordin.App, services controllers.ServiceController) {
	app.Post("/demo/upload", services.Upload)
	app.Post("/demo/jobs/welcome", services.QueueWelcome)
}
