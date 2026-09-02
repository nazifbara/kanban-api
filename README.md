# Kanban API

A REST API for managing Kanban boards, columns, and tasks, with JWT-based
authentication and per-user data isolation (a user can only see and modify
boards they created).

## Tech stack

- **Go** — `net/http` standard library router (Go 1.22+ pattern-based
  `ServeMux`, e.g. `POST /api/boards/{boardID}/columns`)
- **PostgreSQL** via `lib/pq`
- **sqlc** — generates typed Go query code from the SQL in `sql/queries/`
  against the schema in `sql/schema/`
- **goose** — SQL migrations (`-- +goose Up` / `-- +goose Down` blocks in
  `sql/schema/`)
- **JWT** bearer tokens for authentication, plus longer-lived opaque
  refresh tokens
- **UUIDs** as primary keys everywhere (`github.com/google/uuid`)

## Prerequisites

- Go (recent version — the router relies on Go 1.22's method+wildcard
  patterns)
- PostgreSQL
- `sqlc` CLI (query code must be generated before the project builds)
- `goose` CLI, for running migrations
- `curl` and `jq`, if you want to use the `seed.sh` / `e2e_tests.sh` helper
  scripts

## Environment variables

| Variable     | Required | Notes                                                                                             |
| ------------ | -------- | ------------------------------------------------------------------------------------------------- |
| `DB_URL`     | Yes      | Postgres connection string, e.g. `postgres://user:pass@localhost:5432/kanban-db?sslmode=disable`  |
| `JWT_SECRET` | Yes      | The server calls `log.Fatal` on startup if this isn't set. Used to sign and verify access tokens. |
| `ENV`        | No       | Set to `dev` to enable `POST /reset` (see below). Any other value (or unset) disables it.         |

An example `.env` (values are placeholders, not real secrets):

```
DB_URL="postgres://testuser:testpassword@localhost:5432/kanban-db?sslmode=disable"
JWT_SECRET="a-long-random-string"
ENV="dev"
```

## Running locally

```bash
# 1. Generate the sqlc query code
sqlc generate

# 2. Apply migrations
goose postgres "$DB_URL" -dir sql/schema up
# (or ./scripts/migrateup.sh, if present — used in CI/CD)

# 3. Set required env vars and run the server
export DB_URL="postgres://testuser:testpassword@localhost:5432/kanban-db?sslmode=disable"
export JWT_SECRET="a-long-random-string"
export ENV="dev"
go run .
```

By default the server listens on the port passed into `newServer(port, ...)`
at startup (see wherever `main.go` constructs the server).

### Resetting data

```
POST /reset
```

No auth required, but **only works when `ENV=dev`** — any other value
returns `404 Not Found`, so this can't be triggered in production.
Truncates identities (cascading to boards, columns, tasks, and refresh
tokens) — i.e. it wipes **all** data. `204 No Content` on success, `500`
if the truncate fails.

### Seeding & testing

Two helper scripts were put together alongside this API:

- `seed.sh` — signs up/logs in a seed user and creates a handful of sample
  boards, columns, and tasks through the HTTP API. Supports `--reset` to
  wipe the DB first (requires `ENV=dev` on the server), and `BASE_URL` /
  `SEED_EMAIL` / `SEED_PASSWORD` / `SEED_FIRST_NAME` / `SEED_LAST_NAME`
  env vars.
- `e2e_tests.sh` — an end-to-end test suite covering the validation rules,
  auth flows, and access-control edge cases documented below. It calls
  `POST /reset` on startup (requires `ENV=dev`), so only point it at a
  disposable dev database. Supports `--skip-reset` and `BASE_URL`.

  Note: this suite was written against an earlier version of the reset
  endpoint (expecting `200`/no `ENV` gate) and against a task-scoping bug
  that has since been fixed (see below) — a couple of its assertions will
  need updating to match the current behavior (`204` from `/reset`, and
  the cross-board task calls now correctly returning `404` instead of
  succeeding).

## CI/CD

Two GitHub Actions workflows:

