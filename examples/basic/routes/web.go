package routes

import (
	"basic-example/app/controllers"

	"github.com/savuerka/ordin"
)

func Register(app *ordin.App, users controllers.UserController) {
	app.Get("/", ordin.Text("ORDIN is running"))

	api := app.Route("/api")
	api.Resource("/users", ordin.Resource{
		Index: users.Index,
		Show:  users.Show,
		Store: users.Store,
	})
}
