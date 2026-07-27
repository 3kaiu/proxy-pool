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

WORKDIR /app
COPY --from=builder /build/relay /usr/local/bin/

ENV RELAY_LISTEN=:5010
ENV RELAY_TARGET=https://opencode.ai/zen
ENV RELAY_MAX_RETRIES=50
ENV RELAY_TIMEOUT=5

EXPOSE 5010

CMD ["relay"]
