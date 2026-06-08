package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"goproxy/config"
	"goproxy/internal/proxy"
)

type ReloadFunc func() error

type Handler struct {
	token     string
	routes    func() []config.RouteConfig
	upstreams func() map[string][]proxy.UpstreamStatus
	reload    ReloadFunc
}

func NewHandler(token string, routes func() []config.RouteConfig, upstreams func() map[string][]proxy.UpstreamStatus, reload ReloadFunc) *Handler {
	return &Handler{
		token:     token,
		routes:    routes,
		upstreams: upstreams,
		reload:    reload,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/admin/routes":
		writeJSON(w, h.routes())
	case r.Method == http.MethodGet && r.URL.Path == "/admin/upstreams":
		writeJSON(w, h.upstreams())
	case r.Method == http.MethodPost && r.URL.Path == "/admin/reload":
		if h.reload == nil {
			http.Error(w, "reload unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := h.reload(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "reloaded"})
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) authorized(r *http.Request) bool {
	if h == nil || h.token == "" {
		return false
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[len("bearer "):]) == h.token
	}
	return strings.TrimSpace(r.Header.Get("X-Admin-Token")) == h.token
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
