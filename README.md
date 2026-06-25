# lpwallet

Loyalty Points Wallet backend for Sanlam SFTX. Tracks member accounts and loyalty point transactions with a full audit trail.

---

## Features

- Member accounts with real-time balance tracking
- Earn / spend transaction capture (single, JSON batch, CSV batch)
- Idempotent transactions — duplicate `(reference, account)` pairs are rejected
- Balance guard — spend that would drive balance below zero is rejected
- Audit trail — every write to `user`, `account`, `account_transaction` is logged to `audit_event` + `audit_detail`
- Bearer-token session auth with configurable expiry
- Admin vs member role separation
- PostgreSQL (production) or SQLite (default, zero-config)
- TLS support

---

## Demo 

**Demo Site:** [https://lpwdemo.warky.dev/](https://lpwdemo.warky.dev/)
```
user: admin@localhost
pass: changeme
```

## Quick start

```sh
# SQLite — no database setup needed
make build
ADMIN_PASSWORD=secredt DB_FILE=./lpwallet.db ./bin/lpwallet

# Postgres via Docker Compose
docker compose up
```

Server listens on `http://127.0.0.1:8700` by default.

**API docs:** [http://127.0.0.1:8700/api/v1/swagger/](http://127.0.0.1:8700/api/v1/swagger/)
**OpenAPI spec:** [http://127.0.0.1:8700/api/v1/openapi.json](http://127.0.0.1:8700/api/v1/openapi.json)

---

## Configuration

| Env var | Default | Notes |
|---|---|---|
| `LISTEN_ADDR` | `127.0.0.1:8700` | `host:port` |
| `DB_DRIVER` | auto | `sqlite` or `postgres`; auto-selects `postgres` when `DATABASE_URL` is set |
| `DB_FILE` | `lpwallet.db` | SQLite file path |
| `DATABASE_URL` | — | Full Postgres DSN |
| `DB_HOST` | `localhost` | Postgres only |
| `DB_PORT` | `5432` | Postgres only |
| `DB_NAME` | `lpwallet` | Postgres only |
| `DB_USER` | — | Postgres only |
| `DB_PASSWORD` | — | Postgres only |
| `DB_SSL_MODE` | `prefer` | Postgres only |
| `TLS_CERT_FILE` | — | PEM cert |
| `TLS_KEY_FILE` | — | PEM key |
| `LOG_LEVEL` | `INFO` | `DEBUG` `INFO` `WARN` `ERROR` |
| `ADMIN_EMAIL` | `admin@localhost` | Seeded on first boot if no admin exists |
| `ADMIN_PASSWORD` | — | Required for admin seed |
| `CONFIG_FILE` | `config.yaml` | Optional YAML config path |

YAML config (`config.yaml`) mirrors the env vars under `listen_addr`, `database.*`, `tls.*`, `log_level`, and `admin.*` keys.

---

## API

All endpoints return JSON. Authenticated requests require `Authorization: Bearer <token>`.
Interactive docs: [/api/v1/swagger/](/api/v1/swagger/)

---

## CSV batch format

`POST /api/v1/transactions/batch/csv` — `Content-Type: text/csv`

```csv
ref,account_id,kind,points,occurred_at
tx-001,1,earn,150,2024-06-01T10:00:00
tx-002,1,spend,50,2024-06-02T09:00:00
```

- `kind`: `earn` or `spend`
- `occurred_at`: RFC3339, `2006-01-02T15:04:05`, `2006-01-02 15:04:05`, or `2006-01-02`; defaults to now if empty
- First failure aborts; response includes the row number

---

## Development

```sh
make build      # compile → bin/lpwallet
make test       # go test ./...
make lint       # golangci-lint run
make generate   # regenerate models from sql/model/database_model.dbml
```

Schema changes: edit `sql/model/database_model.dbml` first, then run `make generate`.
