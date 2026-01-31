# html2pdf — HTML/URL → PDF microservice (Go)

A pragmatic HTTP service that renders **HTML** (or a remote **URL**) to **PDF** using headless Chrome.
It ships with a small local stack (Envoy + Postgres + Redis + docs UI) so you can run it as a self-contained component.

[![live demo](https://img.shields.io/badge/live%20demo-html2pdf.aplgr.com-2ea44f)](https://html2pdf.aplgr.com)
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

`make start` will generate a local self-signed TLS certificate if one is missing.

Stop the stack:

```bash
make stop
```

(If you prefer the raw command: `docker compose -f deploy/docker-compose.yml up -d --build`.)

- Live demo (public, **demo only**): `https://html2pdf.aplgr.com`

- Docs UI: `https://localhost/`
- API base URL (via Envoy): `https://localhost/api`
- Health (unprotected): `https://localhost/health`
- Metrics (unprotected): `https://localhost/metrics`

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

High level:

```
Client
  │
  ▼
Envoy (443)
  ├─ /              → Nginx docs UI                 (ext_authz disabled)
  ├─ /health        → html2pdf (Fiber, 8080)        (ext_authz disabled)
  ├─ /metrics       → html2pdf (Fiber, 8080)        (ext_authz disabled)
  └─ /api/*         → ext_authz → html2pdf (8080)
                       │
                       ▼
                    auth-service (Go, 9000)
                      ├─ Postgres (tokens table: token + rate_limit)
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

**Live demo:** `https://html2pdf.aplgr.com`

This endpoint is a personal **demo/playground**. It may be rate-limited, wiped, and redeployed at any time.
**Do not send sensitive data.** If someone manages to break it, the realistic outcome is: I nuke the box and redeploy.

If you expose this service publicly (or run it in production), harden it first:

- Put Envoy in front (as in this repo) and do not expose the renderer directly.
- Add SSRF protections if you allow arbitrary `url=...` rendering.
- Put strict timeouts and size limits on requests and on headless Chrome.
- Consider stricter auth policies (e.g., require API keys for PDF rendering, keep public access only for docs/health).

## Development notes

- The auth component lives in `auth-service/` and ships with its own README + unit tests.
- Redis is shared between auth + renderer; keep DBs/prefixes separated to avoid collisions.
- If you run without Docker, you will need compatible Postgres/Redis endpoints and a local Chrome/Chromium.

### HTTPS local development (self-signed)

Envoy expects TLS files mounted at `/etc/envoy/tls` (see `deploy/docker-compose.yml`).
You can generate a local self-signed certificate with:

```bash
make cert
```

Then start the stack and use `https://localhost/...` (add `-k` to curl to trust the
self-signed certificate). The TLS files are gitignored so private keys are not committed.

### HTTPS production notes

In production, mount your real certificate/key into `/etc/envoy/tls` so Envoy can
read `/etc/envoy/tls/tls.crt` and `/etc/envoy/tls/tls.key`.

## Status

- **status**: alpha (moving pieces are in place; expect sharp edges)
- **scope**: microservice / self-hosted component

## License

MIT. See `LICENSE`.
