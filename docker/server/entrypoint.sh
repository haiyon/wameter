#!/bin/sh
set -e

# Create necessary directories
mkdir -p /app/log /app/data

# Check configuration
if [ ! -f /app/config/server.yaml ]; then
    echo "Warning: Config file not found, using example config..."
    cp /app/config/server.example.yaml /app/config/server.yaml
fi

# Execute the main process
exec /app/wameter "$@"
