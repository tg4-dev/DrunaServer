# Druna Mobile API Guide

Contract for **native mobile clients** (iOS / Android — React Native, Flutter, Swift, Kotlin) integrating with **DrunaServer**.

Backend repo: [DrunaServer](https://github.com/TG4-Dev/DrunaServer)  
Related docs: [README.md](../README.md) · [AGENTS.md](../AGENTS.md)

> This guide is written for a standalone mobile app. There is no WebView, no Telegram Mini App SDK, and no browser origin involved. The app talks to the REST API directly over HTTPS and stores JWT tokens in the platform secure store.

---

## Table of contents

1. [Quick start](#quick-start)
2. [Architecture](#architecture)
3. [Base URLs](#base-urls)
4. [Response envelope](#response-envelope)
5. [Headers](#headers)
6. [Authentication](#authentication)
7. [Token storage & session lifecycle](#token-storage--session-lifecycle)
8. [Profile](#profile)
9. [Events](#events)
10. [Friends](#friends)
11. [Groups](#groups)
12. [Public utility routes](#public-utility-routes)
13. [Client architecture](#client-architecture)
14. [Example client (React Native / TypeScript)](#example-client-react-native--typescript)
15. [Push notifications](#push-notifications)
16. [Testing](#testing)
17. [Changelog notes](#changelog-notes)

---

## Quick start

1. Point the app at a reachable API base URL (see [Base URLs](#base-urls) — `localhost` will **not** work from a device/emulator).
2. Register the user: `POST /auth/sign-up`, then `POST /auth/sign-in` to obtain `accessToken` + `refreshToken`.
3. Store both tokens in the platform **secure store** (Keychain / Keystore).
4. Call `/api/v1/*` routes with `Authorization: Bearer <accessToken>`.
5. On `401`, call `POST /auth/renew-token` with the refresh token, save the new pair, and retry once.
6. On refresh failure, clear tokens and send the user back to the sign-in screen.

Everything else (events, friends, groups, group events) is a plain authenticated JSON call.

---

## Architecture

```
Mobile app (iOS / Android)
    | 1. POST /auth/sign-in { username, password }
    v
DrunaServer  -->  validates credentials  -->  returns { accessToken, refreshToken }
    ^
    | 2. Authorization: Bearer <accessToken>
    v
DrunaServer /api/v1/*  (profile, events, friends, groups, group events)
```

- **Transport:** HTTPS + JSON. No cookies, no sessions — auth is stateless JWT in the `Authorization` header.
- **Auth model:** short-lived access token (12h) + rotating refresh token (7d).
- **No CORS considerations:** native apps are not subject to browser CORS. The server's `CORS_ORIGINS` setting only affects web clients.

---

## Base URLs

Native apps cannot reach `localhost` on the developer machine directly. Use the right host for your target:

| Target | API base URL |
|--------|--------------|
| iOS Simulator | `http://localhost:8000` (simulator shares the host network) |
| Android Emulator | `http://10.0.2.2:8000` (special alias to host `localhost`) |
| Physical device (same LAN) | `http://<your-machine-LAN-IP>:8000` (e.g. `http://192.168.1.20:8000`) |
| Docker API | `http://<host>:22000` |
| Staging / Production | `https://api.yourdomain.com` |

Recommendations:

- Keep the base URL in build config / environment (e.g. `.env` + `react-native-config`, Xcode xcconfig, Gradle `buildConfigField`, Flutter `--dart-define`).
- Use **HTTPS in production**. iOS App Transport Security and Android's default network security config block cleartext HTTP for release builds; only allow HTTP for local development.

OpenAPI / Swagger UI for exploring the contract: `{API_BASE}/swagger/index.html`.

---

## Response envelope

Every JSON response follows this shape.

**Success:**

```json
{
  "data": { },
  "error": null
}
```

**Error:**

```json
{
  "data": null,
  "error": {
    "message": "invalid credentials",
    "code": 401
  }
}
```

Client parsing rule: if `error` is non-null, treat it as a failure and surface `error.message`; otherwise use `data`. The `code` mirrors the HTTP status.

---

## Headers

| Header | When |
|--------|------|
| `Content-Type: application/json` | All POST/PATCH bodies |
| `Authorization: Bearer <accessToken>` | All `/api/v1/*` routes |
| `X-Request-ID` | Optional; server echoes it back (or generates one). Useful for correlating client logs with server logs / bug reports |

---

## Authentication

### Token types

| Token | Use | TTL |
|-------|-----|-----|
| `accessToken` | All `/api/v1/*` requests | **15 minutes** |
| `refreshToken` | `POST /auth/renew-token` and `POST /auth/logout` | **7 days** |

- The auth middleware **rejects refresh tokens** on API routes (`401`).
- Refresh tokens are **rotated**: after a successful renew, the old refresh token is claimed. Reusing it revokes the whole token family, including the token that was just issued. Always persist the new pair and discard the old one.
- `/auth/*` routes are rate limited to **30 requests/minute per IP**.
- Search and free-time routes are limited to **60 requests/minute per user**.

### Sign up

`POST /auth/sign-up`

```json
{
  "name": "Alice",
  "username": "alice",
  "email": "alice@example.com",
  "password": "secret123"
}
```

Send plaintext `password` (8–72 bytes). Do not send a pre-hashed value. `telegramID` in the body is ignored. A taken username or email returns **409** `username or email already taken`. Unknown users and wrong passwords both return **401** `invalid credentials`.

**Response `data`:** `{ "id": 1 }`

Sign-up does not return tokens — follow it with a sign-in call (or do both silently during onboarding).

### Sign in

`POST /auth/sign-in`

```json
{
  "username": "alice",
  "password": "secret123"
}
```

**Response `data`:**

```json
{
  "accessToken": "eyJ...",
  "refreshToken": "eyJ..."
}
```

For both sign-up and sign-in, the request field is `password`. Legacy `passwordHash` is still accepted, but new clients should send `password` only.
### Renew tokens

`POST /auth/renew-token`

```json
{ "refreshToken": "eyJ..." }
```

Alternative: send the refresh token as `Authorization: Bearer <refreshToken>`.

**Response `data`:** a new `accessToken` + `refreshToken` pair. Save both.

### Logout

`POST /auth/logout`

```json
{ "refreshToken": "eyJ..." }
```

Revokes the refresh-token family. The current access token stays valid until it expires (at most 15 minutes). **Response `data`:** `{ "message": "logged out" }`.

### Recommended auth flow

```
1. First launch  -> sign-up (optional) -> sign-in -> store { access, refresh } in secure store
2. Each API call -> Authorization: Bearer <accessToken>
3. On 401        -> POST /auth/renew-token -> store new pair -> retry the original request once
4. Renew fails   -> clear tokens -> navigate to sign-in
5. Explicit logout -> POST /auth/logout with the refresh token -> clear the secure store
```

### Telegram login (optional)

Two contracts, both checked with `BOT_TOKEN`:

`POST /auth/telegram` — Telegram Mini App `initData` (`window.Telegram.WebApp.initData`). The secret is HMAC-SHA256 of the bot token with key `WebAppData`.

```json
{ "initData": "query_id=...&user=...&auth_date=...&hash=..." }
```

`POST /auth/telegram/widget` — Telegram Login Widget. The secret is SHA256 of the bot token. Send the widget fields flat:

```json
{
  "id": 123456789,
  "first_name": "Alice",
  "last_name": "Doe",
  "username": "alice",
  "photo_url": "https://t.me/i/userpic/a.jpg",
  "auth_date": 1710000000,
  "hash": "..."
}
```

- Both reject `auth_date` older than `TELEGRAM_AUTH_TTL_HOURS` (default **24h**) and more than one minute in the future.
- On success the server auto-registers or logs in the user and returns `{ accessToken, refreshToken }`.
- If the Telegram username or email is already taken, the account is created as `tg_<id>`.
- Avatar URLs are stored only when they are `https`.

Unless you specifically integrate Telegram login, **use `sign-up` / `sign-in` as the primary auth method.**

---

## Token storage & session lifecycle

Never store tokens in plain files, `AsyncStorage`, or `UserDefaults`/`SharedPreferences` without encryption. Use the platform secure store:

| Stack | Secure storage |
|-------|----------------|
| React Native | `react-native-keychain` or `expo-secure-store` |
| Flutter | `flutter_secure_storage` |
| iOS native | Keychain Services |
| Android native | EncryptedSharedPreferences / Keystore |

Guidance:

- Persist **both** tokens in the secure store so the user stays logged in across cold starts.
- Keep the access token in memory for the app session; read the refresh token from secure storage when you need to renew.
- Wrap all API calls with a single interceptor that adds the `Authorization` header and handles the `401 -> renew -> retry` cycle in one place.
- On explicit logout, call `POST /auth/logout` and delete both tokens from the secure store.

---

## Profile

Prefix: `/api/v1/users`. All routes require auth.

### Get current user

`GET /api/v1/users/me`

**Response `data`:**

```json
{
  "id": 1,
  "name": "Alice",
  "username": "alice",
  "email": "alice@example.com",
  "avatarURL": "https://example.com/avatar.png",
  "telegramID": 123456789
}
```

`telegramID` is omitted for password-only accounts.

### Update profile

`PATCH /api/v1/users/me`

```json
{
  "name": "Alice Updated",
  "avatarURL": "https://example.com/new.png"
}
```

Both fields are optional; omitted fields are unchanged. Image upload is out of scope — send an already-hosted `avatarURL`. Non-empty `avatarURL` must be an `https` URL.

---

## Events

All routes require auth. Prefix: `/api/v1/events`. These are the user's **personal** events (group events live under `/api/v1/groups/:id/events`).

### List events

`GET /api/v1/events/`

Query params (all optional):

| Param | Type | Description |
|-------|------|-------------|
| `limit` | int | Page size, 1–100 (default 50). Other values return **400** |
| `offset` | int | Pagination offset, must be >= 0 |
| `type` | string | Filter by event type |
| `dateFrom` | RFC3339 | Events ending on or after |
| `dateTo` | RFC3339 | Events starting on or before |

**Response `data`:**

```json
{
  "events": [
    {
      "eventID": 1,
      "userID": 1,
      "title": "Meeting",
      "startTime": "2026-06-17T10:00:00Z",
      "endTime": "2026-06-17T11:00:00Z",
      "type": "work"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Create event

`POST /api/v1/events/`

```json
{
  "title": "Meeting",
  "startTime": "2026-06-17T10:00:00Z",
  "endTime": "2026-06-17T11:00:00Z",
  "type": "work"
}
```

**Response `data`:** `{ "eventId": 1 }`

Validation errors (`400`): end before start, or overlapping with an existing personal event.

### Update event

`PATCH /api/v1/events/:id` — same body as create.

**Response `data`:** `{ "message": "event updated" }`

### Delete event

`DELETE /api/v1/events/:id`

**Response `data`:** `{ "message": "event deleted" }`

### Personal free time

`POST /api/v1/events/free-time`

```json
{ "date": "2026-06-17", "timezone": "Europe/Moscow" }
```

Date format: **`YYYY-MM-DD`**. Optional `timezone` is an IANA name (default `UTC`) and selects the calendar day. Busy time includes personal events and group events the user belongs to. Search and this route are limited to 60 requests/minute per user.

**Response `data`:**

```json
{
  "freeSlots": [
    { "start": "2026-06-17T08:00:00Z", "end": "2026-06-17T10:00:00Z" }
  ]
}
```

---

## Friends

Prefix: `/api/v1/friends`. All routes require auth.

| Method | Path | Body | Response `data` |
|--------|------|------|-----------------|
| GET | `/list` | — | `{ "friends": [FriendInfo] }` |
| GET | `/search?username=ali` | — | `{ "users": [FriendInfo] }` |
| GET | `/request-list` | — | all pending |
| GET | `/requests/incoming` | — | incoming pending |
| GET | `/requests/outgoing` | — | outgoing pending |
| POST | `/request` | `{ "username": "bob" }` | `{ "message": "friend request sent" }` |
| POST | `/accept` | `{ "username": "bob" }` | `{ "message": "friend request accepted" }` |
| POST | `/reject` | `{ "username": "bob" }` | `{ "message": "friend request rejected" }` |
| DELETE | `/` | `{ "username": "bob" }` | `{ "message": "friend deleted" }` |

**FriendInfo:** `{ "id": 2, "name": "Bob", "username": "bob" }`

Business rules: no self-request (**400**). Already friends, a duplicate pending request, or a request the other person already sent you returns **409** (`user already sent you a request`). A rejected request can be sent again. Only the recipient can accept or reject (**403**).

---

## Groups

Prefix: `/api/v1/groups`. All routes require auth.

| Method | Path | Body | Notes |
|--------|------|------|-------|
| POST | `/create` | `{ "name": "Weekend trip" }` | Owner = current user; response includes `groupId` |
| GET | `/list` | — | User's groups |
| GET | `/:id` | — | Details + members |
| POST | `/:id/members` | `{ "username": "bob" }` | Owner only; **member must be an accepted friend** |
| POST | `/:id/confirm` | `{ "confirmedTime": "2026-06-20T18:00:00Z" }` | Member confirms. `confirmedTime` on the group is `null` until every member confirms the same instant |
| POST | `/:id/free-time` | `{ "date": "2026-06-20", "timezone": "Europe/Moscow" }` | Shared free slots. `timezone` is optional IANA, default `UTC` |
| POST | `/:id/leave` | — | Not for owner |
| DELETE | `/:id` | — | Owner only |

**Group free-time** returns the intersection of all members' free slots for the day. Busy time for each member combines their **personal events** and the **group events** of every group they belong to, so a group event blocks the corresponding slot.

### Group events

Prefix: `/api/v1/groups/:id/events`. All routes require the caller to be a **member** of the group (non-members get **403**). A group event is a shared event scoped to the group; it does **not** appear in the personal `GET /api/v1/events/` list of its creator.

| Method | Path | Body | Notes |
|--------|------|------|-------|
| GET | `/:id/events` | — (query: `limit`, `offset`, `type`, `dateFrom`, `dateTo`) | List group events. `data` is the same envelope as personal events (`events`, `total`, `limit`, `offset`) |
| POST | `/:id/events` | `{ "title", "startTime", "endTime", "type" }` | Any member creates; response `{ "eventId": N }` |
| PATCH | `/:id/events/:eventId` | same body as create | Only the **creator** or the **group owner** |
| DELETE | `/:id/events/:eventId` | — | Only the **creator** or the **group owner** |

Event object (in list responses):

```json
{
  "eventID": 12,
  "userID": 3,
  "groupID": 7,
  "title": "Team sync",
  "startTime": "2026-06-17T14:00:00Z",
  "endTime": "2026-06-17T16:00:00Z",
  "type": "meeting"
}
```

Rules:

- `startTime`/`endTime`/`title` are required; `endTime` must be after `startTime` (**400**).
- A new/updated group event must not overlap another event **of the same group** (**400**).
- Update/delete by a member who is neither creator nor owner returns **403**; unknown event returns **404**.
- On create, the other group members get a `group_event_created` entry in the server's notification outbox (see [Push notifications](#push-notifications)).

---

## Public utility routes

| Method | Path | Response |
|--------|------|----------|
| GET | `/ping/` | `{ "data": { "status": "ok", "db": "ok" } }` — on DB failure: HTTP **503**, `"status": "degraded"`, `"db": "error"` |
| GET | `/metrics` | Prometheus (not JSON envelope). Off unless `METRICS_ENABLED=true` |
| GET | `/swagger/*` | Swagger UI. On unless `SWAGGER_ENABLED=false` |

Use `GET /ping/` for a lightweight connectivity/health check (e.g. a startup reachability probe or an offline banner).

---

## Client architecture

Recommended layering for a mobile app (framework-agnostic):

```
app/
  api/        # HTTP client: base URL, auth header, 401 -> renew -> retry interceptor
  auth/       # sign-in/up, token secure storage, session state
  models/     # ApiResponse<T>, Event, FriendInfo, Group, GroupDetails, Tokens
  features/   # calendar (events), friends, groups, group events
  ui/         # native screens & navigation
```

Screen-to-endpoint map for an MVP:

| Screen | API used |
|--------|----------|
| Onboarding / sign-in | `POST /auth/sign-up`, `POST /auth/sign-in` |
| Profile | `GET` / `PATCH /api/v1/users/me` |
| My calendar (list) | `GET /api/v1/events/` |
| Create / edit event | `POST` / `PATCH /api/v1/events/:id`, `DELETE /api/v1/events/:id` |
| Day free time | `POST /api/v1/events/free-time` |
| Friends list | `GET /api/v1/friends/list` |
| Friend requests | `GET .../requests/incoming`, `.../outgoing` |
| Search & add friend | `GET .../search?username=`, `POST .../request` |
| Groups list / create | `GET /api/v1/groups/list`, `POST .../create` |
| Group detail | `GET /api/v1/groups/:id` |
| Group scheduling | `POST .../confirm`, `POST .../free-time` |
| Group events | `GET` / `POST /api/v1/groups/:id/events`, `PATCH` / `DELETE .../:eventId` |

If you also build a web client, extract `api` / `models` / `auth` into shared packages; only the `ui` layer needs to differ (native vs web).

---

## Example client (React Native / TypeScript)

The example uses `fetch` (available in React Native) plus a secure-storage abstraction. Swap the storage calls for `react-native-keychain`, `expo-secure-store`, or the native equivalent. The same contract applies to Flutter/Swift/Kotlin — only the syntax differs.

```typescript
type ApiResponse<T> = {
  data: T | null;
  error: { message: string; code: number } | null;
};

type Tokens = { accessToken: string; refreshToken: string };

// Replace with expo-secure-store / react-native-keychain
interface SecureStore {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  remove(key: string): Promise<void>;
}

class DrunaClient {
  private accessToken: string | null = null;

  constructor(
    private baseUrl: string,
    private store: SecureStore
  ) {}

  // --- auth ---

  async signIn(username: string, password: string): Promise<void> {
    const tokens = await this.raw<Tokens>("/auth/sign-in", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    await this.persist(tokens);
  }

  async signUp(input: {
    name: string;
    username: string;
    email: string;
    password: string;
  }): Promise<void> {
    await this.raw("/auth/sign-up", {
      method: "POST",
      body: JSON.stringify(input),
    });
  }

  async logout(): Promise<void> {
    this.accessToken = null;
    await this.store.remove("accessToken");
    await this.store.remove("refreshToken");
  }

  private async persist(tokens: Tokens): Promise<void> {
    this.accessToken = tokens.accessToken;
    await this.store.set("accessToken", tokens.accessToken);
    await this.store.set("refreshToken", tokens.refreshToken);
  }

  private async renew(): Promise<boolean> {
    const refreshToken = await this.store.get("refreshToken");
    if (!refreshToken) return false;
    try {
      const tokens = await this.raw<Tokens>("/auth/renew-token", {
        method: "POST",
        body: JSON.stringify({ refreshToken }),
      });
      await this.persist(tokens);
      return true;
    } catch {
      await this.logout();
      return false;
    }
  }

  // --- low-level request without auth retry ---

  private async raw<T>(path: string, init: RequestInit = {}): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...init.headers },
    });
    const body: ApiResponse<T> = await res.json();
    if (body.error) throw new Error(body.error.message);
    return body.data as T;
  }

  // --- authenticated request with 401 -> renew -> retry ---

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!this.accessToken) {
      this.accessToken = await this.store.get("accessToken");
    }
    const call = async (): Promise<Response> =>
      fetch(`${this.baseUrl}${path}`, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          ...(this.accessToken
            ? { Authorization: `Bearer ${this.accessToken}` }
            : {}),
          ...init.headers,
        },
      });

    let res = await call();
    if (res.status === 401 && (await this.renew())) {
      res = await call();
    }
    const body: ApiResponse<T> = await res.json();
    if (body.error) throw new Error(body.error.message);
    return body.data as T;
  }

  // --- example domain calls ---

  getEvents(params?: Record<string, string>) {
    const q = params ? "?" + new URLSearchParams(params) : "";
    return this.request(`/api/v1/events/${q}`);
  }

  createEvent(event: object) {
    return this.request("/api/v1/events/", {
      method: "POST",
      body: JSON.stringify(event),
    });
  }

  getFreeTime(date: string) {
    return this.request("/api/v1/events/free-time", {
      method: "POST",
      body: JSON.stringify({ date }),
    });
  }

  listGroupEvents(groupId: number, params?: Record<string, string>) {
    const q = params ? "?" + new URLSearchParams(params) : "";
    return this.request(`/api/v1/groups/${groupId}/events${q}`);
  }

  createGroupEvent(groupId: number, event: object) {
    return this.request(`/api/v1/groups/${groupId}/events`, {
      method: "POST",
      body: JSON.stringify(event),
    });
  }
}
```

---

## Push notifications

The server records notification-worthy events in a `notification_outbox` table:

- `friend_request` — someone sent a friend request
- `group_confirm` — a member confirmed a group time
- `group_event_created` — a group event was created (payload: `groupId`, `eventId`, `title`, `startTime`)

There is **no push delivery built into DrunaServer**. A companion bot reads `notification_outbox` and sets `delivered_at` when a row is delivered. Enqueue failures are logged and do not fail the API call. Until a push pipeline exists, the mobile app should surface these by **polling** the relevant list endpoints when it foregrounds or on pull-to-refresh:

- Incoming friend requests: `GET /api/v1/friends/requests/incoming`
- Group state: `GET /api/v1/groups/:id`
- Group events: `GET /api/v1/groups/:id/events`

When a push pipeline is added later, the app will register its APNs/FCM device token via a (future) endpoint; that is not part of the current contract.

---

## Testing

### Run the backend locally

```bash
cp configs/config.yaml.example configs/config.yaml
cp .env.example .env   # set DB_PASSWORD, JWT_SECRET (BOT_TOKEN only if testing Telegram login)
go run cmd/main.go
```

### Smoke test the auth + API path

```bash
make smoke
# or: BASE_URL=http://localhost:8000 make smoke
```

### Manual test from a device/emulator

1. Start the backend and note the reachable base URL for your target (see [Base URLs](#base-urls)).
2. In the app, sign up then sign in; confirm tokens are stored in the secure store.
3. Verify `GET /api/v1/users/me` and `GET /api/v1/events/` return `200`.
4. Kill and relaunch the app — it should restore the session from stored tokens.
5. Wait past access-token expiry (or clear the in-memory token) and confirm the `401 -> renew -> retry` path works transparently.

### Quick manual request (curl)

```bash
# sign in
curl -s -X POST http://localhost:8000/auth/sign-in \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}'

# authenticated call
curl -s http://localhost:8000/api/v1/events/ \
  -H 'Authorization: Bearer <accessToken>'
```

---

## Changelog notes

- All responses use the `{ data, error }` envelope.
- Use the `/api/v1/` prefix for every protected route.
- Primary mobile auth is `POST /auth/sign-up` + `POST /auth/sign-in`; `POST /auth/telegram` is optional (Telegram integration only).
- Access vs refresh tokens — only the access token is valid on API routes.
- Refresh rotation — always save the new token pair after every renew.
- Store tokens in the platform secure store (Keychain / Keystore), never in plaintext.
- Group events live under `/api/v1/groups/:id/events` and are member-scoped.
