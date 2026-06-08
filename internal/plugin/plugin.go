package plugin

import "net/http"

type Plugin interface {
	Name() string
	Wrap(http.Handler) http.Handler
}

type Registry struct {
	plugins map[string]Plugin
}

func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

func (r *Registry) Register(plugin Plugin) {
	if r == nil || plugin == nil || plugin.Name() == "" {
		return
	}
	r.plugins[plugin.Name()] = plugin
}

func (r *Registry) Get(name string) (Plugin, bool) {
	if r == nil {
		return nil, false
	}
	plugin, ok := r.plugins[name]
	return plugin, ok
}

func Chain(handler http.Handler, plugins ...Plugin) http.Handler {
	for i := len(plugins) - 1; i >= 0; i-- {
		if plugins[i] != nil {
			handler = plugins[i].Wrap(handler)
		}
	}
	return handler
}

type MiddlewarePlugin struct {
	PluginName string
	Middleware func(http.Handler) http.Handler
}

func (p MiddlewarePlugin) Name() string {
	return p.PluginName
}

func (p MiddlewarePlugin) Wrap(next http.Handler) http.Handler {
	if p.Middleware == nil {
		return next
	}
	return p.Middleware(next)
}
