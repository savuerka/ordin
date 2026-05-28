package framework

import (
	"context"
	"net/http"
	"strconv"
)

// Ctx returns the request context.
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// ParamInt reads a route parameter and converts it to int.
func (c *Context) ParamInt(name string) (int, error) {
	return strconv.Atoi(c.Param(name))
}

func (c *Context) OK(data any) error {
	return c.JSON(http.StatusOK, data)
}

func (c *Context) Created(data any) error {
	return c.JSON(http.StatusCreated, data)
}

func (c *Context) NoContent() error {
	c.Writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (c *Context) Error(code int, message string) error {
	return c.JSON(code, map[string]string{"error": message})
}

func (c *Context) BadRequest(message string) error {
	return c.Error(http.StatusBadRequest, message)
}

func (c *Context) Unauthorized(message string) error {
	return c.Error(http.StatusUnauthorized, message)
}

func (c *Context) Forbidden(message string) error {
	return c.Error(http.StatusForbidden, message)
}

func (c *Context) NotFound(message string) error {
	return c.Error(http.StatusNotFound, message)
}

// Bind decodes JSON body into T and returns the value.
func Bind[T any](c *Context) (T, error) {
	var value T
	err := c.BindJSON(&value)
	return value, err
}