- **`ci.yml`** — runs on every PR into `main`:
  - `tests` job: `sqlc generate`, then `go test ./... -cover`, then a
    `gosec` security scan
  - `style` job: `sqlc generate`, then `go fmt` and `staticcheck`
    formatting/lint checks
- **`cd.yml`** — runs on push to `main`:
  1. `sqlc generate`
  2. Runs DB migrations via `./scripts/migrateup.sh` (using the `DB_URL`
     secret)
  3. Authenticates to Google Cloud and builds the image
     (`./scripts/buildprod.sh`)
  4. Submits the image to Artifact Registry and deploys it to **Cloud
     Run** (`kanban-api` service, `us-central1`, max 4 instances,
     unauthenticated access allowed at the Cloud Run layer — the app's
     own auth still applies on top)

## Authentication

Every endpoint except `POST /api/sign-up`, `POST /api/login`,
`POST /api/refresh`, and `POST /reset` requires a bearer token:

```
Authorization: Bearer <access_token>
```

- **Access tokens** are JWTs signed with `JWT_SECRET` and are valid for
  **1 hour**.
- **Refresh tokens** are opaque strings, valid for **30 days**, and are
  exchanged for a new access token via `POST /api/refresh` (sent the same
  way, as a bearer token).
- On sign-up you get a user record back — call `POST /api/login`
  separately to get your first access/refresh token pair.

### Ownership & access control

Boards are scoped to the user who created them (`creator_id`). Any request
touching a board you don't own — directly, or via a column/task nested
under that board — returns `403 Forbidden`. Columns and tasks are in turn
checked against the `{boardID}` in the URL: an ID that exists but belongs
to a _different_ board than the one in the path returns `404 Not Found`
(e.g. `"column not found"` or `"task not found in the board"`). This is
enforced consistently for tasks now, including on
`PATCH /api/boards/{boardID}/tasks/{taskID}` and
`DELETE /api/boards/{boardID}/tasks/{taskID}`, which explicitly verify
`task.BoardID == ctxBoard.ID` before acting.

## Error responses

Errors come back as non-2xx status codes with a JSON body describing the
problem (exact shape depends on `respondWithError`, which wasn't part of
the files reviewed — but expect the message text from the errors listed
below to appear in the body somewhere).

Common status codes:

| Code      | Meaning                                                                                                                                   |
| --------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 400       | Malformed JSON body, or a validation failure (missing/empty required field, out-of-range position, title too long, nil `column_id`, etc.) |
| 401       | Missing/invalid/expired bearer token                                                                                                      |
| 403       | Authenticated, but you don't own the board being accessed                                                                                 |
| 404       | Resource doesn't exist, `ENV != dev` (for `/reset`), or a column/task ID doesn't belong to the board in the path                          |
| 409       | Conflict — e.g. duplicate email on sign-up                                                                                                |
| 500       | Unexpected server/database error                                                                                                          |
| 503 / 504 | Database unavailable / query timed out                                                                                                    |

---

## API Reference

### Auth

#### `POST /api/sign-up`

Create a new user. No auth required.

Request body:

```json
{
  "email": "ada@example.com",
  "password": "correct-horse-battery-staple",
  "first_name": "Ada",
  "last_name": "Lovelace"
}
```

- `email`, `password` required (email must be a valid format)
- `first_name`/`last_name` validation rules weren't visible in the files
  reviewed (validated by an internal `users.ValidateParams` not shared) —
  confirm against your own copy if these are required.

`201 Created`:

```json
{
  "email": "ada@example.com",
  "first_name": "Ada",
  "last_name": "Lovelace",
  "created_at": "...",
  "updated_at": "..."
}
```

Note: sign-up does **not** return a token — log in afterward to get one.
Duplicate email returns `409 Conflict`.

#### `POST /api/login`

Request body:

```json
{ "email": "ada@example.com", "password": "correct-horse-battery-staple" }
```

`200 OK`:

```json
{ "id": "uuid", "token": "jwt...", "refresh_token": "opaque-string" }
```

Wrong password or unknown email → `404 Not Found`.

