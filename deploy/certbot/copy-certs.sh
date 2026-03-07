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
if [ -n "${LOCAL_UID:-}" ] && [ -n "${LOCAL_GID:-}" ]; then
  if ! chown "${LOCAL_UID}:${LOCAL_GID}" /etc/envoy/tls/tls.key /etc/envoy/tls/tls.crt; then
    echo "warning: could not chown cert files to ${LOCAL_UID}:${LOCAL_GID}" >&2
  fi
fi
chmod 644 /etc/envoy/tls/tls.crt
chmod 600 /etc/envoy/tls/tls.key
