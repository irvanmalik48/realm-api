#!/bin/sh
set -e

# Ensure storage directory exists and has proper permissions for appuser
STORAGE_PATH="${STORAGE_DIR:-/data/storage}"
mkdir -p "$STORAGE_PATH"
chown -R appuser:appgroup "$STORAGE_PATH" 2>/dev/null || true
chmod -R 775 "$STORAGE_PATH" 2>/dev/null || true

# Drop root privileges and execute application as appuser
exec su-exec appuser:appgroup "$@"
