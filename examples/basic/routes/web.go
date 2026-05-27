package routes

import (
	"basic-example/app/controllers"

	"github.com/savuerka/ordin/framework"
)

func Register(app *framework.App, users controllers.UserController) {
	app.Get("/", func(c *framework.Context) error { return c.Text(200, "Mini Larago is running") })

	app.Group("/api", func(r *framework.Router) {
		r.Get("/users", users.Index)
		r.Get("/users/{id}", users.Show)
		r.Post("/users", users.Store)
	})
}
