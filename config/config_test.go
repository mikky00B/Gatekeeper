package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("GATEWAY_ADDR", "")
	t.Setenv("GATEWAY_UPSTREAM", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Address != defaultAddress {
		t.Fatalf("address = %q, want %q", cfg.Server.Address, defaultAddress)
	}
	if cfg.Proxy.Upstream != defaultUpstream {
		t.Fatalf("upstream = %q, want %q", cfg.Proxy.Upstream, defaultUpstream)
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("routes length = %d, want 1", len(cfg.Routes))
	}
	if cfg.Routes[0].PathPrefix != "/" {
		t.Fatalf("default route path prefix = %q, want /", cfg.Routes[0].PathPrefix)
	}
}

func TestLoadReadsConfigFile(t *testing.T) {
	t.Setenv("GATEWAY_ADDR", "")
	t.Setenv("GATEWAY_UPSTREAM", "")

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`server:
  address: ":9000"
proxy:
  upstream: "http://example.test"
database:
  driver: "postgres"
  dsn: "postgres://gateway:gateway@localhost:5432/gateway?sslmode=disable"
jwt:
  secret: "jwt-secret"
  issuer: "gateway"
  audience: "api"
analytics:
  enabled: true
routes:
  - id: "api"
    path_prefix: "/api"
    methods: "GET,POST"
    upstream: "http://api.example.test"
    strip_prefix: true
    require_auth: true
    require_jwt: true
    rate_limit_capacity: 10
    rate_limit_refill_per_second: 2.5
api_keys:
  - id: "dev"
    key: "secret"
    tenant: "local"
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Address != ":9000" {
		t.Fatalf("address = %q, want :9000", cfg.Server.Address)
	}
	if cfg.Proxy.Upstream != "http://example.test" {
		t.Fatalf("upstream = %q, want http://example.test", cfg.Proxy.Upstream)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("database driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("database dsn is empty")
	}
	if cfg.JWT.Secret != "jwt-secret" {
		t.Fatalf("jwt secret = %q, want jwt-secret", cfg.JWT.Secret)
	}
	if cfg.JWT.Issuer != "gateway" {
		t.Fatalf("jwt issuer = %q, want gateway", cfg.JWT.Issuer)
	}
	if cfg.JWT.Audience != "api" {
		t.Fatalf("jwt audience = %q, want api", cfg.JWT.Audience)
	}
	if !cfg.Analytics.Enabled {
		t.Fatal("analytics enabled = false, want true")
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("routes length = %d, want 1", len(cfg.Routes))
	}
	if cfg.Routes[0].ID != "api" {
		t.Fatalf("route id = %q, want api", cfg.Routes[0].ID)
	}
	if cfg.Routes[0].PathPrefix != "/api" {
		t.Fatalf("route path prefix = %q, want /api", cfg.Routes[0].PathPrefix)
	}
	if len(cfg.Routes[0].Methods) != 2 || cfg.Routes[0].Methods[0] != "GET" || cfg.Routes[0].Methods[1] != "POST" {
		t.Fatalf("route methods = %#v, want GET and POST", cfg.Routes[0].Methods)
	}
	if cfg.Routes[0].Upstream != "http://api.example.test" {
		t.Fatalf("route upstream = %q, want http://api.example.test", cfg.Routes[0].Upstream)
	}
	if !cfg.Routes[0].StripPrefix {
		t.Fatal("route strip prefix = false, want true")
	}
	if !cfg.Routes[0].RequireAuth {
		t.Fatal("route require auth = false, want true")
	}
	if !cfg.Routes[0].RequireJWT {
		t.Fatal("route require jwt = false, want true")
	}
	if cfg.Routes[0].RateLimitCapacity != 10 {
		t.Fatalf("route rate limit capacity = %d, want 10", cfg.Routes[0].RateLimitCapacity)
	}
	if cfg.Routes[0].RateLimitRefillPerSec != 2.5 {
		t.Fatalf("route rate limit refill = %f, want 2.5", cfg.Routes[0].RateLimitRefillPerSec)
	}
	if len(cfg.APIKeys) != 1 {
		t.Fatalf("api keys length = %d, want 1", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0].ID != "dev" {
		t.Fatalf("api key id = %q, want dev", cfg.APIKeys[0].ID)
	}
	if cfg.APIKeys[0].Key != "secret" {
		t.Fatalf("api key value = %q, want secret", cfg.APIKeys[0].Key)
	}
	if cfg.APIKeys[0].Tenant != "local" {
		t.Fatalf("api key tenant = %q, want local", cfg.APIKeys[0].Tenant)
	}
}

func TestLoadEnvironmentOverridesFile(t *testing.T) {
	t.Setenv("GATEWAY_ADDR", ":7000")
	t.Setenv("GATEWAY_UPSTREAM", "http://env.example")

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`server:
  address: ":9000"
proxy:
  upstream: "http://file.example"
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Address != ":7000" {
		t.Fatalf("address = %q, want :7000", cfg.Server.Address)
	}
	if cfg.Proxy.Upstream != "http://env.example" {
		t.Fatalf("upstream = %q, want http://env.example", cfg.Proxy.Upstream)
	}
}

func TestLoadReadsProductionFeatures(t *testing.T) {
	t.Setenv("GATEWAY_ADMIN_TOKEN", "admin-secret")

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`server:
  address: ":9443"
  shutdown_timeout: 5s
  tls:
    enabled: true
    cert_file: "/tmp/cert.pem"
    key_file: "/tmp/key.pem"
database:
  enabled: true
  driver: "postgres"
  dsn: "postgres://gateway:gateway@localhost:5432/gateway?sslmode=disable"
redis:
  enabled: true
  address: "localhost:6379"
admin:
  enabled: true
  token_env: "GATEWAY_ADMIN_TOKEN"
hot_reload:
  enabled: true
  interval: 3s
analytics:
  enabled: true
  persistent: true
  window: 30m
routes:
  - id: "api"
    path_prefix: "/api"
    methods:
      - GET
      - POST
    upstreams:
      - "http://api-a.example.test"
      - "http://api-b.example.test"
    health_check:
      enabled: true
      path: "/ready"
      interval: 2s
      timeout: 500ms
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !cfg.Server.TLS.Enabled {
		t.Fatal("tls enabled = false, want true")
	}
	if !cfg.Database.Enabled {
		t.Fatal("database enabled = false, want true")
	}
	if !cfg.Redis.Enabled {
		t.Fatal("redis enabled = false, want true")
	}
	if cfg.Admin.Token != "admin-secret" {
		t.Fatalf("admin token = %q, want admin-secret", cfg.Admin.Token)
	}
	if !cfg.Analytics.Persistent {
		t.Fatal("analytics persistent = false, want true")
	}
	if len(cfg.Routes[0].Upstreams) != 2 {
		t.Fatalf("upstream count = %d, want 2", len(cfg.Routes[0].Upstreams))
	}
	if cfg.Routes[0].HealthCheck.Path != "/ready" {
		t.Fatalf("health path = %q, want /ready", cfg.Routes[0].HealthCheck.Path)
	}
}
