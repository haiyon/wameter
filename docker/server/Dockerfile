# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make git

WORKDIR /src

# Download dependencies first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 make build-server

# Final stage
FROM alpine:3.19

# Add non-root user
RUN adduser -D -H -h /app wameter
WORKDIR /app

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary and config
COPY --from=builder /src/bin/*_*/wameter-server /app/
COPY --from=builder /src/examples/server.example.yaml /app/config/server.yaml
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
    org.opencontainers.image.description="Wameter Server - Network monitoring server" \
    org.opencontainers.image.licenses="MIT"
