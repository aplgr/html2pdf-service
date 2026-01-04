# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY . .

# Build a small static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o auth-service ./cmd/auth-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates dumb-init

WORKDIR /app

COPY --from=builder /build/auth-service /app/auth-service

ENTRYPOINT ["dumb-init", "--"]
CMD ["./auth-service"]
