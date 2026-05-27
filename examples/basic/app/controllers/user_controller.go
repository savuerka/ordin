package controllers

import (
	"net/http"
	"strconv"

	"basic-example/app/models"

	"github.com/savuerka/ordin/framework"
)

type UserController struct{ DB *framework.DB }

func (uc UserController) Index(c *framework.Context) error {
	var users []models.User
	if err := uc.DB.Table("users").OrderBy("id DESC").Get(c.Request.Context(), &users); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

func (uc UserController) Show(c *framework.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User
	if err := uc.DB.Table("users").Where("id = ?", id).First(c.Request.Context(), &user); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	return c.JSON(http.StatusOK, user)
}

func (uc UserController) Store(c *framework.Context) error {
	var user models.User
	if err := c.BindJSON(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := uc.DB.Table("users").Insert(c.Request.Context(), user); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, user)
}
