#!/bin/sh
set -e

# Create necessary directories
mkdir -p /app/log /app/cache

# Check configuration
if [ ! -f /app/config/agent.yaml ]; then
    echo "Warning: Config file not found, using example config..."
    cp /app/config/agent.example.yaml /app/config/agent.yaml
fi

# Execute the main process
exec /app/wameter-agent "$@"
