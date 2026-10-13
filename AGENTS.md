# AGENTS.md — DrunaServer

Guidance for AI agents and contributors working on this repository.

## Project overview

Go REST API (Gin + PostgreSQL) for users, events, friends, and groups.

```
HTTP → pkg/handler → pkg/service → pkg/repository → PostgreSQL
```

| Layer | Path | Role |
|-------|------|------|
| Entry | `cmd/main.go` | Config, DB, graceful shutdown |
| HTTP | `pkg/handler/` | Routes, middleware, JSON |
| Business | `pkg/service/` | JWT, validation, domain rules |
| Data | `pkg/repository/` | SQL via sqlx |
| Models | `pkg/model/` | Domain structs; Swagger types in `structs_doc.go` |
| Tests | `pkg/*_test.go`, `tests/integration/` | Unit and integration tests |

## Quick start

```bash
go mod tidy
cp configs/config.yaml.example configs/config.yaml
cp .env.example .env
go run cmd/main.go
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DB_PASSWORD` | Yes | PostgreSQL password |
| `JWT_SECRET` | Yes | JWT HMAC signing key |
| `BOT_TOKEN` | Telegram auth | Telegram bot token for initData HMAC |
| `TELEGRAM_AUTH_TTL_HOURS` | No | Max Telegram initData age in hours (default `24`) |
| `METRICS_ENABLED` | No | Expose `/metrics` (default `false`) |
| `SWAGGER_ENABLED` | No | Expose `/swagger` (default `true`) |
| `TRUSTED_PROXIES` | No | Comma-separated proxy CIDRs. Empty means do not trust forwarded headers |
| `CORS_ORIGINS` | No | Comma-separated allowed origins (default `*`) |
| `DATABASE_URL` | Docker | Auto-migrations in `docker-entrypoint.sh` |
| `TEST_DATABASE_URL` | Integration tests | Postgres DSN for `tests/integration` |

Runtime config (port, db host/port/name): `configs/config.yaml`. Docker uses `configs/config.docker.yaml`.

## API conventions

### Versioning

- Preferred: `/api/v1/...`
- Legacy alias: `/api/...` (same handlers)
- Auth: `/auth/...` (not versioned)

### Response envelope

Always use helpers from `pkg/handler/response.go`:

- Success: `Success(c, status, data)` → `{ "data": ..., "error": null }`
- Error: `NewErrorResponse(c, status, message)` → `{ "data": null, "error": { "message", "code" } }`

Do not return raw `gin.H{"error": ...}` in new code.

### Auth

- Access tokens (`token_type=access`, 15 minutes) — required for `/api/*` middleware
- Refresh tokens (`token_type=refresh`, 7 days) — `POST /auth/renew-token` and `POST /auth/logout`
- Refresh rotation claims the old JTI. Reuse revokes the token family (`revoked_token_families`)
- Sign-in/sign-up body: `password` (plaintext); legacy `passwordHash` still accepted; DB column remains `password_hash`
- User ID from JWT middleware (`userCtx`); never trust query/body for identity

### Rate limiting

`/auth/*` routes: 30 requests/minute per IP. Search and free-time: 60 requests/minute per user.

## Swagger

OpenAPI spec is generated from handler annotations (`@Summary`, `@Router`, etc.).

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/main.go
```

Output: `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`. UI: `GET /swagger/index.html`.

Annotation conventions:

- Use `/api/v1/...` paths (legacy `/api/...` is not duplicated in spec)
- Protected routes: `@Security ApiKeyAuth` + `Authorization: Bearer <accessToken>`
- Request/response types: `model.APIResponse` envelope; body types in `pkg/model/` (`structs_doc.go`, domain structs)
- After adding or changing handlers: regenerate Swagger and commit `docs/*`

## Adding a new endpoint

1. Add repository method in `pkg/repository/repository.go` + `*_postgres.go`
2. Add service method in `pkg/service/` + interface in `service.go`
3. Add handler in `pkg/handler/` and register in `registerProtectedRoutes()` or auth group in `handler.go`
4. Add Swagger annotations; run `swag init -g cmd/main.go`
5. Update `README.md` endpoint table and `docs/FRONTEND_API.md` for client-facing changes
6. Add unit tests; integration test if DB logic is involved

## Makefile targets

```bash
make build          # docker-compose build
make up / down      # start/stop containers
make dev-up         # docker-compose.dev.yml with air hot reload
make migrate-up     # run migrations in compose network
make test           # go test ./...
make lint           # golangci-lint
make smoke          # scripts/smoke_test.sh
make hook-install   # set git core.hooksPath to .githooks
```

## Testing

```bash
JWT_SECRET=test-secret-key go test ./pkg/... -count=1
TEST_DATABASE_URL='postgres://...' go test ./tests/integration/... -count=1
```

CI runs unit tests + integration tests with Postgres service (`.github/workflows/ci.yml`).

## Migrations

SQL files in `migrations/`. Format: golang-migrate timestamp_name.{up,down}.sql.

Recent additions:

- `20250617000001` — initial schema (users, events, friends, groups)
- `20250617000002` — unique constraint on `group_members(group_id, user_id)`
- `20250617000003` — `revoked_tokens` table + performance indexes
- `20250617000004` — `notification_outbox` table for companion bot hooks
- `20250617000005` — nullable `events.group_id` (FK to `groups`) + partial index for group events
- `20250617000006` — refresh token `family_id` and `revoked_token_families`
- `20250617000007` — unordered unique pair on `friends`
- `20250617000008` — event and group times as `timestamptz`

If a database was migrated with the old timestamp `20250619011729_migration_init`, update `schema_migrations` manually or recreate the dev DB before applying `20250617000001`.

## Observability

- `GET /ping/` — health + DB ping (`503` if DB down, `status: "degraded"`)
- `GET /metrics` — Prometheus (only when `METRICS_ENABLED=true`)
- `notification_outbox` is read by an external bot, which sets `delivered_at`. This server only enqueues rows.
- Request ID: `X-Request-ID` header (auto-generated if missing)
- Access logs: JSON lines with `request_id`, method, path, status, duration

## Pre-PR checklist

- [ ] `make test`
- [ ] `go vet ./...`
- [ ] `make lint` (or CI green)
- [ ] Swagger regenerated if handlers changed
- [ ] Migration added for schema changes
- [ ] `README.md` updated for new public routes

## Known limitations

- bcrypt password hashing without legacy migration — old users must re-register
- Legacy `/api/*` route prefix mirrors `/api/v1/*` but is omitted from Swagger
- `/swagger` and `/metrics` are public — protect via reverse proxy or env flags in production
