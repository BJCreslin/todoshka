# Todoshka

Web app combining a kanban task board with markdown notes, multi-user with co-authorship sharing.

## Run (development)

```bash
cd ~/IdeaProjects/todoshka
go run .
# open http://localhost:8080
```

Set `TODOSHKA_DB` to override the SQLite path, `TODOSHKA_PORT` to override the dev port (e.g. `:18080`), `TODOSHKA_JWT_SECRET` to override the dev JWT secret.

## Run (single binary)

The frontend is embedded into the binary via `//go:embed`. Build and run anywhere:

```bash
go build -o todoshka .
./todoshka  # uses default :8080
# or with overrides:
TODOSHKA_PORT=:18080 TODOSHKA_DB=/var/lib/todoshka/t.db TODOSHKA_JWT_SECRET=$(openssl rand -hex 32) ./todoshka
```

No `web/` directory needed at runtime — everything is in the binary.

## Docker

```bash
docker build -t todoshka .
docker run -d --name todoshka -p 8080:8080 \
  -v todoshka-data:/app/data \
  -e TODOSHKA_JWT_SECRET=$(openssl rand -hex 32) \
  todoshka
```

## Smoke test (manual)

1. Register user A (e.g. `alice`) in one browser.
2. Register user B (e.g. `bob`) in another browser / incognito.
3. As A: create a task, add subtasks, add a tag.
4. As A: create a note, type Markdown, save.
5. As A: open the task, click "Поделиться", share with `bob`.
6. As B: open `/#/shared` — should see the task. Open it, change status.
7. As A: refresh the task — Bob's status change should be there.
8. As A: edit the note, click "История", restore an earlier version.
9. Search from sidebar — both task and note should appear.

## Tests

```bash
go test ./...
```

## Architecture

- **Backend:** Go 1.21+ with `net/http`, SQLite via `mattn/go-sqlite3`, JWT via `golang-jwt/jwt/v5`.
- **Frontend:** vanilla HTML/CSS/JS, no build step, `marked` for Markdown (vendored).
- **Storage:** single SQLite file at `TODOSHKA_DB` (default `data/todoshka.db`).
- **Auth:** JWT in `Authorization: Bearer` header, 30-day TTL.
- **Sharing:** full co-authorship on tasks and notes via the `shares` table.

See `docs/superpowers/specs/2026-06-12-todoshka-design.md` and `docs/superpowers/plans/2026-06-12-todoshka-implementation.md` for the design and implementation plan.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `TODOSHKA_PORT` | `:8080` | HTTP listen address |
| `TODOSHKA_DB` | `data/todoshka.db` | SQLite file path |
| `TODOSHKA_JWT_SECRET` | dev secret | **Set in production!** 32+ random bytes |

## API summary

| Endpoint | Description |
|---|---|
| `POST /api/register` | Create user, returns `{token, user}` |
| `POST /api/login` | Authenticate, returns `{token, user}` |
| `GET /api/me` | Current user info |
| `GET /api/users?q=` | Search users (for share autocomplete) |
| `GET/POST /api/tasks` | List/create tasks |
| `GET/PATCH/DELETE /api/tasks/{id}` | Single task CRUD |
| `POST/DELETE /api/tasks/{id}/subtasks[/{sid}]` | Subtask add/update/delete |
| `POST/DELETE /api/tasks/{id}/tags[/{tag}]` | Tag add/remove |
| `POST/DELETE /api/tasks/{id}/notes/{nid}` | Note link/unlink |
| `GET /api/tasks/{id}/shares` | List users this task is shared with |
| `GET/POST /api/notes` | List/create notes |
| `GET/PATCH/DELETE /api/notes/{id}` | Single note CRUD |
| `GET /api/notes/{id}/versions` | Version history |
| `POST /api/notes/{id}/restore/{vid}` | Restore version |
| `GET /api/notes/{id}/shares` | List users this note is shared with |
| `POST/DELETE /api/share` | Add/remove share |
| `GET /api/shared` | Resources shared with me |
