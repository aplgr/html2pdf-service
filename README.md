# html2pdf — HTML/URL → PDF microservice (Go)

A pragmatic HTTP service that renders **HTML** (or a remote **URL**) to **PDF** using headless Chrome.
It ships with a small local stack (Envoy + Postgres + Redis + docs UI) so you can run it as a self-contained component.

[![live demo](https://img.shields.io/badge/live%20demo-html2pdf.aplgr.com-2ea44f)](https://html2pdf.aplgr.com)
[![status](https://img.shields.io/badge/status-alpha-orange)](#status)
[![scope](https://img.shields.io/badge/scope-microservice-blue)](#scope)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](#requirements)
[![Go Reference](https://pkg.go.dev/badge/github.com/aplgr/html2pdf-service.svg)](https://pkg.go.dev/github.com/aplgr/html2pdf-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/aplgr/html2pdf-service)](https://goreportcard.com/report/github.com/aplgr/html2pdf-service)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../../pulls)

## Quick start

```bash
docker compose up --build
```

- Live demo (public, **demo only**): `https://html2pdf.aplgr.com`

- Docs UI: `http://localhost/`
- API base URL (via Envoy): `http://localhost/api`
- Health check:

```bash
curl -sS http://localhost/api/health
```

The curl examples for **POST HTML → PDF** and **GET URL → PDF** are in the [API](#api) section below.

## Requirements

- **Docker + Docker Compose** (recommended, easiest)
- Optional for local dev (without Docker): **Go 1.25+** and **Chrome/Chromium**

## What it does

- **Render PDF from raw HTML**: `POST /api/v1/pdf`
- **Render PDF from a URL**: `GET /api/v1/pdf?url=https://…`
- Optional **short-lived PDF cache** in Redis
- Two-tier **rate limiting**
  - **Token-based** via `X-API-Key` (limits are stored in Postgres)
  - **User-based** fallback via `IP + User-Agent` (default: 20 requests / hour)
- One Docker Compose stack with:
  - **Envoy** as reverse proxy (`/api/*` → service, `/` → docs)
  - **Postgres** for API token storage (simple `tokens` table)
  - **Redis** for rate limiting + PDF cache
  - **Nginx** serving the built-in docs UI

## API

### POST HTML → PDF

```bash
curl -X POST "http://localhost/api/v1/pdf" \
  -F "html=<h1>Hello PDF</h1>" \
  -F "format=A4" \
  -o out.pdf
```

With API key:

```bash
curl -X POST "http://localhost/api/v1/pdf" \
  -H "X-API-Key: <token>" \
  -F "html=<h1>Priority lane</h1>" \
  -o out.pdf
```

### GET URL → PDF

```bash
curl -L "http://localhost/api/v1/pdf?url=https://example.org" -o out.pdf
```

## Architecture

High level:

```
Client
  │
  ▼
Envoy (80)
  ├─ /        → Nginx docs UI
  └─ /api/*   → prefix_rewrite "/" → html2pdf (Fiber, 8080)
                      │
                      ├─ Postgres (tokens table)
                      └─ Redis (rate limit store + optional PDF cache)
```

Notes:

- Envoy rewrites `/api/...` to `/...` so the Go service can stay on `/v1/*`.
- Postgres is used as a tiny token store. Tokens are loaded periodically at runtime.
- Redis is used for:
  - limiter storage (default DB `0`)
  - PDF cache (default DB `1`)

## Configuration

Config is a single YAML file (default: `config/html2pdf.yaml`).

- Override path via `CONFIG_PATH=/path/to/html2pdf.yaml`
- Key sections:
  - `server` (listen host/port, prefork)
  - `cache` (Redis host, DBs, PDF cache on/off, TTL)
  - `rate_limiter` (interval, user limiter toggle + limit)
  - `auth.postgres` (token table source)
  - `pdf` (timeout, chrome flags, pooling, paper sizes)

Related docs:

- `docs/redis.md`
- `docs/postgres.md`

## Observability

- Health: `GET /health`
- Simple runtime info: `GET /v1/chrome/stats`
- Fiber monitor UI: `GET /v1/monitor`

(With Envoy, those endpoints are available under `http://localhost/api/...`.)

## Security notes

**Live demo:** `https://html2pdf.aplgr.com`

This endpoint is a personal **demo/playground**. It may be rate-limited, wiped, and redeployed at any time.
**Do not send sensitive data.** If someone manages to break it, the realistic outcome is: I nuke the box and redeploy.

If you expose this service publicly (or run it in production), harden it first. It can render arbitrary HTML and fetch arbitrary URLs, which makes it effectively **browser-as-a-service**.

Security-related PRs are very welcome. If you find something serious, prefer **GitHub Security Advisories** over a public issue.

## Contributing

PRs are welcome.

Especially welcome:
- security + hardening improvements (auth/rate limiting at the edge, SSRF defenses, safer defaults)
- tests (unit + integration) and better real-world examples
- performance and stability improvements around Chrome lifecycle handling

If you plan a larger change, open an issue first so we can align on direction and scope.

Basic expectations:

- run `go test ./...` and keep tests green
- run `gofmt` on Go code
- keep examples in `examples/` runnable (curl snippets should work)
- if you change config, update `config/html2pdf.yaml` and the README

## Mini-roadmap

Ideas that would meaningfully improve this service:

- move API key verification + rate limiting from the Go app into Envoy (where it fits better)
- improve quality and coverage of examples (more real-world HTML, auth + cache scenarios)
- add unit + integration tests (rate limiting, token reload, cache behavior, chrome lifecycle)
- performance + security hardening (timeouts, SSRF protection, resource limits, input size limits)
- move observability endpoints under the service path and protect them separately (e.g. dedicated API key)
- make Chrome tab/session handling more robust (leaks, crashes, concurrency edge cases)

## Status

- **status**: alpha (moving pieces are in place; expect sharp edges)
- **scope**: microservice / self-hosted component

## License

MIT. See `LICENSE`.
