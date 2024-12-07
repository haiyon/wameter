# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make git gcc libc-dev

WORKDIR /src

# Download dependencies first to leverage Docker cache
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
    -o /src/bin/wameter \
    ./cmd/server

# Final stage
FROM alpine:3.19

# Add non-root user
RUN adduser -D -H -h /app wameter
WORKDIR /app

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary and config
COPY --from=builder /src/bin/wameter /app/
COPY --from=builder /src/examples/server.example.yaml /app/config/
COPY docker/server/entrypoint.sh /app/

# Set ownership and permissions
RUN chown -R wameter:wameter /app && \
    chmod +x /app/entrypoint.sh

# Use non-root user
USER wameter

# Create directories for logs and data
RUN mkdir -p /app/log /app/data

# Expose API port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["-config", "/app/config/server.yaml"]

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/v1/health || exit 1

# Labels
LABEL org.opencontainers.image.source="https://github.com/haiyon/wameter" \
    org.opencontainers.image.title="wameter" \
    org.opencontainers.image.description="Monitoring server" \
    org.opencontainers.image.licenses="MIT"
