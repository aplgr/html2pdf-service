# html2pdf-auth-service

This service is meant to sit in front of one or more internal microservices behind Envoy.
Envoy calls it via `ext_authz` (HTTP service) to decide whether a request is allowed.

It implements two access modes:

- **Public access**: no `X-API-Key` header present
- **API key access**: `X-API-Key` present and validated against tokens loaded from Postgres

Rate limiting is enforced here as an additional access barrier:

- **Public** requests can be limited per user (derived from IP + User-Agent)
- **Token** requests can be limited per token using the per-token limits loaded from Postgres

Redis is used as the rate limit storage backend (shared with the html2pdf render service).
Postgres is used only for token storage.

## Endpoints

- `GET /ext-authz` and `GET /ext-authz/*`
  - Returns `200 OK` when allowed
  - Returns `401` for invalid API keys
  - Returns `503` when the token store is not ready yet (startup window)
  - Adds `X-Auth-Mode: public|token` for easy debugging

- `GET /health`
  - Basic health check endpoint (Fiber healthcheck middleware)

## Configuration (environment variables)

All config is environment-driven (Docker friendly). The names match the project compose setup.

### Server

- `AUTH_LISTEN_ADDR` (default `:8081`)

### Postgres token store

- `AUTH_POSTGRES_DSN` (e.g. `postgres://html2pdf:html2pdf@postgres:5432/html2pdf?sslmode=disable`)
- `AUTH_TOKEN_RELOAD_INTERVAL` (default `10s`)

### Redis rate limit store

- `AUTH_REDIS_ADDR` (default `redis:6379`)
- `AUTH_REDIS_PASSWORD` (default empty)
- `AUTH_REDIS_RATE_DB` (default `0`)

### Rate limiting behaviour

- `AUTH_RL_INTERVAL` (default `1h`)
- `AUTH_RL_ENABLE_USER_LIMITER` (default `true`)
- `AUTH_RL_USER_LIMIT` (default `60`)
- `AUTH_RL_ENABLE_TOKEN_LIMITER` (default `true`)

## How it is used from Envoy (conceptual)

Envoy is configured with `envoy.filters.http.ext_authz` and points to this service (cluster).
For each request, Envoy calls `/ext-authz` and allows the original request only if this service returns `200 OK`.

## Development

Run unit tests:

```bash
cd auth-service
go test ./...
```

Build:

```bash
cd auth-service
go build ./cmd/auth-service
```

## Project layout

The layout is intentionally small but keeps the separation of concerns:

- `internal/tokens`: in-memory token cache + periodic reloader
- `internal/infra/*`: adapters (Postgres token repository, Redis/memory rate limit storage)
- `internal/http/*`: transport (Fiber server, middleware, ext_authz handler)
- `cmd/auth-service`: wiring / entrypoint
