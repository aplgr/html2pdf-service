#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="deploy/docker-compose.yml"
DC=(docker compose -f "$COMPOSE_FILE")
BASE_URL="https://localhost"
API_URL="$BASE_URL/api/v0/pdf"
TOKEN="abc123-test-first-token"
INVALID_TOKEN="definitely-invalid-token"
HTML_FIXTURE="examples/invoice.html"

cleanup() {
  "${DC[@]}" down >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
  echo "[FAIL] $*" >&2
  exit 1
}

log() {
  echo "[INFO] $*"
}

assert_status() {
  local expected="$1"
  local actual="$2"
  local msg="$3"
  if [[ "$expected" != "$actual" ]]; then
    fail "$msg (expected $expected, got $actual)"
  fi
}

wait_for_https() {
  local deadline=$((SECONDS + 180))
  while (( SECONDS < deadline )); do
    local status
    # During startup Envoy/TLS can reset connections transiently; suppress noisy curl transport errors here.
    status=$(curl -k -s -o /dev/null -w '%{http_code}' "$BASE_URL/" 2>/dev/null || true)
    if [[ "$status" == "200" ]]; then
      log "Gateway is ready"
      return 0
    fi
    sleep 2
  done
  fail "Timed out waiting for Envoy/docs endpoint"
}

wait_for_token_reload() {
  local html
  html=$(cat "$HTML_FIXTURE")
  local deadline=$((SECONDS + 120))
  while (( SECONDS < deadline )); do
    local status
    status=$(curl -k -s -o /dev/null -w '%{http_code}' \
      -H "X-API-Key: $TOKEN" \
      --form-string "html=$html" \
      "$API_URL" 2>/dev/null || true)
    if [[ "$status" == "200" ]]; then
      log "Token is active in auth-service cache"
      return 0
    fi
    # could be 401 before initial DB load or 503 while cache warming
    sleep 2
  done
  fail "Timed out waiting for token to become active"
}

log "Starting integration stack"
make start
wait_for_https
wait_for_token_reload

html=$(cat "$HTML_FIXTURE")

log "Test: ext_authz enforcement for invalid token"
invalid_status=$(curl -k -sS -o /dev/null -w '%{http_code}' \
  -H "X-API-Key: $INVALID_TOKEN" \
  --form-string "html=$html" \
  "$API_URL")
assert_status "401" "$invalid_status" "invalid token should be rejected"

log "Test: valid token request succeeds and returns PDF"
valid_status=$(curl -k -sS -o /tmp/int-valid.pdf -w '%{http_code}' \
  -H "X-API-Key: $TOKEN" \
  --form-string "html=$html" \
  "$API_URL")
assert_status "200" "$valid_status" "valid token request should succeed"
[[ -s /tmp/int-valid.pdf ]] || fail "expected non-empty PDF body for valid token"

log "Test: rate limiting for token path returns 429"
# Build a token with small rate limit for this test.
"${DC[@]}" exec -T postgres psql -U html2pdf -d html2pdf -c \
  "INSERT INTO tokens(token, rate_limit, scope, comment)
   VALUES ('int-low-limit-token', 1, '{\"api\": true}'::jsonb, 'integration low limit')
   ON CONFLICT (token) DO UPDATE SET rate_limit = EXCLUDED.rate_limit, scope = EXCLUDED.scope;" >/dev/null

sleep 65

first_low=$(curl -k -sS -o /dev/null -w '%{http_code}' \
  -H "X-API-Key: int-low-limit-token" \
  --form-string "html=$html" \
  "$API_URL")
second_low=$(curl -k -sS -o /dev/null -w '%{http_code}' \
  -H "X-API-Key: int-low-limit-token" \
  --form-string "html=$html" \
  "$API_URL")
assert_status "200" "$first_low" "first low-limit token request should pass"
assert_status "429" "$second_low" "second low-limit token request should be rate limited"

log "Test: cache hit on repeated request (Redis key exists)"
"${DC[@]}" exec -T redis redis-cli -n 1 FLUSHDB >/dev/null
cache_1=$(curl -k -sS -o /tmp/int-cache-1.pdf -w '%{http_code}' \
  --form-string "html=$html" \
  "$API_URL")
cache_2=$(curl -k -sS -o /tmp/int-cache-2.pdf -w '%{http_code}' \
  --form-string "html=$html" \
  "$API_URL")
assert_status "200" "$cache_1" "first cache request should succeed"
assert_status "200" "$cache_2" "second cache request should succeed"

cache_count=$("${DC[@]}" exec -T redis redis-cli -n 1 --scan --pattern 'pdfcache:*' | wc -l | tr -d ' ')
if [[ "$cache_count" -lt 1 ]]; then
  fail "expected cached pdf key(s) in redis after repeated request"
fi

log "All integration checks passed"
