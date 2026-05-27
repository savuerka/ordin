package framework

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
	params  map[string]string
}

func newContext(w http.ResponseWriter, r *http.Request, params map[string]string) *Context {
	if params == nil {
		params = map[string]string{}
	}
	return &Context{Writer: w, Request: r, params: params}
}

func (c *Context) Param(name string) string { return c.params[name] }

func (c *Context) Query(name string) string { return c.Request.URL.Query().Get(name) }

func (c *Context) Header(name string) string { return c.Request.Header.Get(name) }

func (c *Context) Status(code int) *Context {
	c.Writer.WriteHeader(code)
	return c
}

func (c *Context) JSON(code int, data any) error {
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	return json.NewEncoder(c.Writer).Encode(data)
}

func (c *Context) Text(code int, text string) error {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	_, err := c.Writer.Write([]byte(text))
	return err
}

func (c *Context) BindJSON(dst any) error {
	if c.Request.Body == nil {
		return errors.New("request body is empty")
	}
	defer c.Request.Body.Close()
	return json.NewDecoder(c.Request.Body).Decode(dst)
}
