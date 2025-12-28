#!/bin/bash
# Simple load testing script for html2pdf service.
# Usage: ./scripts/loadtest.sh [url] [html_file] [concurrency] [requests]

URL=${1:-http://localhost/api/v1/pdf}
HTML_FILE=${2:-examples/example.html}
CONCURRENCY=${3:-10}
REQUESTS=${4:-200}

# Make variables available to subshells spawned by xargs
export URL HTML_FILE CONCURRENCY REQUESTS

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for this script" >&2
  exit 1
fi

run_request() {
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$URL" -F "html=<${HTML_FILE}"
}

export -f run_request
seq "$REQUESTS" | xargs -n1 -P "$CONCURRENCY" bash -c run_request
