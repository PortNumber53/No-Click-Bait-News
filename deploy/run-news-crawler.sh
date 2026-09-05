#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${APP_DIR}/.env"
LOG_DIR="${APP_DIR}/logs"
LOG_FILE="${LOG_DIR}/news-crawler.log"
LOCK_FILE="${APP_DIR}/.news-crawler.lock"

mkdir -p "${LOG_DIR}"
touch "${LOG_FILE}"
exec >>"${LOG_FILE}" 2>&1

timestamp() {
  date --iso-8601=seconds
}

exec 9>"${LOCK_FILE}"
if ! flock -n 9; then
  printf '%s News crawl skipped: another run is active\n' "$(timestamp)"
  exit 0
fi

if [[ ! -r "${ENV_FILE}" ]]; then
  printf '%s News crawl failed: environment file is not readable\n' "$(timestamp)"
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

CRAWL_LIMIT="${1:-${NEWS_CRAWLER_LIMIT:-25}}"
if [[ ! "${CRAWL_LIMIT}" =~ ^[1-9][0-9]*$ ]]; then
  printf '%s News crawl failed: NEWS_CRAWLER_LIMIT must be a positive integer\n' "$(timestamp)"
  exit 1
fi

printf '%s News crawl starting: limit=%s\n' "$(timestamp)" "${CRAWL_LIMIT}"
set +e
"${APP_DIR}/api-ncbnews-backend" crawl-news "${CRAWL_LIMIT}"
status=$?
set -e
printf '%s News crawl finished: status=%s\n' "$(timestamp)" "${status}"
exit "${status}"
