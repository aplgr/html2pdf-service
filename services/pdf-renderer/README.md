# pdf-renderer (html2pdf)

This service renders **HTML** (or a remote **URL**) to a **PDF** using headless Chromium (chromedp).

It is designed to run behind the project gateway (Envoy). Authentication and rate limiting are expected to be handled upstream (e.g. via the companion `auth-service` and Envoy filters).

## Endpoints

All endpoints are under `/v1`:

- `POST /v1/pdf`
  - Content type: `application/x-www-form-urlencoded` or `multipart/form-data`
  - Form fields:
    - `html` (required) — HTML string (min length checks apply)
    - `format` (optional) — paper format key (e.g. `A4`, `LETTER`, `LEGAL`, …). Defaults to `pdf.default_paper`.
    - `orientation` (optional) — `portrait` (default) or `landscape`
    - `margin` (optional) — float inches, `0.1` … `2.0` (default `0.4`)
    - `filename` (optional) — must end with `.pdf` and match `^[a-zA-Z0-9_.-]+$` (default `output.pdf`)
  - Response: `application/pdf`

- `GET /v1/pdf`
  - Query parameters:
    - `url` (required) — `http` / `https` URL to render
    - `format`, `orientation`, `margin`, `filename` — same meaning as in `POST /v1/pdf`
  - Response: `application/pdf`

- `GET /v1/chrome/stats`
  - Basic stats about the Chrome pool (useful for debugging load / pooling).

- `GET /v1/monitor`
  - Fiber monitor endpoint (intended for internal use).

## Configuration

Configuration is YAML-driven. By default the service loads:

- `config/html2pdf.yaml`

You can override the path via:

- `CONFIG_PATH=/path/to/html2pdf.yaml`

### Key settings (YAML)

- `server.host`, `server.port`, `server.prefork`
  - Note: prefork does **not** mix well with a shared Chrome pool. If you need more throughput, prefer increasing `pdf.chrome_pool_size`.

- `limits.max_html_bytes`, `limits.max_pdf_bytes`

- `logger.file`, `logger.level`, `logger.max_size_mb`, `logger.max_backups`, `logger.max_age_days`, `logger.compress`

- `cache.pdf_cache_enabled`
  - Enables short-lived PDF caching in Redis (useful when users click “generate” multiple times in quick succession).

- `cache.pdf_cache_ttl`
  - TTL for cached PDFs (e.g. `2m`, `5m`, `10m`). If `0`, a safe default is applied.

- `cache.redis_host`, `cache.redis_pdf_db`
  - Redis connection settings for PDF caching.

- `pdf.default_paper`, `pdf.paper_sizes`
  - Defines available paper formats and their width/height (inches).

- `pdf.timeout_secs`
  - Render timeout (seconds).

- `pdf.chrome_path`
  - Explicit path to Chromium/Chrome binary.

- `pdf.chrome_no_sandbox`
  - Adds `--no-sandbox` when launching Chromium (typical in containers).

- `pdf.chrome_pool_size`
  - Preloaded (pooled) Chrome tabs. `0` disables pooling and starts Chrome per request.

- `pdf.user_data_dir`
  - Fixed user data dir for Chromium (recommended when pooling).

### Environment override

- `CHROME_BIN`
  - If set, it overrides `pdf.chrome_path` (handy in container environments).

## Build & run (Docker)

From the repo root:

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Or build just this service:

```bash
cd services/pdf-renderer
docker build -t pdf-renderer .
```
