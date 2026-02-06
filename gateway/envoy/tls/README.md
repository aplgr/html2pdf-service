# Envoy TLS certificates

This directory is mounted into the Envoy container at `/etc/envoy/tls`.

For local development, generate a self-signed certificate and private key here.
These files are gitignored so private material is not committed.

For staging/production, point Envoy at publicly trusted certificates (e.g.
Let's Encrypt). The certbot workflow in `deploy/` can copy the renewed
`fullchain.pem` and `privkey.pem` into this directory so Envoy continues to read
`/etc/envoy/tls/tls.crt` and `/etc/envoy/tls/tls.key`.
