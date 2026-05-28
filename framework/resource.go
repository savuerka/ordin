package framework

// Resource describes conventional CRUD handlers.
type Resource struct {
	Index  HandlerFunc
	Show   HandlerFunc
	Store  HandlerFunc
	Update HandlerFunc
	Delete HandlerFunc
}

// Resource registers conventional CRUD routes for a resource path.
func (r *Router) Resource(path string, resource Resource, middlewares ...Middleware) {
	if resource.Index != nil {
		r.Get(path, resource.Index, middlewares...)
	}
	if resource.Show != nil {
		r.Get(joinPath(path, "{id}"), resource.Show, middlewares...)
	}
	if resource.Store != nil {
		r.Post(path, resource.Store, middlewares...)
	}
	if resource.Update != nil {
		r.Put(joinPath(path, "{id}"), resource.Update, middlewares...)
	}
	if resource.Delete != nil {
		r.Delete(joinPath(path, "{id}"), resource.Delete, middlewares...)
	}
}
