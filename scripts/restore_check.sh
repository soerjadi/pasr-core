#!/usr/bin/env bash
# Restore test: load the latest dump into a throwaway postgres:17-alpine container and check that
# the core tables are readable. Exits non-zero on any failure. Needs Docker.
#
# Usage: scripts/restore_check.sh [dump-file]
# Configuration (environment):
#   BACKUP_DIR        optional  where backup.sh writes dumps (default: ./backups); the newest is used
#   RESTORE_IMAGE     optional  image to restore into (default: postgres:17-alpine)
#   RESTORE_TIMEOUT   optional  seconds to wait for the server (default: 60)
#   RESTORE_MIN_ROWS  optional  1 = also fail when users, products or orders is empty (default: 0)
set -euo pipefail

log() { printf '%s restore_check: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }
die() { log "FAIL: $*"; exit 1; }

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RESTORE_IMAGE="${RESTORE_IMAGE:-postgres:17-alpine}"
RESTORE_TIMEOUT="${RESTORE_TIMEOUT:-60}"
RESTORE_MIN_ROWS="${RESTORE_MIN_ROWS:-0}"

command -v docker >/dev/null || die "docker is not installed"

dump="${1:-}"
if [[ -z "$dump" ]]; then
  # newest by name: backup.sh names dumps nusantara-<UTC timestamp>.dump
  dump="$(find "$BACKUP_DIR" -maxdepth 1 -name 'nusantara-*.dump' -type f 2>/dev/null | sort | tail -n 1)"
fi
[[ -n "$dump" && -s "$dump" ]] || die "no dump found (pass a file or set BACKUP_DIR)"
log "restoring $dump into a throwaway $RESTORE_IMAGE"

container="nusantara-restore-check-$$"
# A random password that never leaves this process and the container.
password="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24 || true)"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --rm --name "$container" -e POSTGRES_PASSWORD="$password" -e POSTGRES_DB=restore_check \
  "$RESTORE_IMAGE" >/dev/null

# The image's init phase runs a socket-only server; TCP readiness means the real server is up.
deadline=$((SECONDS + RESTORE_TIMEOUT))
until docker exec "$container" pg_isready -h 127.0.0.1 -U postgres -q 2>/dev/null; do
  ((SECONDS < deadline)) || die "server not ready after ${RESTORE_TIMEOUT}s"
  sleep 1
done

docker cp "$dump" "$container:/tmp/restore.dump" >/dev/null
docker exec "$container" pg_restore --exit-on-error --no-owner --no-acl -U postgres -d restore_check /tmp/restore.dump ||
  die "pg_restore failed"

status=0
for table in users products orders; do
  if ! n="$(docker exec "$container" psql -U postgres -d restore_check -tAqc "SELECT count(*) FROM $table")"; then
    log "FAIL: cannot read $table"
    status=1
    continue
  fi
  log "$table: $n rows"
  if [[ "$RESTORE_MIN_ROWS" == "1" && "$n" -lt 1 ]]; then
    log "FAIL: $table is empty"
    status=1
  fi
done
((status == 0)) || exit "$status"
log "OK"
