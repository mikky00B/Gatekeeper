# goproxy

A self-hosted API gateway in Go. It provides config-driven reverse proxy routing, multiple upstreams per route, round-robin load balancing, optional upstream health checks, API key authentication, optional HS256 JWT verification, in-memory or Redis-backed rate limiting, request logging, route-level analytics, hot config reload, TLS, an admin API, a static dashboard, and CLI helpers.

## Run Locally

Create a local config from the example, start any backend on port 3000, then run:

```sh
cp config/exampleconfig.yaml config/config.yaml
go run ./cmd/gateway
```

The gateway reads `config/config.yaml` by default and listens on `:8080`.

Useful endpoints:

- `GET /healthz`
- `GET /analytics/routes` when `analytics.enabled` is `true`
- `GET /dashboard/`

## Multi-Upstream Routes

Each route can target one upstream or a pool of upstreams:

```yaml
routes:
  - id: "users"
    path_prefix: "/users"
    strip_prefix: true
    upstreams:
      - "http://users-a:3000"
      - "http://users-b:3000"
    health_check:
      enabled: true
      path: "/healthz"
      interval: 5s
      timeout: 2s
```

Requests are sent to healthy upstreams by round robin. If every upstream in a route is unhealthy, the gateway returns `502`.

## Auth And Rate Limits

Per-route API key auth uses the `X-API-Key` header:

```yaml
routes:
  - id: "private"
    path_prefix: "/private"
    require_auth: true

api_keys:
  - id: "local-dev"
    key_env: "LOCAL_API_KEY"
    tenant: "local"
```

When `redis.enabled` is `true`, route rate limits use Redis. Otherwise they use the in-memory limiter.

## Persistence

PostgreSQL wiring is opt-in:

```yaml
database:
  enabled: true
  driver: "postgres"
  dsn_env: "GATEWAY_DB_DSN"

analytics:
  enabled: true
  persistent: true
  window: 1h
```

When enabled, request logs are written to PostgreSQL and `/analytics/routes` reads from the `request_logs` table. API key lookup checks PostgreSQL first and then falls back to config-backed local keys.

Migrations live in `internal/db/migrations`.

## TLS

```yaml
server:
  address: ":8443"
  tls:
    enabled: true
    cert_file: "/etc/goproxy/tls.crt"
    key_file: "/etc/goproxy/tls.key"
```

## Hot Reload

```yaml
hot_reload:
  enabled: true
  interval: 2s
```

The gateway polls the config file and atomically swaps in a rebuilt handler when the file changes. Listener settings such as address and TLS files are read at process startup; restart the process to change those.

## Admin API

Enable admin endpoints with a token from the environment or a file:

```yaml
admin:
  enabled: true
  token_env: "GATEWAY_ADMIN_TOKEN"
```

Then call:

```sh
curl -H "Authorization: Bearer $GATEWAY_ADMIN_TOKEN" http://localhost:8080/admin/routes
curl -H "Authorization: Bearer $GATEWAY_ADMIN_TOKEN" http://localhost:8080/admin/upstreams
curl -X POST -H "Authorization: Bearer $GATEWAY_ADMIN_TOKEN" http://localhost:8080/admin/reload
```

## CLI

```sh
go run ./cmd/cli --config config/config.yaml routes list
go run ./cmd/cli --config config/config.yaml keys list
go run ./cmd/cli --config config/config.yaml keys create local-user --tenant local
go run ./cmd/cli stats
```

## Docker

```sh
docker compose up --build
```

This starts the gateway, a local echo upstream, PostgreSQL, and Redis. Compose enables database-backed logging, persistent analytics, Redis-backed rate limiting, and the admin API with `local-admin-token`.

## Tests

```sh
GOCACHE=/tmp/go-build-cache go test ./...
```

Use the explicit `GOCACHE` value in read-only environments.

## Plugins

Plugins implement `internal/plugin.Plugin`:

```go
type Plugin interface {
	Name() string
	Wrap(http.Handler) http.Handler
}
```

The lifecycle is registration, ordered selection from config or gateway startup code, then request-time execution as ordinary `http.Handler` middleware.
