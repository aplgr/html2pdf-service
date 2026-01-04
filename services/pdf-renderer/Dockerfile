# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY . .

# Build a small static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o html2pdf ./cmd/html2pdf/main.go

FROM alpine:3.20

RUN apk add --no-cache \
    chromium \
    ttf-dejavu \
    font-noto \
    font-noto-emoji \
    dumb-init

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_NO_SANDBOX=true

WORKDIR /app

COPY --from=builder /build/html2pdf /app/html2pdf
ENTRYPOINT ["dumb-init", "--"]
CMD ["./html2pdf"]
