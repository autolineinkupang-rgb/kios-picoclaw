# kios-picoclaw — Railway-ready image.
# Stage 1: build the Go binary.
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

# Stage 2: minimal runtime.
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata curl
ENV PICOCLAW_HOME=/app/.picoclaw
ENV TZ=Asia/Makassar
ENV KIOS_TEMPLATE_DIR=/app/templates
WORKDIR /app

COPY --from=builder /src/build/picoclaw-linux-amd64 /usr/local/bin/picoclaw
COPY workspace /app/workspace
COPY templates /app/templates
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Health/gateway port (Railway overrides via $PORT).
EXPOSE 18790

ENTRYPOINT ["/entrypoint.sh"]
