#!/usr/bin/env bash
# Generate a systemd unit file for the NoClickBait News Go backend.
# Usage: generate-ncbnews-backend-service.sh <target-dir> <output-file> [environment-file]
set -euo pipefail

TARGET_DIR="${1:?Usage: $0 <target-dir> <output-file>}"
OUTPUT="${2:?Usage: $0 <target-dir> <output-file>}"
ENVIRONMENT_FILE="${3:-/etc/ncbnews/backend.env}"

cat > "$OUTPUT" <<UNIT
[Unit]
Description=NoClickBait News Backend
After=network.target postgresql.service

[Service]
Type=simple
User=grimlock
Group=grimlock
WorkingDirectory=${TARGET_DIR}
ExecStart=${TARGET_DIR}/api-ncbnews-backend
Restart=on-failure
RestartSec=5
StandardOutput=append:${TARGET_DIR}/logs/stdout.log
StandardError=append:${TARGET_DIR}/logs/stderr.log
EnvironmentFile=${ENVIRONMENT_FILE}

[Install]
WantedBy=multi-user.target
UNIT

echo "Generated ${OUTPUT}"
