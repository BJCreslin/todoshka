# Todoshka

Web app combining a kanban task board with markdown notes, multi-user with co-authorship sharing.

## Run

```bash
cd ~/IdeaProjects/todoshka
go mod tidy
go run .
# open http://localhost:8080
```

Set `TODOSHKA_DB` to override the SQLite path, `TODOSHKA_PORT` to override the dev port (e.g. `:18080`), `TODOSHKA_JWT_SECRET` to override the dev JWT secret.

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

See `docs/superpowers/specs/2026-06-12-todoshka-design.md` and `docs/superpowers/plans/2026-06-12-todoshka-implementation.md`.
