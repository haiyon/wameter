# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make git gcc libc-dev

WORKDIR /src

# Download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG TARGETOS
ARG TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w \
    -X 'wameter/internal/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")' \
    -X 'wameter/internal/version.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")' \
    -X 'wameter/internal/version.BuildDate=$(date -u "+%Y-%m-%d_%H:%M:%S")' \
    -X 'wameter/internal/version.GoVersion=$(go version | cut -d" " -f3)'" \
    -o /src/bin/wameter-agent \
    ./cmd/agent

# Final stage
FROM alpine:3.19

# Add non-root user
RUN adduser -D -H -h /app wameter
WORKDIR /app

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates tzdata iproute2 net-tools

# Copy binary and config
COPY --from=builder /src/bin/wameter-agent /app/
COPY --from=builder /src/examples/agent.example.yaml /app/config/agent.yaml
COPY docker/agent/entrypoint.sh /app/

# Set ownership and permissions
RUN chown -R wameter:wameter /app && \
    chmod +x /app/entrypoint.sh

# Create directories for logs and cache
RUN mkdir -p /app/log /app/cache && \
    chown -R wameter:wameter /app/log /app/cache

# Use non-root user for better security
USER wameter

# Expose agent metrics port
EXPOSE 8081

# Set entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["-config", "/app/config/agent.yaml"]

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8081/api/v1/healthz || exit 1

# Labels
LABEL org.opencontainers.image.source="https://github.com/haiyon/wameter" \
    org.opencontainers.image.description="Wameter Agent - Network monitoring agent" \
    org.opencontainers.image.licenses="MIT"
