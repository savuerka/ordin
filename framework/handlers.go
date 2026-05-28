package framework

import "net/http"

// Text returns a handler that responds with 200 text/plain.
func Text(text string) HandlerFunc {
	return func(c *Context) error {
		return c.Text(http.StatusOK, text)
	}
}

// JSON returns a handler that responds with 200 application/json.
func JSON(data any) HandlerFunc {
	return func(c *Context) error {
		return c.JSON(http.StatusOK, data)
	}
}
