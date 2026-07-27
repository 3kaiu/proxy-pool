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

# HF Spaces requires running as user 1000
RUN adduser -D -u 1000 relay

WORKDIR /app
COPY --from=builder /build/relay /usr/local/bin/

# Default port 7860 for HF Spaces (override with RELAY_LISTEN for other platforms)
ENV RELAY_LISTEN=:7860
ENV RELAY_TARGET=https://opencode.ai/zen
ENV RELAY_MAX_RETRIES=50
ENV RELAY_TIMEOUT=5

EXPOSE 7860

USER relay

CMD ["relay"]
