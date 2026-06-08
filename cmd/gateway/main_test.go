package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"goproxy/config"
)

func TestGatewayHealthz(t *testing.T) {
	handler, err := newHandler(config.Config{
		Server: config.ServerConfig{Address: ":0"},
		Proxy:  config.ProxyConfig{Upstream: "http://example.test"},
	})
	if err != nil {
		t.Fatalf("newHandler returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "ok\n" {
		t.Fatalf("body = %q, want ok", string(body))
	}
}

func TestGatewayProxiesFallbackRoutes(t *testing.T) {
	_, err := newHandler(config.Config{
		Server: config.ServerConfig{Address: ":0"},
		Proxy:  config.ProxyConfig{Upstream: "localhost:3000"},
	})
	if err == nil {
		t.Fatal("newHandler returned nil error for invalid upstream")
	}
}

func TestGatewayProtectsAuthenticatedRoutes(t *testing.T) {
	handler, err := newHandler(config.Config{
		Server: config.ServerConfig{Address: ":0"},
		Proxy:  config.ProxyConfig{Upstream: "http://example.test"},
		Routes: []config.RouteConfig{
			{
				ID:          "private",
				PathPrefix:  "/private",
				Upstream:    "http://example.test",
				RequireAuth: true,
			},
		},
		APIKeys: []config.APIKeyConfig{
			{ID: "dev", Key: "secret", Tenant: "local"},
		},
	})
	if err != nil {
		t.Fatalf("newHandler returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/private", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGatewayExposesAnalyticsWhenEnabled(t *testing.T) {
	handler, err := newHandler(config.Config{
		Server:    config.ServerConfig{Address: ":0"},
		Proxy:     config.ProxyConfig{Upstream: "http://example.test"},
		Analytics: config.AnalyticsConfig{Enabled: true},
	})
	if err != nil {
		t.Fatalf("newHandler returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/analytics/routes", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
}
