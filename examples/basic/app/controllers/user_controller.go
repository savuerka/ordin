package controllers

import (
	"basic-example/app/models"

	"github.com/savuerka/ordin"
)

type UserController struct {
	DB *ordin.DB
}

func (uc UserController) Index(c *ordin.Context) error {
	users, err := ordin.Query[models.User](uc.DB, "users").
		OrderBy("id DESC").
		All(c.Ctx())
	if err != nil {
		return err
	}

	return c.OK(users)
}

func (uc UserController) Show(c *ordin.Context) error {
	id, err := c.ParamInt("id")
	if err != nil {
		return c.BadRequest("invalid id")
	}

	user, err := ordin.Query[models.User](uc.DB, "users").
		Where("id = ?", id).
		First(c.Ctx())
	if err != nil {
		return c.NotFound("user not found")
	}

	return c.OK(user)
}

func (uc UserController) Store(c *ordin.Context) error {
	user, err := ordin.Bind[models.User](c)
	if err != nil {
		return c.BadRequest(err.Error())
	}

	if err := ordin.Query[models.User](uc.DB, "users").Create(c.Ctx(), user); err != nil {
		return err
	}

	return c.Created(user)
}
