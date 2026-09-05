#!/usr/bin/env bash
# Generate an hourly cron definition for the No-Click Bait News crawler.
# Usage: generate-news-crawler-cron.sh <target-dir> <output-file>
set -euo pipefail

TARGET_DIR="${1:?Usage: $0 <target-dir> <output-file>}"
OUTPUT="${2:?Usage: $0 <target-dir> <output-file>}"

cat >"${OUTPUT}" <<CRON
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin
MAILTO=""

# Run at minute 17 each hour to avoid the top-of-hour traffic spike.
17 * * * * grimlock ${TARGET_DIR}/run-news-crawler.sh
CRON

echo "Generated ${OUTPUT}"
