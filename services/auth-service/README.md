# auth-service

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

## Configuration (YAML file)

The auth-service loads configuration from `config/auth-service.yaml` by default. You can override
the location by setting `CONFIG_PATH`.

The sample config lives at `services/auth-service/config/auth-service.yaml` and mirrors the
Docker Compose defaults (listen address, Postgres DSN, Redis settings, rate limiting, and logging).

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

## Schema management (deploy-time)

The auth-service owns the `tokens` table schema. Apply migrations during deployment (not at runtime):

```bash
psql "postgres://html2pdf:html2pdf@postgres:5432/html2pdf?sslmode=disable" \
  -f services/auth-service/deploy/postgres/migrations/001_create_tokens_table.sql
```

The migration is idempotent and safe to re-run for existing deployments.

### Schema validation

Deploys should fail fast if the schema is missing or invalid. The verification script uses pgTAP, so ensure
the extension is available, then run it as part of the deployment pipeline:

```bash
psql "postgres://html2pdf:html2pdf@postgres:5432/html2pdf?sslmode=disable" \
  -f services/auth-service/deploy/postgres/verify_tokens_schema.sql
```

The script raises an error if required tables, columns, or indexes are missing.

### Docker Compose

The project `deploy/docker-compose.yml` includes a one-shot `auth-migrate` service that waits for Postgres,
applies the migrations, and runs the pgTAP verification script before the auth-service starts.

## Project layout

The layout is intentionally small but keeps the separation of concerns:

- `internal/tokens`: in-memory token cache + periodic reloader
- `internal/infra/*`: adapters (Postgres token repository, Redis/memory rate limit storage)
- `internal/http/*`: transport (Fiber server, middleware, ext_authz handler)
- `cmd/auth-service`: wiring / entrypoint
