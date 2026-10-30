# Nusantara core API

Go backend for the Nusantara pickup marketplace: a hexagonal service (`internal/core` holds the domain,
ports and services; `internal/adapters` the REST handlers and PostgreSQL repositories) behind a JSON API
under `/v1`. The contract is `../docs/api/openapi.yaml`; `../docs/api/README.md` lists the operations and
error codes.

## Requirements

- Go 1.26
- PostgreSQL 17 (`docker compose up -d postgres` starts one)
- Docker, for the integration tests (Testcontainers starts a throwaway `postgres:17-alpine` per test binary)
  and for `scripts/restore_check.sh`
- `golangci-lint` v2 for `make lint`

## Run

```sh
cp config.example.toml config.toml   # then edit; config.toml is git-ignored
make run                             # or: CONFIG_FILE=/path/config.toml go run ./cmd/api
```

With Docker: `docker compose up -d --build` (mounts `./config.toml`; set `database.url` to
`postgres://nusantara:nusantara@postgres:5432/nusantara?sslmode=disable`). `GET /healthz` returns 200, or 503 when the
database is unreachable.

## Configuration

One TOML file, path from `CONFIG_FILE` (default `config.toml`). `config.example.toml` documents every key. Secrets
(`jwt.*_secret`, `otp.secret`, `storage.s3_*`, the FCM credentials file) live only in that file or the deployment's
secret store, never in the repository. With `app.env = "production"` the server refuses to start without JWT and OTP
secrets, and without bucket credentials when `storage.driver = "s3"`.

Main sections: `app` (port, CORS origins, `operator_phones`), `database` (URL, pool, `auto_migrate`, query tracing),
`jwt`, `otp` (TTL, attempt cap, per-phone and per-IP limits), `orders` (accept window, time zone for order numbers),
`images` (byte and pixel limits), `storage` (`local` or `s3`-compatible such as R2), `push` (`log` or `fcm`),
`log`, `tracing` (OTLP).

## Migrations

SQL migrations live in `migrations/` (goose format, forward-only, each with a working `Down`) and are **embedded in the
binary**, so the Docker image needs no migration files:

- `database.auto_migrate = true` applies every pending migration at startup (idempotent).
- Or run them yourself: `make migrate-up DB_URL=postgres://...` (`make migrate-down` rolls back one).

## Test and lint

```sh
make verify          # gofmt check, go vet, unit tests only (-short: no Docker)
go test -race ./...  # everything, integration tests included (needs Docker)
make lint            # golangci-lint
make openapi-check   # every openapi.yaml operation is routed, secured ones answer 401, every body has a fixture
```

Integration tests share one PostgreSQL container per test binary and truncate it before each test. API tests check
responses against `../docs/api/fixtures/<operationId>.json` (`internal/testutil/contract`).

## Backups

`scripts/backup.sh` writes `pg_dump -Fc` to `BACKUP_DIR/nusantara-<UTC timestamp>.dump`, copies it off-site and deletes
local and remote dumps older than `RETENTION_DAYS` (14). Run it nightly (cron or a systemd timer). Environment:

| variable | default | |
|---|---|---|
| `DATABASE_URL` | required | database to dump |
| `BACKUP_REMOTE` | required | `s3://bucket/prefix` (aws CLI; set `AWS_ENDPOINT_URL` for R2/B2) or `remote:path` (rclone) |
| `BACKUP_TOOL` | `aws` for `s3://`, else `rclone` | |
| `BACKUP_DIR` | `./backups` | local copies |
| `RETENTION_DAYS` | `14` | |
| `PG_DUMP` | `pg_dump` | e.g. `docker compose exec -T postgres pg_dump` to use the server's own client |
| `BACKUP_SKIP_UPLOAD` | `0` | `1` skips the off-site copy (testing only) |

Credentials come from the tool's own environment or config; the scripts never print them.

`scripts/restore_check.sh [dump]` restores the newest dump in `BACKUP_DIR` (or the given file) into a throwaway
`postgres:17-alpine` container, counts `users`, `products` and `orders`, and exits non-zero on any failure
(`RESTORE_MIN_ROWS=1` also fails on empty tables). Run it weekly against a dump fetched back from the bucket, so the
off-site copy is the one being proven.

## Operations notes

- **Client IP and Cloudflare.** OTP rate limits key on `CF-Connecting-IP` when present, which any client can forge. The
  origin must be reachable **only through the Cloudflare Tunnel**; never expose the port directly.
- **Refresh tokens rotate.** Each refresh issues a new refresh token; presenting an already-used one (or two concurrent
  refreshes with the same token) is treated as theft and **revokes the whole token family**, logging that device out.
  Clients must serialise refreshes.
- **Operators** are the phone numbers in `app.operator_phones`; the role is stamped on the session at login and kept on
  refresh. Removing a number takes effect for that person only once their refresh token expires (`jwt.refresh_expiry`,
  30 days by default) or after they log in again; to cut access immediately, also delete their rows from
  `refresh_tokens`.
- **Background jobs** run in the API process: notification dispatch every 5 s, auto-cancel of seller-orders not accepted
  within `orders.accept_window_minutes` every 30 s (stock restored, buyer notified), and the upload sweep hourly. Running
  several API instances is safe (row locks and `SKIP LOCKED`).
- **Push notifications** are queued in the `notifications` table in the same transaction as the change that causes them,
  then sent by the dispatcher in batches of 50: at-least-once (a crash mid-batch resends it), up to 5 attempts, and a
  notification counts as delivered when one of the user's devices accepted it (or the user has none). Tokens FCM
  reports as unregistered are deleted. With `push.driver = "log"` messages are only logged.
- **Uploads.** `POST /v1/me/images` gets a 60 s read/write deadline; every other request is limited by the server's
  15 s timeouts. The hourly sweep deletes uploads never attached and photos no product or store uses any more (replaced
  or removed), once they are 24 h old; a photo still shown in an order's line snapshot is kept.
- **Database timeouts.** Order creation runs under a `statement_timeout`; a query cancelled by it answers
  `503 SERVICE_UNAVAILABLE` with `Retry-After`, which clients may retry (with the same `Idempotency-Key`).
- **Logs.** The `log` OTP sender writes codes to the log; use it in development only.
