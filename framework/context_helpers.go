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

// Storage returns the configured object storage service.
func (c *Context) Storage() Storage {
	return c.storage
}

// MustStorage returns the configured object storage service or panics.
func (c *Context) MustStorage() Storage {
	if c.storage == nil {
		panic(storageNotConfiguredError())
	}
	return c.storage
}

// Queue returns the configured queue service.
func (c *Context) Queue() Queue {
	return c.queue
}

// MustQueue returns the configured queue service or panics.
func (c *Context) MustQueue() Queue {
	if c.queue == nil {
		panic("queue is not configured")
	}
	return c.queue
}

// Cache returns the configured cache service.
func (c *Context) Cache() Cache {
	return c.cache
}

// MustCache returns the configured cache service or panics.
func (c *Context) MustCache() Cache {
	if c.cache == nil {
		panic("cache is not configured")
	}
	return c.cache
}

// Redis returns the configured Redis-backed cache service when available.
func (c *Context) Redis() *RedisCache {
	cache, _ := c.cache.(*RedisCache)
	return cache
}

// MustRedis returns the configured Redis-backed cache service or panics.
func (c *Context) MustRedis() *RedisCache {
	cache := c.Redis()
	if cache == nil {
		panic("redis cache is not configured")
	}
	return cache
}

// SFTP returns the configured file transport service.
func (c *Context) SFTP() FileTransport {
	return c.sftp
}

// MustSFTP returns the configured file transport service or panics.
func (c *Context) MustSFTP() FileTransport {
	if c.sftp == nil {
		panic("sftp transport is not configured")
	}
	return c.sftp
}

// Scheduler returns the configured in-process scheduler.
func (c *Context) Scheduler() *Scheduler {
	return c.scheduler
}

// MustScheduler returns the configured scheduler or panics.
func (c *Context) MustScheduler() *Scheduler {
	if c.scheduler == nil {
		panic("scheduler is not configured")
	}
	return c.scheduler
}

// View renders a configured HTML view with HTTP 200.
func (c *Context) View(name string, data any) error {
	return c.ViewStatus(http.StatusOK, name, data)
}

// ViewStatus renders a configured HTML view with a custom HTTP status.
func (c *Context) ViewStatus(code int, name string, data any) error {
	if c.renderer == nil {
		return c.Error(http.StatusInternalServerError, "view renderer is not configured")
	}
	return c.renderer.Render(c.Writer, code, name, data)
}

// Bind decodes JSON body into T and returns the value.
func Bind[T any](c *Context) (T, error) {
	var value T
	err := c.BindJSON(&value)
	return value, err
}
