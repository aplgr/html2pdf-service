#!/bin/sh
set -eu

if [ -z "${RENEWED_LINEAGE:-}" ]; then
  echo "RENEWED_LINEAGE is not set; certbot deploy hook expects it." >&2
  exit 1
fi

if [ ! -f "${RENEWED_LINEAGE}/fullchain.pem" ] || [ ! -f "${RENEWED_LINEAGE}/privkey.pem" ]; then
  echo "Expected certbot files not found in ${RENEWED_LINEAGE}." >&2
  exit 1
fi

mkdir -p /etc/envoy/tls
cp "${RENEWED_LINEAGE}/fullchain.pem" /etc/envoy/tls/tls.crt
cp "${RENEWED_LINEAGE}/privkey.pem" /etc/envoy/tls/tls.key
chmod 644 /etc/envoy/tls/tls.crt /etc/envoy/tls/tls.key
