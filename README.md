# Druna Server

Backend REST API for the Druna app: users, events, friends, and groups.

## Features

- JWT auth with separate access/refresh tokens, rotation, family revocation, and logout
- Telegram Mini App and Login Widget auth
- Events with overlap validation, filters, pagination, and free-time slots
- Friends with search, incoming/outgoing requests, and re-request after reject
- Groups with members, time confirmation, group free-time, group events, leave/delete
- Unified JSON response envelope
- Prometheus metrics on a separate address, Grafana in Docker, and a health check with DB probe
- GitHub Actions CI (unit + Postgres integration tests)

## Tech stack

Go 1.25 · Gin · PostgreSQL · JWT · Swagger · Docker

## Quick start

```bash
go mod tidy
cp configs/config.yaml.example configs/config.yaml
cp .env.example .env
# edit .env: DB_PASSWORD, JWT_SECRET
go run cmd/main.go
```

Default port: `8000` (from `configs/config.yaml`).

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DB_PASSWORD` | Yes | PostgreSQL password |
| `JWT_SECRET` | Yes | JWT signing key |
| `BOT_TOKEN` | For Telegram | Telegram bot token |
| `TELEGRAM_AUTH_TTL_HOURS` | No | Max initData age in hours (default `24`) |
| `METRICS_ENABLED` | No | Serve Prometheus on `METRICS_ADDR` (default `false`). The public API does not expose metrics |
| `METRICS_ADDR` | No | Metrics listen address (default `127.0.0.1:9090`) |
| `SWAGGER_ENABLED` | No | Expose `/swagger` (default `true`) |
| `TRUSTED_PROXIES` | No | Comma-separated proxy CIDRs for client IP. Empty trusts none |
| `CORS_ORIGINS` | No | Comma-separated origins (default `*`) |
| `DATABASE_URL` | Docker | Used by entrypoint for auto-migrations |
| `TEST_DATABASE_URL` | Tests | DSN for integration tests |

Database host/port/dbname are configured in `configs/config.yaml`.

## Response format

Success:

```json
{ "data": { "accessToken": "...", "refreshToken": "..." }, "error": null }
```

Error:

```json
{ "data": null, "error": { "message": "invalid credentials", "code": 401 } }
```

## Authentication

Sign in:

```bash
curl -X POST http://localhost:8000/auth/sign-in \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secret"}'
```

Use the access token for protected routes:

```
Authorization: Bearer <accessToken>
```

Renew (refresh token rotation):

```bash
curl -X POST http://localhost:8000/auth/renew-token \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":"<refreshToken>"}'
```

Only **access** tokens work on `/api/*`. Refresh tokens are rejected by the auth middleware.

Token TTL: access **15 minutes**, refresh **7 days**. Logout: `POST /auth/logout` with the refresh token revokes that token family.

## Docker

```bash
make build && make up     # app on http://localhost:22000, migrations run on start
make dev-up               # hot reload via air (docker-compose.dev.yml)
make migrate-up           # manual migrations in running compose network
make down
```

On a laptop, Grafana is at `http://127.0.0.1:3000` (`admin` / `druna-dev`). Change `GF_SECURITY_ADMIN_PASSWORD` in compose before anyone else can reach the host. Prometheus scrapes the app on the Docker network (`METRICS_ADDR=0.0.0.0:9090`); that port is not published. A local `go run` with `METRICS_ENABLED=true` listens on `127.0.0.1:9090`.

## Production

`docker-compose.yml` starts Postgres, the API, Prometheus, and Grafana together. Prometheus scrapes `app:9090` inside the `druna-net` network. The metrics port and Prometheus stay off the published ports. Grafana is bound to `127.0.0.1:3000` on the server.

On the host, edit `docker-compose.yml` before the first start:

| Setting | Production value |
|---------|------------------|
| `POSTGRES_PASSWORD` and the password in `DATABASE_URL` | Same strong password |
| `JWT_SECRET` | Long random secret |
| `BOT_TOKEN` | Telegram bot token, if Telegram login is used |
| `CORS_ORIGINS` | Real client origins, comma-separated |
| `SWAGGER_ENABLED` | `false` |
| `TRUSTED_PROXIES` | CIDRs of the reverse proxy, if one sits in front of the API |
| `GF_SECURITY_ADMIN_PASSWORD` | Grafana password other than `druna-dev` |
| `db.ports` | Remove `5432:5432` so Postgres is reachable only on `druna-net` |

Leave `METRICS_ENABLED=true` and `METRICS_ADDR=0.0.0.0:9090`. Keep `9090` and the Prometheus service out of `ports`. Publish the API on `22000` and put TLS in front of that port only.

```bash
docker compose -f docker-compose.yml up -d --build
```

Migrations run when the app container starts (`DATABASE_URL`). Postgres data stays in the `pgdata` volume. The Druna dashboard and the Prometheus datasource are loaded from `deploy/grafana/` on every start.

Check the API on the server:

```bash
curl -sS http://127.0.0.1:22000/ping/
```

Open Grafana from a laptop through an SSH tunnel:

```bash
ssh -L 3000:127.0.0.1:3000 user@your-server
```

Then open `http://127.0.0.1:3000` (`admin` and the password set in compose). The **Druna** dashboard is already there: request rate, p95 latency, 5xx share, and goroutines.

## Testing & quality

```bash
make test                 # all unit tests
make lint                 # golangci-lint
make smoke                # end-to-end curl script (server must be running)
make hook-install         # install git pre-commit hook

# integration tests (Postgres required)
make migrate-up
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/druna_db?sslmode=disable' \
  go test ./tests/integration/... -count=1
```

## Swagger

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/main.go
```

UI: `http://localhost:8000/swagger/index.html`

## API routes

All protected routes are available under **`/api/v1/...`** and legacy **`/api/...`**.

### Public

| Method | Path | Description |
|--------|------|-------------|
| GET | /ping/ | Health check (`status`, `db`) |
| GET | /swagger/* | Swagger UI (`SWAGGER_ENABLED=false` to hide) |
| POST | /auth/sign-up | Register |
| POST | /auth/sign-in | Login |
| POST | /auth/renew-token | Refresh token rotation |
| POST | /auth/logout | Revoke the refresh-token family |
| POST | /auth/telegram | Telegram Mini App login |
| POST | /auth/telegram/widget | Telegram Login Widget |

### Users (auth required)

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/users/me | Current user profile |
| PATCH | /api/v1/users/me | Update name / avatarURL |

### Events (auth required)

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/events/ | List events (`limit`, `offset`, `type`, `dateFrom`, `dateTo`) |
| POST | /api/v1/events/ | Create event |
| PATCH | /api/v1/events/:id | Update event |
| DELETE | /api/v1/events/:id | Delete event |
| POST | /api/v1/events/free-time | Personal free slots for a day (`date`, optional `timezone`) |

### Friends (auth required)

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/friends/list | Friend list |
| GET | /api/v1/friends/search?username= | Search users by username prefix |
| GET | /api/v1/friends/request-list | All pending requests |
| GET | /api/v1/friends/requests/incoming | Incoming requests |
| GET | /api/v1/friends/requests/outgoing | Outgoing requests |
| POST | /api/v1/friends/request | Send request |
| POST | /api/v1/friends/accept | Accept request |
| POST | /api/v1/friends/reject | Reject request |
| DELETE | /api/v1/friends/ | Remove friend |

### Groups (auth required)

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/v1/groups/create | Create group |
| GET | /api/v1/groups/list | List user groups |
| GET | /api/v1/groups/:id | Group details with members |
| DELETE | /api/v1/groups/:id | Delete group (owner only) |
| POST | /api/v1/groups/:id/leave | Leave group |
| POST | /api/v1/groups/:id/members | Add accepted friend (owner only) |
| POST | /api/v1/groups/:id/confirm | Confirm proposed time |
| POST | /api/v1/groups/:id/free-time | Intersection of members' free slots (personal + group events) |
| GET | /api/v1/groups/:id/events | List group events (`limit`, `offset`, `type`, `dateFrom`, `dateTo`) |
| POST | /api/v1/groups/:id/events | Create group event (any member) |
| PATCH | /api/v1/groups/:id/events/:eventId | Update group event (creator or owner) |
| DELETE | /api/v1/groups/:id/events/:eventId | Delete group event (creator or owner) |

## Migrations

Applied automatically in Docker via `docker-entrypoint.sh`, or manually:

```bash
make migrate-up
make migrate-down
```

See [AGENTS.md](AGENTS.md) for development conventions.

## Frontend integration

See [docs/FRONTEND_API.md](docs/FRONTEND_API.md) for the complete API contract for client apps (auth flow, request/response shapes, TypeScript sketch).
