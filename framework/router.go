package framework

import (
	"net/http"
	"strings"
)

type route struct {
	method      string
	pattern     string
	parts       []string
	handler     HandlerFunc
	middlewares []Middleware
}

type routerConfig struct {
	renderer  Renderer
	storage   Storage
	queue     Queue
	cache     Cache
	sftp      FileTransport
	scheduler *Scheduler
}

type Router struct {
	prefix      string
	routes      *[]route
	middlewares []Middleware
	config      *routerConfig
}

func NewRouter() *Router {
	routes := make([]route, 0)
	return &Router{routes: &routes, config: &routerConfig{}}
}

func (r *Router) Use(middlewares ...Middleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

// Route returns a fluent route group. Routes added to it are registered on the same router.
func (r *Router) Route(prefix string, middlewares ...Middleware) *Router {
	return &Router{
		prefix:      joinPath(r.prefix, prefix),
		routes:      r.routes,
		middlewares: appendMiddlewares(r.middlewares, middlewares...),
		config:      r.config,
	}
}

// Group keeps the callback style API for backward compatibility.
func (r *Router) Group(prefix string, fn func(*Router), middlewares ...Middleware) {
	fn(r.Route(prefix, middlewares...))
}

func (r *Router) Get(path string, h HandlerFunc, middlewares ...Middleware) {
	r.add(http.MethodGet, path, h, middlewares...)
}

func (r *Router) Post(path string, h HandlerFunc, middlewares ...Middleware) {
	r.add(http.MethodPost, path, h, middlewares...)
}

func (r *Router) Put(path string, h HandlerFunc, middlewares ...Middleware) {
	r.add(http.MethodPut, path, h, middlewares...)
}

func (r *Router) Patch(path string, h HandlerFunc, middlewares ...Middleware) {
	r.add(http.MethodPatch, path, h, middlewares...)
}

func (r *Router) Delete(path string, h HandlerFunc, middlewares ...Middleware) {
	r.add(http.MethodDelete, path, h, middlewares...)
}

// Handle accepts a compact pattern such as "GET /users/{id}".
func (r *Router) Handle(pattern string, h HandlerFunc, middlewares ...Middleware) {
	method, path, ok := strings.Cut(strings.TrimSpace(pattern), " ")
	if !ok {
		r.add(http.MethodGet, pattern, h, middlewares...)
		return
	}
	r.add(strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(path), h, middlewares...)
}

func (r *Router) add(method, path string, h HandlerFunc, middlewares ...Middleware) {
	full := joinPath(r.prefix, path)
	mw := appendMiddlewares(r.middlewares, middlewares...)
	*r.routes = append(*r.routes, route{
		method:      method,
		pattern:     full,
		parts:       splitPath(full),
		handler:     h,
		middlewares: mw,
	})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range *r.routes {
		if rt.method != req.Method {
			continue
		}

		params, ok := match(rt.parts, splitPath(req.URL.Path))
		if !ok {
			continue
		}

		ctx := newContext(w, req, params, r.config)
		if err := chain(rt.handler, rt.middlewares...)(ctx); err != nil {
			_ = ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}

	http.NotFound(w, req)
}

func appendMiddlewares(base []Middleware, extra ...Middleware) []Middleware {
	middlewares := append([]Middleware{}, base...)
	middlewares = append(middlewares, extra...)
	return middlewares
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func match(patternParts, requestParts []string) (map[string]string, bool) {
	if len(patternParts) != len(requestParts) {
		return nil, false
	}

	params := map[string]string{}
	for i := range patternParts {
		pp, rp := patternParts[i], requestParts[i]
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			params[strings.TrimSuffix(strings.TrimPrefix(pp, "{"), "}")] = rp
			continue
		}
		if pp != rp {
			return nil, false
		}
	}

	return params, true
}

func joinPath(a, b string) string {
	joined := "/" + strings.Trim(strings.TrimRight(a, "/")+"/"+strings.TrimLeft(b, "/"), "/")
	if joined == "/" {
		return "/"
	}
	return strings.ReplaceAll(joined, "//", "/")
}
