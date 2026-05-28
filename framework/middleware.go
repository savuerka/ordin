package framework

import (
	"log"
	"net/http"
	"time"
)

type HandlerFunc func(*Context) error

type Middleware func(HandlerFunc) HandlerFunc

func chain(h HandlerFunc, middlewares ...Middleware) HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func Logger() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			started := time.Now()
			err := next(c)
			log.Printf("%s %s %s", c.Request.Method, c.Request.URL.Path, time.Since(started))
			return err
		}
	}
}

func Recover() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic recovered: %v", r)
					err = c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				}
			}()
			return next(c)
		}
	}
}