#### `POST /api/refresh`

Send the **refresh token** as the bearer token:

```
Authorization: Bearer <refresh_token>
```

`201 Created`:

```json
{ "token": "new-jwt..." }
```

Returns `401` if the refresh token is expired or has been revoked, `404`
if it doesn't exist, `400` if the header is missing.

### Boards

All board endpoints require `Authorization: Bearer <access_token>`.

#### `POST /api/boards`

```json
{ "name": "Q3 Product Launch", "description": "optional" }
```

`name` is required. `201 Created` → Board object.

#### `GET /api/boards`

`200 OK` → array of boards owned by the authenticated user only.

#### `GET /api/boards/{boardID}`

`200 OK` → Board. `403` if you don't own it, `404` if it doesn't exist.

#### `PUT /api/boards/{boardID}`

```json
{ "name": "New name" }
```

`name` required. Returns **`201 Created`** on success (not `200`, despite
being an update — that's the current behavior of the handler).

#### `DELETE /api/boards/{boardID}`

`204 No Content`. Cascades to the board's columns and tasks.

### Columns

Nested under a board; all require board ownership.

#### `POST /api/boards/{boardID}/columns`

```json
{ "title": "In Progress", "description": "optional", "position": 0 }
```

- `title` required
- `position` must be in `[0, current column count]` (i.e. you can insert
  anywhere up to and including the end)

`201 Created` → Column object.

#### `GET /api/boards/{boardID}/columns`

`200 OK` → array of columns for the board, ordered by position.

#### `PATCH /api/boards/{boardID}/columns/{columnID}`

```json
{ "title": "optional", "description": "optional", "position": 2 }
```

All fields optional (partial update). If `title` is provided it can't be
empty. If `position` is provided it must be in `[0, current column count - 1]`.
Moving a column shifts the columns in between to keep positions
contiguous. `200 OK` → Column.

#### `DELETE /api/boards/{boardID}/columns/{columnID}`

`204 No Content`. Cascades to the column's tasks and shifts the positions
of the remaining columns down to close the gap.

### Tasks

Creation and listing-by-column are nested under a column; updates,
deletes, and listing-by-board address the task/board directly.

#### `POST /api/boards/{boardID}/columns/{columnID}/tasks`

```json
{ "title": "Write tests", "description": "optional", "position": 0 }
```

- `title` required, max 255 characters
- `position` must be in `[0, current task count in this column]`

`201 Created` → Task object.

#### `GET /api/boards/{boardID}/columns/{columnID}/tasks`

`200 OK` → array of tasks in that column, ordered by position.

#### `GET /api/boards/{boardID}/tasks`

`200 OK` → array of all tasks on the board (across all columns).

#### `PATCH /api/boards/{boardID}/tasks/{taskID}`

```json
{
  "column_id": "optional uuid — move to a different column",
  "title": "optional",
  "description": "optional",
  "position": 1
}
```

- `title`, if provided, can't be empty or exceed 255 characters
- `column_id`, if provided, can't be the nil UUID
  (`00000000-0000-0000-0000-000000000000`) — returns
  `400 "invalid column ID"`
- The task must belong to the board in the path, or this returns
  `404 "task not found in the board"`
- Moving a task to a different column requires that column to belong to
  the same board (`404 "column not found"` otherwise) and shifts
  positions in both the old and new column to stay contiguous
- Changing position within the same column shifts the tasks in between

`200 OK` → Task.

#### `DELETE /api/boards/{boardID}/tasks/{taskID}`

`200 OK` (note: this one returns `200`, not `204` like board/column
deletes). The task must belong to the board in the path, or this returns
`404 "task not found in the board"`. Shifts the positions of the
remaining tasks in that column down to close the gap.

---

## Data models

```ts
Board {
  id, name, description, creator_id, created_at, updated_at
}

Column {
  id, title, description, board_id, creator_id, position, created_at, updated_at
}

Task {
  id, title, description, board_id, column_id, creator_id, position, created_at, updated_at
}

User {
  email, first_name, last_name, created_at, updated_at
}
```
