# === Build Stage ===
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build lightweight relay (no SQLite, no scheduler, lazy validation)
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o relay ./cmd/relay/

# === Run Stage ===
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

RUN adduser -D -u 1000 relay

WORKDIR /app
COPY --from=builder /build/relay /usr/local/bin/

ENV RELAY_TARGET=https://opencode.ai/zen
ENV RELAY_MAX_RETRIES=50
ENV RELAY_TIMEOUT=5

# Shell form so $PORT (set by Render/Northflank/etc.) is expanded at runtime.
# Falls back to :7860 if PORT is not set.
CMD sh -c 'RELAY_LISTEN="${RELAY_LISTEN:-:${PORT:-7860}}" relay'
