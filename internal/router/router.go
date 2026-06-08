package router

import (
	"net/http"
	"strings"

	"goproxy/internal/requestctx"
)

type Route struct {
	ID          string
	PathPrefix  string
	Methods     []string
	StripPrefix bool
	Handler     http.Handler
}

type Router struct {
	routes []Route
}

func New(routes []Route) *Router {
	copied := make([]Route, len(routes))
	copy(copied, routes)

	return &Router{routes: copied}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route, ok := r.match(req)
	if !ok {
		http.NotFound(w, req)
		return
	}

	if fields, ok := requestctx.LogFieldsFromContext(req.Context()); ok {
		fields.RouteID = route.ID
	}

	if route.StripPrefix && route.PathPrefix != "/" {
		cloned := req.Clone(req.Context())
		cloned.Header.Set("X-Forwarded-Prefix", strings.TrimRight(route.PathPrefix, "/"))
		cloned.Header.Set("X-Forwarded-Uri", req.URL.RequestURI())
		cloned.URL.Path = strings.TrimPrefix(req.URL.Path, route.PathPrefix)
		if cloned.URL.Path == "" {
			cloned.URL.Path = "/"
		}
		route.Handler.ServeHTTP(w, cloned)
		return
	}

	route.Handler.ServeHTTP(w, req)
}

func (r *Router) match(req *http.Request) (Route, bool) {
	var matched Route
	found := false

	for _, route := range r.routes {
		if !pathMatches(req.URL.Path, route.PathPrefix) || !methodMatches(req.Method, route.Methods) {
			continue
		}

		if !found || len(route.PathPrefix) > len(matched.PathPrefix) {
			matched = route
			found = true
		}
	}

	return matched, found
}

func pathMatches(path string, prefix string) bool {
	if prefix == "" || prefix == "/" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/")
}

func methodMatches(method string, methods []string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, allowed := range methods {
		if strings.EqualFold(method, allowed) {
			return true
		}
	}
	return false
}
