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
			writer := &statusResponseWriter{ResponseWriter: c.Writer}
			c.Writer = writer
			err := next(c)
			statusCode := writer.statusCode()
			if err != nil && !writer.wroteHeader {
				statusCode = http.StatusInternalServerError
			}
			log.Printf("%s %s %s %s %d", c.Request.RemoteAddr, c.Request.Method, c.Request.URL.Path, time.Since(started), statusCode)
			return err
		}
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
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
