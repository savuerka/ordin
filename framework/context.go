package framework

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
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

func (c *Context) Param(name string) string {
	return c.params[name]
}

func (c *Context) Query(name string) string {
	return c.Request.URL.Query().Get(name)
}

func (c *Context) Header(name string) string {
	return c.Request.Header.Get(name)
}

func (c *Context) Status(code int) *Context {
	c.Writer.WriteHeader(code)
	return c
}

// JSON sends a JSON response.
//
// Usage:
//
//	return c.JSON(200, account)
//	return c.JSON(200, account, []string{"password", "token"})
//
// Excluded fields are matched by json tag name first, then by Go struct field name.
func (c *Context) JSON(code int, data any, exclude ...[]string) error {
	payload := data
	if len(exclude) > 0 && len(exclude[0]) > 0 {
		payload = excludeJSONFields(data, exclude[0])
	}

	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	return json.NewEncoder(c.Writer).Encode(payload)
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

func excludeJSONFields(data any, fields []string) any {
	excluded := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		excluded[field] = struct{}{}
	}
	return filterValue(reflect.ValueOf(data), excluded)
}

func filterValue(value reflect.Value, excluded map[string]struct{}) any {
	if !value.IsValid() {
		return nil
	}

	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		return filterStruct(value, excluded)
	case reflect.Slice, reflect.Array:
		items := make([]any, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			items = append(items, filterValue(value.Index(i), excluded))
		}
		return items
	case reflect.Map:
		return filterMap(value, excluded)
	default:
		return value.Interface()
	}
}

func filterStruct(value reflect.Value, excluded map[string]struct{}) map[string]any {
	result := make(map[string]any)
	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		fieldValue := value.Field(i)

		// Skip unexported fields.
		if field.PkgPath != "" {
			continue
		}

		jsonName := jsonFieldName(field)
		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = field.Name
		}

		if _, exists := excluded[jsonName]; exists {
			continue
		}
		if _, exists := excluded[field.Name]; exists {
			continue
		}

		result[jsonName] = filterValue(fieldValue, excluded)
	}

	return result
}

func filterMap(value reflect.Value, excluded map[string]struct{}) any {
	if value.Type().Key().Kind() != reflect.String {
		return value.Interface()
	}

	result := make(map[string]any, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		if _, exists := excluded[key]; exists {
			continue
		}
		result[key] = filterValue(iter.Value(), excluded)
	}
	return result
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	return parts[0]
}
