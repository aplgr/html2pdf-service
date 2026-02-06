# html2pdf — HTML/URL → PDF microservice (Go)

A pragmatic HTTP service that renders **HTML** (or a remote **URL**) to **PDF** using headless Chrome.
It ships with a small local stack (Envoy + Postgres + Redis + docs UI) so you can run it as a self-contained component.

[![live demo](http://img.shields.io/badge/live%20demo-html2pdf.aplgr.com-2ea44f)](http://html2pdf.aplgr.com)
[![status](https://img.shields.io/badge/status-alpha-orange)](#status)
[![scope](https://img.shields.io/badge/scope-microservice-blue)](#scope)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[![Tests](https://github.com/aplgr/html2pdf-service/actions/workflows/test.yml/badge.svg)](https://github.com/aplgr/html2pdf-service/actions/workflows/test.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](#requirements)
[![Go Reference](https://pkg.go.dev/badge/github.com/aplgr/html2pdf-service.svg)](https://pkg.go.dev/github.com/aplgr/html2pdf-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/aplgr/html2pdf-service)](https://goreportcard.com/report/github.com/aplgr/html2pdf-service)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../../pulls)

## Quick start

```bash
make start
```

`make start` defaults to `mode=dev` and will generate a local self-signed TLS
certificate if one is missing. In dev mode, the examples index is always
regenerated.

Stop the stack:

```bash
make stop
```

(If you prefer the raw command: `docker compose -f deploy/docker-compose.yml up -d --build`.)

- Live demo (public, **demo only**): `https://html2pdf.aplgr.com`

- Docs UI: `https://localhost/`
- API base URL (via Envoy): `https://localhost/api`
- Ops health (requires ops-enabled token): `https://localhost/ops/health`

The curl examples for **POST HTML → PDF** and **GET URL → PDF** are in the [API](#api) section below.

## Requirements

- **Docker + Docker Compose** (recommended, easiest)
- Optional for local dev (without Docker): **Go 1.23+** and **Chrome/Chromium**
  - plus **Postgres** and **Redis** if you still want auth + rate limits + caching

## What it does

- **Render PDF from raw HTML**: `POST /api/v0/pdf`
- **Render PDF from a URL**: `GET /api/v0/pdf?url=https://…`
- Optional **short-lived PDF cache** in Redis

Access + limits are enforced at the edge:

- **Public access** when no `X-API-Key` header is provided (still rate-limited).
- **API-key access** via `X-API-Key`
  - tokens + per-token `rate_limit` are stored in Postgres (`tokens` table)
  - tokens also include a JSONB `scope` (default: `{"api": true}`) to control ops access
  - the dedicated `auth-service` reloads tokens periodically
- **Rate limiting** is tracked in Redis (shared with the renderer cache)
  - token-based limiting uses the per-token `rate_limit` value
  - public limiting uses a user fingerprint derived from `IP + User-Agent` (default: 20 req/hour)

One Docker Compose stack with:

- **Envoy** as API gateway (`/api/*` → renderer, `/` → docs)
- **auth-service** (Go) as Envoy `ext_authz` backend (token auth + rate limiting)
- **html2pdf** (Go) renderer service
- **Postgres** for API token storage (simple `tokens` table)
- **Redis** shared
  - **rate limiting counters** (default DB 0, auth-service)
  - **PDF cache** (default DB 1, renderer)
- **Nginx** serving the built-in docs UI

## API

### POST HTML → PDF

```bash
curl -k -X POST "https://localhost/api/v0/pdf" --form-string "html=<h1>Hello PDF</h1>" -o out.pdf
```

### GET URL → PDF

```bash
curl -k "https://localhost/api/v0/pdf?url=https://example.org" -o out.pdf
```

### Auth / Rate limits

- Public request (no key): just call the API.
- API-key request:

```bash
curl -k -H "X-API-Key: YOUR_TOKEN" -X POST "https://localhost/api/v0/pdf" --for-string "html=<h1>Hello PDF</h1>" -o out.pdf
```

If a key is invalid → **401**. If a limit is exceeded → **429**.

## Architecture

High level:

```
Client
  │
  ▼
Envoy (443)
  ├─ /              → Nginx docs UI                 (ext_authz disabled)
  ├─ /ops/health    → ext_authz → html2pdf (8080)
  └─ /api/*         → ext_authz → html2pdf (8080)
                       │
                       ▼
                    auth-service (Go, 9000)
                      ├─ Postgres (tokens table: token + rate_limit + scope)
                      └─ Redis (rate limit counters; default DB 0)

html2pdf (Go) also uses Redis for the PDF cache (default DB 1).
```

Notes:

- Envoy listens on HTTPS (443) and redirects HTTP (80) to HTTPS.
- Envoy rewrites `/api/...` to `/...` so the Go service can stay on `/v0/*`.
- Postgres is used as a tiny token store. Tokens are loaded periodically at runtime.
- Redis is used for:
  - limiter storage (default DB `0`)
  - PDF cache (default DB `1`)

## Security notes

**Live demo:** [http://html2pdf.aplgr.com](https://html2pdf.aplgr.com)

This endpoint is a personal **demo/playground**. It may be rate-limited, wiped, and redeployed at any time.
**Do not send sensitive data.** If someone manages to break it, the realistic outcome is: I nuke the box and redeploy.

If you expose this service publicly (or run it in production), harden it first:

- Put Envoy in front (as in this repo) and do not expose the renderer directly.
- Add SSRF protections if you allow arbitrary `url=...` rendering.
- Put strict timeouts and size limits on requests and on headless Chrome.
- Consider stricter auth policies (e.g., require API keys for PDF rendering, keep public access only for docs).

### Ops endpoint auth

Ops endpoints (under `/ops/*`) require a token that includes the `ops` scope.
Tokens are stored in Postgres and default to `{"api": true}`. To allow ops access, set `ops: true`:

```sql
UPDATE tokens
SET scope = jsonb_set(scope, '{ops}', 'true', true)
WHERE token = 'YOUR_TOKEN';
```

## Development notes

- The auth component lives in `auth-service/` and ships with its own README + unit tests.
- Redis is shared between auth + renderer; keep DBs/prefixes separated to avoid collisions.
- If you run without Docker, you will need compatible Postgres/Redis endpoints and a local Chrome/Chromium.

### Testing

- Unit tests for both Go modules:

```bash
make test
```

- Stack-level integration tests (Docker Compose + Envoy/auth/renderer/Redis/Postgres):

```bash
make test-integration
```

The integration suite validates ext_authz token enforcement, rate-limiting behavior, cache population,
and an end-to-end Chrome render of a known HTML fixture.

### HTTPS local development (self-signed)

Envoy expects TLS files mounted at `/etc/envoy/tls` (see `deploy/docker-compose.yml`).
You can generate a local self-signed certificate with:

```bash
make start
```

`make start` (default `mode=dev`) generates a self-signed cert and starts the stack. Use `https://localhost/...`

### HTTPS staging/production (Let's Encrypt)

For staging/production, use publicly trusted certificates (Let's Encrypt) and
keep self-signed certs only for local dev. `make start mode=prod` does not
generate self-signed certificates. The Docker Compose stack includes an
optional certbot workflow (via the `staging`/`prod` profiles) that:

- Obtains initial certificates with HTTP-01.
- Auto-renews certificates on a schedule.
- Copies renewed certs into the shared TLS directory Envoy already reads.

**Prereqs**

- Point your DNS (A/AAAA record) at the host running this stack.
- Ensure ports **80** and **443** are reachable.

**Single-step bootstrap (recommended)**

```bash
make start mode=prod domain=YOUR_DOMAIN email=YOUR_EMAIL examples=yes
```

This runs the full flow (bring up the prod profile stack, issue the initial
cert, start renewals) without rebuilding images. If images are missing locally,
run `make build` once before the bootstrap. You can skip regenerating the
examples index in prod by omitting `examples=yes`.

**Enable renewals**

Renewals are handled by the certbot container that starts as part of the
`make start mode=prod ...` bootstrap. If you stop the stack and want to re-enable
renewals, just run the same command again.

**Envoy reload on renewal**

Envoy reads `/etc/envoy/tls/tls.crt` and `/etc/envoy/tls/tls.key`. The certbot
deploy hook copies the renewed `fullchain.pem`/`privkey.pem` into that directory.
If your Envoy build doesn't hot-reload certs, restart it after renewal:

```bash
make restart mode=prod domain=YOUR_DOMAIN email=YOUR_EMAIL
```

**ACME HTTP-01 routing**

Envoy keeps the global HTTP → HTTPS redirect, but it excludes
`/.well-known/acme-challenge/` so certbot can validate ownership over port 80.

**Optional host paths**

If you prefer different host locations, set:

- `LETSENCRYPT_DIR` (defaults to `deploy/letsencrypt`)
- `CERTBOT_WEBROOT_DIR` (defaults to `deploy/certbot/www`)
- `ENVOY_TLS_DIR` (defaults to `gateway/envoy/tls`)

## Status

- **status**: alpha (moving pieces are in place; expect sharp edges)
- **scope**: microservice / self-hosted component

## License

MIT. See `LICENSE`.
