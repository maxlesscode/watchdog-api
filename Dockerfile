FROM golang:1.26-alpine AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-X main.version=${VERSION}" \
    -o watchdog \
    ./cmd/watchdog/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S watchdog && \
    adduser -S -G watchdog watchdog

WORKDIR /app
COPY --from=builder --chown=watchdog:watchdog /build/watchdog .

USER watchdog
EXPOSE 9999

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:9999/health || exit 1

ENTRYPOINT ["./watchdog"]
