#!/usr/bin/env bash
# Nightly PostgreSQL backup: pg_dump -Fc to a timestamped file, copy it to the off-site bucket,
# then delete local and remote dumps older than the retention period.
#
# Configuration (environment only; nothing secret is printed):
#   DATABASE_URL        required  libpq URL of the database to dump
#   BACKUP_REMOTE       required  off-site location: s3://bucket/prefix (aws) or remote:path (rclone)
#   BACKUP_TOOL         optional  aws | rclone (default: aws for s3:// remotes, otherwise rclone)
#   BACKUP_DIR          optional  local directory for dumps (default: ./backups)
#   RETENTION_DAYS      optional  days to keep dumps locally and remotely (default: 14)
#   BACKUP_SKIP_UPLOAD  optional  1 = dump and prune locally only (no off-site copy; for testing)
#   PG_DUMP             optional  pg_dump command (default: pg_dump), for example
#                                 "docker compose exec -T postgres pg_dump" to use the server's own client
# aws reads its usual variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_ENDPOINT_URL for R2/B2);
# rclone reads its config file or RCLONE_CONFIG_* variables.
set -euo pipefail

log() { printf '%s backup: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

: "${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
BACKUP_SKIP_UPLOAD="${BACKUP_SKIP_UPLOAD:-0}"
PG_DUMP="${PG_DUMP:-pg_dump}"
PREFIX="nusantara-"

[[ "$RETENTION_DAYS" =~ ^[0-9]+$ ]] || die "RETENTION_DAYS must be a whole number"

if [[ "$BACKUP_SKIP_UPLOAD" != "1" ]]; then
  : "${BACKUP_REMOTE:?BACKUP_REMOTE is required (or set BACKUP_SKIP_UPLOAD=1)}"
  BACKUP_REMOTE="${BACKUP_REMOTE%/}"
  if [[ -z "${BACKUP_TOOL:-}" ]]; then
    if [[ "$BACKUP_REMOTE" == s3://* ]]; then BACKUP_TOOL=aws; else BACKUP_TOOL=rclone; fi
  fi
  case "$BACKUP_TOOL" in
    aws | rclone) command -v "$BACKUP_TOOL" >/dev/null || die "$BACKUP_TOOL is not installed" ;;
    *) die "BACKUP_TOOL must be aws or rclone" ;;
  esac
fi

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
name="${PREFIX}${stamp}.dump"
file="${BACKUP_DIR%/}/${name}"
partial="${file}.partial"
trap 'rm -f "$partial"' EXIT

log "dumping database to $file"
# shellcheck disable=SC2086 # PG_DUMP may be a multi-word command
$PG_DUMP --format=custom --no-owner --no-acl --dbname="$DATABASE_URL" >"$partial"
[[ -s "$partial" ]] || die "pg_dump produced an empty file"
mv "$partial" "$file"
log "dump complete ($(wc -c <"$file" | tr -d ' ') bytes)"

# cutoff_stamp prints the UTC timestamp RETENTION_DAYS ago in the dump-name format (GNU or BSD date).
cutoff_stamp() {
  date -u -d "-${RETENTION_DAYS} days" +%Y%m%dT%H%M%SZ 2>/dev/null ||
    date -u -v-"${RETENTION_DAYS}"d +%Y%m%dT%H%M%SZ
}
cutoff="$(cutoff_stamp)"

if [[ "$BACKUP_SKIP_UPLOAD" != "1" ]]; then
  log "copying $name off-site with $BACKUP_TOOL"
  if [[ "$BACKUP_TOOL" == aws ]]; then
    aws s3 cp --only-show-errors "$file" "$BACKUP_REMOTE/$name"
    # Names sort by time, so every dump named before the cutoff is older than the retention period.
    aws s3 ls "$BACKUP_REMOTE/" | awk '{print $NF}' | while read -r old; do
      if [[ "$old" == ${PREFIX}*.dump && "$old" < "${PREFIX}${cutoff}.dump" ]]; then
        log "deleting remote $old"
        aws s3 rm --only-show-errors "$BACKUP_REMOTE/$old"
      fi
    done
  else
    rclone copyto "$file" "$BACKUP_REMOTE/$name"
    rclone delete "$BACKUP_REMOTE" --min-age "${RETENTION_DAYS}d" --include "${PREFIX}*.dump"
  fi
else
  log "BACKUP_SKIP_UPLOAD=1: not copying off-site"
fi

for old in "${BACKUP_DIR%/}/${PREFIX}"*.dump; do
  [[ -e "$old" ]] || continue
  base="$(basename "$old")"
  if [[ "$base" < "${PREFIX}${cutoff}.dump" ]]; then
    log "deleting local $base"
    rm -f "$old"
  fi
done
log "done"
