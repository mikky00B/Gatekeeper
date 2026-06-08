package proxy

import "net/http"

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares in order: first = outermost (runs first)
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	// Apply in reverse so the first middleware in the list runs first
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
