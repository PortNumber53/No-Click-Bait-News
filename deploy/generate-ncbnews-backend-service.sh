#!/usr/bin/env bash
# Generate a systemd unit file for the No-Click Bait News Go backend.
# Usage: generate-ncbnews-backend-service.sh <target-dir> <output-file>
set -euo pipefail

TARGET_DIR="${1:?Usage: $0 <target-dir> <output-file>}"
OUTPUT="${2:?Usage: $0 <target-dir> <output-file>}"

cat > "$OUTPUT" <<UNIT
[Unit]
Description=No-Click Bait News Backend
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
EnvironmentFile=${TARGET_DIR}/.env

[Install]
WantedBy=multi-user.target
UNIT

echo "Generated ${OUTPUT}"
