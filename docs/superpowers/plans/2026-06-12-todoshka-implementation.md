# Todoshka Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a multi-user web app combining a kanban task board with markdown notes, full co-authorship sharing, and per-note version history.

**Architecture:** Single Go binary serves a vanilla-JS SPA from `web/` and exposes a JSON REST API. SQLite for storage. JWT in `Authorization: Bearer` for auth.

**Tech Stack:** Go 1.21+, `net/http`, `database/sql` + `mattn/go-sqlite3`, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, vanilla HTML/CSS/JS, `marked` for Markdown.

**Reference:** `docs/superpowers/specs/2026-06-12-todoshka-design.md`

---

## File Structure

```
todoshka/
├── go.mod
├── main.go
├── internal/
│   ├── db/
│   │   ├── db.go            # connection + schema migration
│   │   ├── users.go         # User CRUD
│   │   ├── tasks.go         # Task/Subtask/Tag queries
│   │   ├── notes.go         # Note/Version/Tag queries
│   │   └── sharing.go       # Shares + access checks
│   ├── auth/
│   │   ├── password.go      # bcrypt
│   │   ├── jwt.go           # token issue/parse
│   │   └── middleware.go    # context-based auth
│   ├── handlers/
│   │   ├── errors.go        # JSON error helpers
│   │   ├── auth.go          # register/login/me + Mount
│   │   ├── users.go         # user search
│   │   ├── tasks.go         # task CRUD + subtasks + tags + links
│   │   ├── notes.go         # note CRUD + versions + tags + links
│   │   └── share.go         # share/unshare/shared-with-me
│   ├── models/
│   │   └── models.go
│   └── server/
│       └── server.go        # logging middleware
├── web/
│   ├── index.html
│   ├── style.css
│   ├── app.js               # entry
│   ├── api.js store.js router.js
│   ├── components/{layout.js,sidebar.js}
│   ├── views/{login,tasks,task,notes,note,shared,search}.js
│   └── vendor/marked.min.js
└── data/.gitkeep
```

**Boundaries:** `models` = pure data, no I/O. `db` = SQL only, no HTTP/JSON. `auth` = no SQL, no HTTP, no JSON. `handlers` = HTTP+JSON, calls `db` and `auth`. `server` = wiring only.

---

## Task 1: Bootstrap Go module and HTTP server

**Files:**
- Create: `go.mod`, `main.go`, `internal/server/server.go`, `internal/handlers/errors.go`

- [ ] **Step 1: Init module and add deps**

```bash
cd ~/IdeaProjects/todoshka
go mod init github.com/vladimirkreslin/todoshka
go get github.com/mattn/go-sqlite3@latest
go get github.com/golang-jwt/jwt/v5@latest
go get golang.org/x/crypto/bcrypt@latest
```

- [ ] **Step 2: Write `internal/handlers/errors.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIError{Error: msg, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Unauthorized(w http.ResponseWriter, msg string) { writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", msg) }
func Forbidden(w http.ResponseWriter, msg string)    { writeError(w, http.StatusForbidden, "FORBIDDEN", msg) }
func NotFound(w http.ResponseWriter, msg string)     { writeError(w, http.StatusNotFound, "NOT_FOUND", msg) }
func BadRequest(w http.ResponseWriter, code, msg string) { writeError(w, http.StatusBadRequest, code, msg) }
func Conflict(w http.ResponseWriter, code, msg string)   { writeError(w, http.StatusConflict, code, msg) }
func Internal(w http.ResponseWriter, msg string)         { writeError(w, http.StatusInternalServerError, "INTERNAL", msg) }
```

- [ ] **Step 3: Write `internal/server/server.go`**

```go
package server

import (
	"net/http"
	"time"
)

func Wrap(h http.Handler) http.Handler { return logMiddleware(h) }

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		println(r.Method, r.URL.Path, time.Since(start).String())
	})
}
```

- [ ] **Step 4: Write `main.go`**

```go
package main

import (
	"log"
	"net/http"

	"github.com/vladimirkreslin/todoshka/internal/server"
)

func main() {
	addr := ":8080"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	log.Printf("todoshka starting on %s", addr)
	if err := http.ListenAndServe(addr, server.Wrap(mux)); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Build and smoke test**

```bash
go build ./...
go run . &
sleep 1
curl -s http://localhost:8080/api/health
# Expected: {"status":"ok"}
kill %1
```

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum main.go internal/
git commit -m "feat: bootstrap go module with health endpoint"
```

---

## Task 2: Database connection and schema

**Files:**
- Create: `internal/db/db.go`, `data/.gitkeep`

- [ ] **Step 1: Write `internal/db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	raw, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := raw.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := migrate(raw); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{raw}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL, description TEXT,
  status TEXT NOT NULL DEFAULT 'todo',
  priority TEXT NOT NULL DEFAULT 'medium',
  due_date DATE, position INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS subtasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  title TEXT NOT NULL, done BOOLEAN NOT NULL DEFAULT 0,
  position INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag TEXT NOT NULL, PRIMARY KEY (task_id, tag));
CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL, body_md TEXT NOT NULL,
  pinned BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS note_versions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  title TEXT NOT NULL, body_md TEXT NOT NULL,
  editor_id INTEGER NOT NULL REFERENCES users(id),
  saved_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS note_task_links (
  note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  PRIMARY KEY (note_id, task_id));
CREATE TABLE IF NOT EXISTS shares (
  resource_type TEXT NOT NULL, resource_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (resource_type, resource_id, user_id));
`

func migrate(d *sql.DB) error { _, err := d.Exec(schema); return err }
```

- [ ] **Step 2: Wire DB into `main.go`**

Replace `main.go` with:

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/server"
)

func main() {
	dbPath := os.Getenv("TODOSHKA_DB")
	if dbPath == "" { dbPath = "data/todoshka.db" }
	d, err := db.Open(dbPath)
	if err != nil { log.Fatalf("open db: %v", err) }
	defer d.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := d.Ping(); err != nil { w.WriteHeader(http.StatusServiceUnavailable); return }
		w.Write([]byte(`{"status":"ok"}`))
	})
	addr := ":8080"
	log.Printf("todoshka starting on %s (db=%s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, server.Wrap(mux)))
}
```

- [ ] **Step 3: Build, run, verify schema**

```bash
touch data/.gitkeep
go build ./...
go run . &
sleep 1
curl -s http://localhost:8080/api/health
# Expected: {"status":"ok"}
sqlite3 data/todoshka.db ".tables"
# Expected: note_task_links note_versions notes shares subtasks task_tags tasks users
kill %1
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: add sqlite connection and schema migration"
```

---

## Task 3: Domain models

**Files:** Create: `internal/models/models.go`

- [ ] **Step 1: Write the file**

```go
package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
	ID          int64      `json:"id"`
	OwnerID     int64      `json:"owner_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *string    `json:"due_date"`
	Position    int64      `json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Subtasks    []Subtask  `json:"subtasks,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	LinkedNotes []int64    `json:"linked_notes,omitempty"`
}

type Subtask struct {
	ID       int64  `json:"id"`
	TaskID   int64  `json:"task_id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int64  `json:"position"`
}

type Note struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Title       string    `json:"title"`
	BodyMD      string    `json:"body_md"`
	Pinned      bool      `json:"pinned"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
	LinkedTasks []int64   `json:"linked_tasks,omitempty"`
}

type NoteVersion struct {
	ID         int64     `json:"id"`
	NoteID     int64     `json:"note_id"`
	Title      string    `json:"title"`
	BodyMD     string    `json:"body_md"`
	EditorID   int64     `json:"editor_id"`
	SavedAt    time.Time `json:"saved_at"`
	EditorName string    `json:"editor_name,omitempty"`
}
```

- [ ] **Step 2: Build and commit**

```bash
go build ./...
git add internal/models/
git commit -m "feat: add domain models"
```

---

## Task 4: Password hashing and JWT

**Files:** Create: `internal/auth/password.go`, `password_test.go`, `jwt.go`, `jwt_test.go`

- [ ] **Step 1: Write `password_test.go`**

```go
package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil { t.Fatal(err) }
	if hash == "hunter2" { t.Fatal("hash equals plaintext") }
	if !VerifyPassword(hash, "hunter2") { t.Fatal("verify failed for correct") }
	if VerifyPassword(hash, "wrong")     { t.Fatal("verify succeeded for wrong") }
}
```

- [ ] **Step 2: Write `internal/auth/password.go`**

```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
```

- [ ] **Step 3: Write `jwt_test.go`**

```go
package auth

import (
	"testing"
	"time"
)

func TestJWTRoundtrip(t *testing.T) {
	const secret = "secretsecretsecretsecret"
	tok, err := IssueToken(42, "alice", secret, 30*time.Minute)
	if err != nil { t.Fatal(err) }
	uid, uname, err := ParseToken(tok, secret)
	if err != nil { t.Fatal(err) }
	if uid != 42 || uname != "alice" { t.Fatalf("got %d %s", uid, uname) }
}

func TestJWTWrongSecret(t *testing.T) {
	const s = "secretsecretsecretsecret"
	tok, _ := IssueToken(1, "x", s, time.Minute)
	if _, _, err := ParseToken(tok, "differentdifferentdifferent"); err == nil {
		t.Fatal("expected wrong-secret error")
	}
}

func TestJWTExpired(t *testing.T) {
	const s = "secretsecretsecretsecret"
	tok, _ := IssueToken(1, "x", s, -time.Minute)
	if _, _, err := ParseToken(tok, s); err == nil {
		t.Fatal("expected expired error")
	}
}
```

- [ ] **Step 4: Write `internal/auth/jwt.go`**

```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"un"`
	jwt.RegisteredClaims
}

func IssueToken(userID int64, username, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID, Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (int64, string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("bad alg: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil { return 0, "", err }
	c, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid { return 0, "", errors.New("invalid claims") }
	return c.UserID, c.Username, nil
}
```

- [ ] **Step 5: Run all auth tests**

```bash
go test ./internal/auth/...
# Expected: PASS (4 tests)
```

- [ ] **Step 6: Commit**

```bash
git add internal/auth/
git commit -m "feat: add password hashing and jwt"
```

---

## Task 5: Auth middleware

**Files:** Create: `internal/auth/middleware.go`, `middleware_test.go`

- [ ] **Step 1: Write `middleware_test.go`**

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testKey = "user"

func TestMiddlewareRejectsMissing(t *testing.T) {
	h := RequireUser("secretsecretsecretsecret", testKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusUnauthorized { t.Fatalf("got %d, want 401", rec.Code) }
}

func TestMiddlewareAcceptsValid(t *testing.T) {
	tok, _ := IssueToken(7, "bob", "secretsecretsecretsecret", 60_000_000_000)
	var id int64; var name string
	h := RequireUser("secretsecretsecretsecret", testKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context(), testKey)
		id, name = u.ID, u.Username
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("got %d", rec.Code) }
	if id != 7 || name != "bob" { t.Fatalf("got %d %s", id, name) }
}
```

- [ ] **Step 2: Write `internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"
	"strings"
)

type CtxUser struct {
	ID       int64
	Username string
}

func RequireUser(secret string, key any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, `{"error":"missing token","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}
			uid, uname, err := ParseToken(strings.TrimPrefix(h, "Bearer "), secret)
			if err != nil {
				http.Error(w, `{"error":"invalid token","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), key, CtxUser{ID: uid, Username: uname})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context, key any) CtxUser {
	v, _ := ctx.Value(key).(CtxUser)
	return v
}
```

- [ ] **Step 3: Run all auth tests and commit**

```bash
go test ./internal/auth/...
# Expected: PASS
git add internal/auth/
git commit -m "feat: add auth middleware"
```

---

## Task 6: User DB layer (Create, Get, Search)

**Files:** Create: `internal/db/users.go`, `users_test.go`

- [ ] **Step 1: Write `users_test.go`**

```go
package db

import "testing"

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUserCreateAndGet(t *testing.T) {
	d := newTestDB(t)
	id, err := d.CreateUser("alice", "$2a$10$fakehash")
	if err != nil { t.Fatal(err) }
	u, err := d.GetUserByUsername("alice")
	if err != nil { t.Fatal(err) }
	if u.ID != id || u.Username != "alice" { t.Fatalf("%+v", u) }
}

func TestUserDuplicate(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.CreateUser("bob", "x"); err != nil { t.Fatal(err) }
	if _, err := d.CreateUser("bob", "y"); err == nil { t.Fatal("expected dup") }
}

func TestUserSearch(t *testing.T) {
	d := newTestDB(t)
	d.CreateUser("alice", "x"); d.CreateUser("alicia", "x"); d.CreateUser("bob", "x")
	got, err := d.SearchUsers("ali")
	if err != nil { t.Fatal(err) }
	if len(got) != 2 { t.Fatalf("got %d: %+v", len(got), got) }
}
```

- [ ] **Step 2: Write `internal/db/users.go`**

```go
package db

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

var ErrNotFound = errors.New("not found")

func (d *DB) CreateUser(username, passwordHash string) (int64, error) {
	res, err := d.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") { return 0, errors.New("username taken") }
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := d.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	return &u, nil
}

func (d *DB) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := d.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	return &u, nil
}

func (d *DB) SearchUsers(q string) ([]models.User, error) {
	rows, err := d.Query(`SELECT id, username, '' AS password_hash, created_at FROM users WHERE username LIKE ? ORDER BY username LIMIT 20`, q+"%")
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil { return nil, err }
		out = append(out, u)
	}
	return out, rows.Err()
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./internal/db/...
# Expected: PASS
git add internal/db/
git commit -m "feat: add user db layer with search"
```

---

## Task 7: Auth handlers (register, login, me) + Mount

**Files:** Create: `internal/handlers/auth.go`, `auth_test.go`. Modify: `main.go`.

- [ ] **Step 1: Write `auth_test.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/db"
)

func setup(t *testing.T) (*db.DB, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { d.Close() })
	return d, "test-jwt-secret-test-jwt-secret"
}

func registerAs(t *testing.T, mux http.Handler, username, password string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	var r struct{ Token string }
	json.Unmarshal(rec.Body.Bytes(), &r)
	return r.Token
}

func TestRegisterAndLogin(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"alice","password":"hunter2hunter2"}`)))
	if rec.Code != http.StatusCreated { t.Fatalf("register: %d %s", rec.Code, rec.Body.String()) }

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"alice","password":"hunter2hunter2"}`)))
	if rec.Code != http.StatusOK { t.Fatalf("login: %d %s", rec.Code, rec.Body.String()) }
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"x","password":"short"}`)))
	if rec.Code != http.StatusBadRequest { t.Fatalf("got %d", rec.Code) }
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"a","password":"hunter2hunter2"}`)))
		if i == 0 && rec.Code != 201 { t.Fatalf("first: %d", rec.Code) }
		if i == 1 && rec.Code != 409 { t.Fatalf("second: %d", rec.Code) }
	}
}
```

- [ ] **Step 2: Write `internal/handlers/auth.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
)

const userCtxKey = "user"
const MinPasswordLen = 8

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type publicUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}
type tokenResp struct {
	Token string     `json:"token"`
	User  publicUser `json:"user"`
}

func Mount(mux *http.ServeMux, d *db.DB, secret string) {
	mountAuth(mux, d, secret)
	mountUsers(mux, d, secret)
	mountTasks(mux, d, secret)
	mountNotes(mux, d, secret)
	mountShare(mux, d, secret)
}

func mountAuth(mux *http.ServeMux, d *db.DB, secret string) {
	mux.HandleFunc("POST /api/register", registerHandler(d, secret))
	mux.HandleFunc("POST /api/login", loginHandler(d, secret))
	mux.Handle("GET /api/me", auth.RequireUser(secret, userCtxKey)(http.HandlerFunc(meHandler)))
}

func registerHandler(d *db.DB, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequest(w, "INVALID_JSON", "invalid json body"); return
		}
		if len(req.Username) < 3 || len(req.Username) > 32 {
			BadRequest(w, "INVALID_USERNAME", "username must be 3-32 chars"); return
		}
		if len(req.Password) < MinPasswordLen {
			BadRequest(w, "WEAK_PASSWORD", "password must be at least 8 chars"); return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil { Internal(w, "hash"); return }
		id, err := d.CreateUser(req.Username, hash)
		if err != nil {
			if err.Error() == "username taken" { Conflict(w, "USERNAME_TAKEN", "username already exists"); return }
			Internal(w, "create user"); return
		}
		tok, _ := auth.IssueToken(id, req.Username, secret, 30*24*time.Hour)
		writeJSON(w, http.StatusCreated, tokenResp{Token: tok, User: publicUser{ID: id, Username: req.Username}})
	}
}

func loginHandler(d *db.DB, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequest(w, "INVALID_JSON", "invalid json body"); return
		}
		u, err := d.GetUserByUsername(req.Username)
		if err != nil { Unauthorized(w, "invalid credentials"); return }
		if !auth.VerifyPassword(u.PasswordHash, req.Password) { Unauthorized(w, "invalid credentials"); return }
		tok, _ := auth.IssueToken(u.ID, u.Username, secret, 30*24*time.Hour)
		writeJSON(w, http.StatusOK, tokenResp{Token: tok, User: publicUser{ID: u.ID, Username: u.Username}})
	}
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context(), userCtxKey)
	writeJSON(w, http.StatusOK, publicUser{ID: u.ID, Username: u.Username})
}
```

- [ ] **Step 3: Add stubs for the other `mount*` functions (so the package compiles)**

Add temporary stubs to `auth.go` (we'll replace them in later tasks):

```go
func mountUsers(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountTasks(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountNotes(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountShare(mux *http.ServeMux, d *db.DB, secret string)  {}
```

- [ ] **Step 4: Wire handlers into `main.go`**

Add to `main.go`, after the `/api/health` route:

```go
secret := os.Getenv("TODOSHKA_JWT_SECRET")
if secret == "" { secret = "dev-secret-change-me-please-32bytes" }
import_helpers := handlers.Mount
_ = import_helpers
```

Replace the `_ = import_helpers` with the actual call:

```go
handlers.Mount(mux, d, secret)
```

And add the import at the top of `main.go`:

```go
"github.com/vladimirkreslin/todoshka/internal/handlers"
```

- [ ] **Step 5: Run all tests**

```bash
go test ./...
# Expected: PASS
```

- [ ] **Step 6: Smoke test**

```bash
go run . &
sleep 1
curl -s -X POST http://localhost:8080/api/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"hunter2hunter2"}'
# Expected: {"token":"...","user":{"id":1,"username":"alice"}}
kill %1
```

- [ ] **Step 7: Commit**

```bash
git add .
git commit -m "feat: add register/login/me handlers"
```

---

## Task 8: Users search handler

**Files:** Modify: `internal/handlers/auth.go` (replace `mountUsers` stub)

- [ ] **Step 1: Create `internal/handlers/users.go`**

```go
package handlers

import (
	"net/http"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
)

func mountUsers(mux *http.ServeMux, d *db.DB, secret string) {
	mux.Handle("GET /api/users", auth.RequireUser(secret, userCtxKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" { writeJSON(w, http.StatusOK, []any{}); return }
		users, err := d.SearchUsers(q)
		if err != nil { Internal(w, "search users"); return }
		out := make([]publicUser, 0, len(users))
		for _, u := range users { out = append(out, publicUser{ID: u.ID, Username: u.Username}) }
		writeJSON(w, http.StatusOK, out)
	})))
}
```

- [ ] **Step 2: Build and smoke test**

```bash
go build ./...
go run . &
sleep 1
TOK=$(curl -s -X POST http://localhost:8080/api/register -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"hunter2hunter2"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
curl -s "http://localhost:8080/api/users?q=bo" -H "Authorization: Bearer $TOK"
# Expected: [{"id":1,"username":"bob"}]
kill %1
```

- [ ] **Step 3: Commit**

```bash
git add internal/handlers/users.go
git commit -m "feat: add user search endpoint"
```

---

## Task 9: Task DB layer (CRUD, list, access control, subtasks, tags)

**Files:** Create: `internal/db/tasks.go`, `tasks_test.go`

- [ ] **Step 1: Write `tasks_test.go`**

```go
package db

import (
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

func TestTaskCreateAndList(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, err := d.CreateTask(uid, &models.Task{Title: "Buy milk", Status: "todo", Priority: "high"})
	if err != nil { t.Fatal(err) }
	list, _ := d.ListTasksForUser(uid, TaskFilter{})
	if len(list) != 1 || list[0].Title != "Buy milk" || list[0].ID != id {
		t.Fatalf("got %+v", list)
	}
}

func TestTaskUpdateAndDelete(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, _ := d.CreateTask(uid, &models.Task{Title: "A"})
	desc := "new"
	if err := d.UpdateTask(id, uid, map[string]any{"description": desc, "status": "done"}); err != nil { t.Fatal(err) }
	got, _ := d.GetTask(id, uid)
	if got.Status != "done" || got.Description == nil || *got.Description != desc { t.Fatalf("%+v", got) }
	if err := d.DeleteTask(id, uid); err != nil { t.Fatal(err) }
	if _, err := d.GetTask(id, uid); err == nil { t.Fatal("expected not found") }
}

func TestTaskAccessControl(t *testing.T) {
	d := newTestDB(t)
	owner, _ := d.CreateUser("alice", "x")
	other, _ := d.CreateUser("bob", "x")
	id, _ := d.CreateTask(owner, &models.Task{Title: "A"})
	if _, err := d.GetTask(id, other); err == nil { t.Fatal("bob should not see alice's task") }
	if err := d.Share("task", id, other); err != nil { t.Fatal(err) }
	if _, err := d.GetTask(id, other); err != nil { t.Fatal("bob should see after share") }
}

func TestSubtasksAndTags(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	tid, _ := d.CreateTask(uid, &models.Task{Title: "A", Tags: []string{"work", "urgent"}})
	sid, err := d.CreateSubtask(tid, uid, "step 1")
	if err != nil { t.Fatal(err) }
	if err := d.UpdateSubtask(sid, uid, map[string]any{"done": true}); err != nil { t.Fatal(err) }
	task, _ := d.GetTask(tid, uid)
	if !task.Subtasks[0].Done { t.Fatal("not done") }
	if len(task.Tags) != 2 { t.Fatalf("tags: %+v", task.Tags) }
	if err := d.AddTaskTag(tid, uid, "home"); err != nil { t.Fatal(err) }
	if err := d.RemoveTaskTag(tid, uid, "work"); err != nil { t.Fatal(err) }
	if err := d.DeleteSubtask(sid, uid); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Write `internal/db/tasks.go`**

```go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

type TaskFilter struct {
	Status string
	Tag    string
	Q      string
}

func defaultStr(s, d string) string { if s == "" { return d }; return s }

func (d *DB) CreateTask(ownerID int64, t *models.Task) (int64, error) {
	pos, err := d.nextTaskPosition(ownerID, defaultStr(t.Status, "todo"))
	if err != nil { return 0, err }
	res, err := d.Exec(`INSERT INTO tasks (owner_id, title, description, status, priority, due_date, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ownerID, t.Title, t.Description, defaultStr(t.Status, "todo"),
		defaultStr(t.Priority, "medium"), t.DueDate, pos)
	if err != nil { return 0, err }
	id, _ := res.LastInsertId()
	if len(t.Tags) > 0 { if err := d.setTaskTags(id, t.Tags); err != nil { return 0, err } }
	return id, nil
}

func (d *DB) nextTaskPosition(ownerID int64, status string) (int64, error) {
	var maxPos sql.NullInt64
	err := d.QueryRow(`SELECT MAX(position) FROM tasks WHERE owner_id = ? AND status = ?`, ownerID, status).Scan(&maxPos)
	if err != nil { return 0, err }
	if !maxPos.Valid { return 0, nil }
	return maxPos.Int64 + 1, nil
}

func (d *DB) GetTask(id, userID int64) (*models.Task, error) {
	if !d.userHasTaskAccess(id, userID) { return nil, ErrNotFound }
	var t models.Task
	err := d.QueryRow(`SELECT id, owner_id, title, description, status, priority, due_date, position, created_at, updated_at
		FROM tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.Position, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	_ = d.loadTaskRelations(&t)
	return &t, nil
}

func (d *DB) loadTaskRelations(t *models.Task) error {
	rows, err := d.Query(`SELECT id, task_id, title, done, position FROM subtasks WHERE task_id = ? ORDER BY position, id`, t.ID)
	if err != nil { return err }
	for rows.Next() {
		var s models.Subtask
		if err := rows.Scan(&s.ID, &s.TaskID, &s.Title, &s.Done, &s.Position); err != nil { rows.Close(); return err }
		t.Subtasks = append(t.Subtasks, s)
	}
	rows.Close()
	tagRows, err := d.Query(`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, t.ID)
	if err != nil { return err }
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil { tagRows.Close(); return err }
		t.Tags = append(t.Tags, tag)
	}
	tagRows.Close()
	linkRows, err := d.Query(`SELECT note_id FROM note_task_links WHERE task_id = ? ORDER BY note_id`, t.ID)
	if err != nil { return err }
	for linkRows.Next() {
		var nid int64
		if err := linkRows.Scan(&nid); err != nil { linkRows.Close(); return err }
		t.LinkedNotes = append(t.LinkedNotes, nid)
	}
	linkRows.Close()
	return nil
}

func (d *DB) ListTasksForUser(userID int64, f TaskFilter) ([]models.Task, error) {
	q := `SELECT id, owner_id, title, description, status, priority, due_date, position, created_at, updated_at
		FROM tasks
		WHERE (owner_id = ? OR id IN (SELECT resource_id FROM shares WHERE resource_type='task' AND user_id=?))`
	args := []any{userID, userID}
	if f.Status != "" { q += ` AND status = ?`; args = append(args, f.Status) }
	if f.Tag != "" { q += ` AND id IN (SELECT task_id FROM task_tags WHERE tag = ?)`; args = append(args, f.Tag) }
	if f.Q != "" { q += ` AND title LIKE ?`; args = append(args, "%"+f.Q+"%") }
	q += ` ORDER BY status, position, id`
	rows, err := d.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.Position, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTask(id, userID int64, fields map[string]any) error {
	if !d.userHasTaskAccess(id, userID) { return ErrNotFound }
	allowed := map[string]bool{"title": true, "description": true, "status": true, "priority": true, "due_date": true, "position": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE tasks SET "+strings.Join(sets, ", ")+", updated_at = CURRENT_TIMESTAMP WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteTask(id, userID int64) error {
	if !d.userIsTaskOwner(id, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (d *DB) userHasTaskAccess(taskID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM tasks t
		WHERE t.id = ? AND (t.owner_id = ? OR EXISTS (
			SELECT 1 FROM shares s WHERE s.resource_type='task' AND s.resource_id=t.id AND s.user_id=?
		))`, taskID, userID, userID).Scan(&n)
	return n > 0
}

func (d *DB) userIsTaskOwner(taskID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ? AND owner_id = ?`, taskID, userID).Scan(&n)
	return n > 0
}

func (d *DB) setTaskTags(taskID int64, tags []string) error {
	tx, err := d.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM task_tags WHERE task_id = ?`, taskID); err != nil { return err }
	for _, t := range tags {
		if t == "" { continue }
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, t); err != nil { return err }
	}
	return tx.Commit()
}

func (d *DB) CreateSubtask(taskID, userID int64, title string) (int64, error) {
	if !d.userHasTaskAccess(taskID, userID) { return 0, ErrNotFound }
	var maxPos sql.NullInt64
	_ = d.QueryRow(`SELECT MAX(position) FROM subtasks WHERE task_id = ?`, taskID).Scan(&maxPos)
	pos := int64(0)
	if maxPos.Valid { pos = maxPos.Int64 + 1 }
	res, err := d.Exec(`INSERT INTO subtasks (task_id, title, position) VALUES (?, ?, ?)`, taskID, title, pos)
	if err != nil { return 0, err }
	return res.LastInsertId()
}

func (d *DB) UpdateSubtask(id, userID int64, fields map[string]any) error {
	var taskID int64
	if err := d.QueryRow(`SELECT task_id FROM subtasks WHERE id = ?`, id).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }
		return err
	}
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	allowed := map[string]bool{"title": true, "done": true, "position": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE subtasks SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteSubtask(id, userID int64) error {
	var taskID int64
	if err := d.QueryRow(`SELECT task_id FROM subtasks WHERE id = ?`, id).Scan(&taskID); err != nil { return ErrNotFound }
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM subtasks WHERE id = ?`, id)
	return err
}

func (d *DB) AddTaskTag(taskID, userID int64, tag string) error {
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, tag)
	return err
}

func (d *DB) RemoveTaskTag(taskID, userID int64, tag string) error {
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag = ?`, taskID, tag)
	return err
}
```

- [ ] **Step 3: Add minimal `sharing.go` stub so the access test compiles**

Create `internal/db/sharing.go` (we'll extend in Task 13):

```go
package db

func (d *DB) Share(resourceType string, resourceID, userID int64) error {
	_, err := d.Exec(`INSERT OR IGNORE INTO shares (resource_type, resource_id, user_id) VALUES (?, ?, ?)`,
		resourceType, resourceID, userID)
	return err
}

func (d *DB) Unshare(resourceType string, resourceID, userID int64) error {
	_, err := d.Exec(`DELETE FROM shares WHERE resource_type = ? AND resource_id = ? AND user_id = ?`,
		resourceType, resourceID, userID)
	return err
}

func (d *DB) IsOwner(resourceType string, resourceID, userID int64) (bool, error) {
	var n int
	var q string
	switch resourceType {
	case "task": q = `SELECT COUNT(*) FROM tasks WHERE id = ? AND owner_id = ?`
	case "note": q = `SELECT COUNT(*) FROM notes WHERE id = ? AND owner_id = ?`
	default:     return false, nil
	}
	if err := d.QueryRow(q, resourceID, userID).Scan(&n); err != nil { return false, err }
	return n > 0, nil
}

func (d *DB) userHasNoteAccess(noteID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM notes n
		WHERE n.id = ? AND (n.owner_id = ? OR EXISTS (
			SELECT 1 FROM shares s WHERE s.resource_type='note' AND s.resource_id=n.id AND s.user_id=?
		))`, noteID, userID, userID).Scan(&n)
	return n > 0
}

func (d *DB) LinkNoteToTask(noteID, taskID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) || !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO note_task_links (note_id, task_id) VALUES (?, ?)`, noteID, taskID)
	return err
}

func (d *DB) UnlinkNoteFromTask(noteID, taskID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM note_task_links WHERE note_id = ? AND task_id = ?`, noteID, taskID)
	return err
}

type SharedItem struct {
	Type string
	ID   int64
}

func (d *DB) ListSharedWithUser(userID int64) ([]SharedItem, error) {
	rows, err := d.Query(`SELECT resource_type, resource_id FROM shares WHERE user_id = ?`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []SharedItem
	for rows.Next() {
		var s SharedItem
		if err := rows.Scan(&s.Type, &s.ID); err != nil { return nil, err }
		out = append(out, s)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run tests and commit**

```bash
go test ./internal/db/...
# Expected: PASS
git add internal/db/
git commit -m "feat: add task db layer with access control and subtasks"
```

---

## Task 10: Task handlers (CRUD + subtasks + tags + links)

**Files:** Create: `internal/handlers/tasks.go`, `tasks_test.go`. Modify: `internal/handlers/auth.go` (replace `mountTasks` stub).

- [ ] **Step 1: Write `internal/handlers/tasks.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/models"
)

func mountTasks(mux *http.ServeMux, d *db.DB, secret string) {
	auth := auth.RequireUser(secret, userCtxKey)
	mux.Handle("GET /api/tasks", auth(http.HandlerFunc(taskList(d))))
	mux.Handle("POST /api/tasks", auth(http.HandlerFunc(taskCreate(d))))
	mux.Handle("GET /api/tasks/{id}", auth(http.HandlerFunc(taskGet(d))))
	mux.Handle("PATCH /api/tasks/{id}", auth(http.HandlerFunc(taskUpdate(d))))
	mux.Handle("DELETE /api/tasks/{id}", auth(http.HandlerFunc(taskDelete(d))))
	mux.Handle("POST /api/tasks/{id}/subtasks", auth(http.HandlerFunc(subtaskAdd(d))))
	mux.Handle("PATCH /api/tasks/{id}/subtasks/{sid}", auth(http.HandlerFunc(subtaskUpdate(d))))
	mux.Handle("DELETE /api/tasks/{id}/subtasks/{sid}", auth(http.HandlerFunc(subtaskDelete(d))))
	mux.Handle("POST /api/tasks/{id}/tags", auth(http.HandlerFunc(taskAddTag(d))))
	mux.Handle("DELETE /api/tasks/{id}/tags/{tag}", auth(http.HandlerFunc(taskRemoveTag(d))))
	mux.Handle("POST /api/tasks/{id}/notes/{nid}", auth(http.HandlerFunc(taskLinkNote(d))))
	mux.Handle("DELETE /api/tasks/{id}/notes/{nid}", auth(http.HandlerFunc(taskUnlinkNote(d))))
}

func taskList(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		f := db.TaskFilter{Status: r.URL.Query().Get("status"), Tag: r.URL.Query().Get("tag"), Q: r.URL.Query().Get("q")}
		tasks, err := d.ListTasksForUser(u.ID, f)
		if err != nil { Internal(w, "list"); return }
		if tasks == nil { tasks = []models.Task{} }
		writeJSON(w, http.StatusOK, tasks)
	}
}

func taskCreate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		var t models.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if strings.TrimSpace(t.Title) == "" { BadRequest(w, "INVALID_TITLE", "title required"); return }
		id, err := d.CreateTask(u.ID, &t)
		if err != nil { Internal(w, "create"); return }
		task, _ := d.GetTask(id, u.ID)
		writeJSON(w, http.StatusCreated, task)
	}
}

func taskGet(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		task, err := d.GetTask(id, u.ID)
		if err != nil { NotFound(w, "task not found"); return }
		writeJSON(w, http.StatusOK, task)
	}
}

func taskUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if err := d.UpdateTask(id, u.ID, body); err != nil {
			if err == db.ErrNotFound { NotFound(w, "task not found"); return }
			Internal(w, "update"); return
		}
		task, _ := d.GetTask(id, u.ID)
		writeJSON(w, http.StatusOK, task)
	}
}

func taskDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		if err := d.DeleteTask(id, u.ID); err != nil { NotFound(w, "task not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func subtaskAdd(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r); if !ok { return }
		var body struct{ Title string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" { BadRequest(w, "INVALID_BODY", "title required"); return }
		sid, err := d.CreateSubtask(tid, u.ID, body.Title)
		if err != nil { NotFound(w, "task not found"); return }
		writeJSON(w, http.StatusCreated, map[string]any{"id": sid, "task_id": tid, "title": body.Title, "done": false})
	}
}

func subtaskUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		sid, err := strconv.ParseInt(r.PathValue("sid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad id"); return }
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if err := d.UpdateSubtask(sid, u.ID, body); err != nil { NotFound(w, "subtask not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func subtaskDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		sid, _ := strconv.ParseInt(r.PathValue("sid"), 10, 64)
		if err := d.DeleteSubtask(sid, u.ID); err != nil { NotFound(w, "subtask not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskAddTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r); if !ok { return }
		var body struct{ Tag string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" { BadRequest(w, "INVALID_BODY", "tag required"); return }
		if err := d.AddTaskTag(tid, u.ID, body.Tag); err != nil { NotFound(w, "task not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskRemoveTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r); if !ok { return }
		if err := d.RemoveTaskTag(tid, u.ID, r.PathValue("tag")); err != nil { NotFound(w, "task not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskLinkNote(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r); if !ok { return }
		nid, err := strconv.ParseInt(r.PathValue("nid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad note id"); return }
		if err := d.LinkNoteToTask(nid, tid, u.ID); err != nil { NotFound(w, "task or note not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskUnlinkNote(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r); if !ok { return }
		nid, _ := strconv.ParseInt(r.PathValue("nid"), 10, 64)
		if err := d.UnlinkNoteFromTask(nid, tid, u.ID); err != nil { NotFound(w, "task or note not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil { BadRequest(w, "INVALID_ID", "id must be an integer"); return 0, false }
	return id, true
}
```

- [ ] **Step 2: Write `internal/handlers/tasks_test.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func createTask(t *testing.T, mux http.Handler, tok, body string) int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 { t.Fatalf("create: %d %s", rec.Code, rec.Body.String()) }
	var r struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &r)
	return r.ID
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

func TestTaskCRUDFlow(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux(); Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")
	tid := createTask(t, mux, tok, `{"title":"Buy milk","priority":"high"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("list: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/api/tasks/"+itoa(tid), strings.NewReader(`{"status":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("update: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 { t.Fatalf("delete: %d", rec.Code) }
}

func TestSubtasksAndTags(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux(); Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")
	tid := createTask(t, mux, tok, `{"title":"A"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tasks/"+itoa(tid)+"/subtasks", strings.NewReader(`{"title":"step 1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 { t.Fatalf("subtask: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/tasks/"+itoa(tid)+"/tags", strings.NewReader(`{"tag":"work"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 { t.Fatalf("tag add: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("get: %d", rec.Code) }
	if !strings.Contains(rec.Body.String(), `"tags":["work"]`) { t.Fatalf("body: %s", rec.Body.String()) }
}
```

- [ ] **Step 3: Run all tests and commit**

```bash
go test ./...
# Expected: PASS
git add .
git commit -m "feat: add task handlers with subtasks and tags"
```

---

## Task 11: Note DB layer (CRUD, list, version history, restore, tags)

**Files:** Create: `internal/db/notes.go`, `notes_test.go`

- [ ] **Step 1: Write `notes_test.go`**

```go
package db

import (
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

func TestNoteCreateAndVersioning(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, err := d.CreateNote(uid, &models.Note{Title: "Hi", BodyMD: "# Hello"})
	if err != nil { t.Fatal(err) }
	if err := d.UpdateNote(id, uid, map[string]any{"title": "Hi v2", "body_md": "## Updated"}, uid); err != nil { t.Fatal(err) }
	vs, _ := d.ListNoteVersions(id, uid)
	if len(vs) != 1 { t.Fatalf("expected 1 version, got %d", len(vs)) }
	if vs[0].Title != "Hi" { t.Fatalf("version should hold old title, got %q", vs[0].Title) }
	if err := d.RestoreNoteVersion(id, vs[0].ID, uid); err != nil { t.Fatal(err) }
	n, _ := d.GetNote(id, uid)
	if n.Title != "Hi" { t.Fatalf("restore failed, got %q", n.Title) }
}

func TestNotePinAndTag(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, _ := d.CreateNote(uid, &models.Note{Title: "A", BodyMD: "x", Tags: []string{"work"}})
	if err := d.UpdateNote(id, uid, map[string]any{"pinned": true}, uid); err != nil { t.Fatal(err) }
	if err := d.AddNoteTag(id, uid, "home"); err != nil { t.Fatal(err) }
	if err := d.RemoveNoteTag(id, uid, "work"); err != nil { t.Fatal(err) }
	n, _ := d.GetNote(id, uid)
	if !n.Pinned { t.Fatal("not pinned") }
	if len(n.Tags) != 1 || n.Tags[0] != "home" { t.Fatalf("tags: %+v", n.Tags) }
}

func TestNoteListFilters(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	d.CreateNote(uid, &models.Note{Title: "Apples", BodyMD: "x"})
	d.CreateNote(uid, &models.Note{Title: "Oranges", BodyMD: "x"})
	d.CreateNote(uid, &models.Note{Title: "Mangoes", BodyMD: "x"})
	got, err := d.ListNotesForUser(uid, NoteFilter{Q: "an"})
	if err != nil { t.Fatal(err) }
	if len(got) < 2 { t.Fatalf("expected >=2, got %d", len(got)) }
}
```

- [ ] **Step 2: Write `internal/db/notes.go`**

```go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

type NoteFilter struct {
	Q      string
	Tag    string
	Pinned *bool
}

func (d *DB) CreateNote(ownerID int64, n *models.Note) (int64, error) {
	if strings.TrimSpace(n.Title) == "" { return 0, errors.New("title required") }
	// Body may be empty — user might fill it in later.
	res, err := d.Exec(`INSERT INTO notes (owner_id, title, body_md, pinned) VALUES (?, ?, ?, ?)`,
		ownerID, n.Title, n.BodyMD, n.Pinned)
	if err != nil { return 0, err }
	id, _ := res.LastInsertId()
	if len(n.Tags) > 0 { if err := d.setNoteTags(id, n.Tags); err != nil { return 0, err } }
	return id, nil
}

func (d *DB) GetNote(id, userID int64) (*models.Note, error) {
	if !d.userHasNoteAccess(id, userID) { return nil, ErrNotFound }
	var n models.Note
	err := d.QueryRow(`SELECT id, owner_id, title, body_md, pinned, created_at, updated_at
		FROM notes WHERE id = ?`, id).Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMD, &n.Pinned, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	_ = d.loadNoteRelations(&n)
	return &n, nil
}

func (d *DB) loadNoteRelations(n *models.Note) error {
	tagRows, err := d.Query(`SELECT tag FROM note_tags WHERE note_id = ? ORDER BY tag`, n.ID)
	if err != nil { return err }
	for tagRows.Next() {
		var t string
		if err := tagRows.Scan(&t); err != nil { tagRows.Close(); return err }
		n.Tags = append(n.Tags, t)
	}
	tagRows.Close()
	linkRows, err := d.Query(`SELECT task_id FROM note_task_links WHERE note_id = ? ORDER BY task_id`, n.ID)
	if err != nil { return err }
	for linkRows.Next() {
		var tid int64
		if err := linkRows.Scan(&tid); err != nil { linkRows.Close(); return err }
		n.LinkedTasks = append(n.LinkedTasks, tid)
	}
	linkRows.Close()
	return nil
}

func boolInt(b bool) int { if b { return 1 }; return 0 }

func (d *DB) ListNotesForUser(userID int64, f NoteFilter) ([]models.Note, error) {
	q := `SELECT id, owner_id, title, body_md, pinned, created_at, updated_at
		FROM notes
		WHERE (owner_id = ? OR id IN (SELECT resource_id FROM shares WHERE resource_type='note' AND user_id=?))`
	args := []any{userID, userID}
	if f.Q != "" {
		q += ` AND (title LIKE ? OR body_md LIKE ?)`
		like := "%" + f.Q + "%"
		args = append(args, like, like)
	}
	if f.Tag != "" { q += ` AND id IN (SELECT note_id FROM note_tags WHERE tag = ?)`; args = append(args, f.Tag) }
	if f.Pinned != nil { q += ` AND pinned = ?`; args = append(args, boolInt(*f.Pinned)) }
	q += ` ORDER BY pinned DESC, updated_at DESC`
	rows, err := d.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.Note
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMD, &n.Pinned, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *DB) UpdateNote(id, userID int64, fields map[string]any, editorID int64) error {
	if !d.userHasNoteAccess(id, userID) { return ErrNotFound }
	var title, body string
	if err := d.QueryRow(`SELECT title, body_md FROM notes WHERE id = ?`, id).Scan(&title, &body); err != nil { return err }
	if _, err := d.Exec(`INSERT INTO note_versions (note_id, title, body_md, editor_id) VALUES (?, ?, ?, ?)`, id, title, body, editorID); err != nil {
		return err
	}
	allowed := map[string]bool{"title": true, "body_md": true, "pinned": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE notes SET "+strings.Join(sets, ", ")+", updated_at = CURRENT_TIMESTAMP WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteNote(id, userID int64) error {
	if !d.userIsNoteOwner(id, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM notes WHERE id = ?`, id)
	return err
}

func (d *DB) userIsNoteOwner(noteID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ? AND owner_id = ?`, noteID, userID).Scan(&n)
	return n > 0
}

func (d *DB) ListNoteVersions(noteID, userID int64) ([]models.NoteVersion, error) {
	if !d.userHasNoteAccess(noteID, userID) { return nil, ErrNotFound }
	rows, err := d.Query(`SELECT v.id, v.note_id, v.title, v.body_md, v.editor_id, v.saved_at, u.username
		FROM note_versions v LEFT JOIN users u ON u.id = v.editor_id
		WHERE v.note_id = ? ORDER BY v.saved_at DESC`, noteID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.NoteVersion
	for rows.Next() {
		var v models.NoteVersion
		var username sql.NullString
		if err := rows.Scan(&v.ID, &v.NoteID, &v.Title, &v.BodyMD, &v.EditorID, &v.SavedAt, &username); err != nil {
			return nil, err
		}
		if username.Valid { v.EditorName = username.String }
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) RestoreNoteVersion(noteID, versionID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	var title, body string
	if err := d.QueryRow(`SELECT title, body_md FROM note_versions WHERE id = ? AND note_id = ?`, versionID, noteID).Scan(&title, &body); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }
		return err
	}
	return d.UpdateNote(noteID, userID, map[string]any{"title": title, "body_md": body}, userID)
}

func (d *DB) AddNoteTag(noteID, userID int64, tag string) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO note_tags (note_id, tag) VALUES (?, ?)`, noteID, tag)
	return err
}

func (d *DB) RemoveNoteTag(noteID, userID int64, tag string) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM note_tags WHERE note_id = ? AND tag = ?`, noteID, tag)
	return err
}

func (d *DB) setNoteTags(noteID int64, tags []string) error {
	tx, err := d.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM note_tags WHERE note_id = ?`, noteID); err != nil { return err }
	for _, t := range tags {
		if t == "" { continue }
		if _, err := tx.Exec(`INSERT OR IGNORE INTO note_tags (note_id, tag) VALUES (?, ?)`, noteID, t); err != nil { return err }
	}
	return tx.Commit()
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./internal/db/...
# Expected: PASS
git add internal/db/notes.go internal/db/notes_test.go
git commit -m "feat: add note db layer with version history"
```

---

## Task 12: Note handlers (CRUD, list, get, update, delete, versions, restore, tags, links)

**Files:** Create: `internal/handlers/notes.go`, `notes_test.go`. Modify: `internal/handlers/auth.go` (replace `mountNotes` stub).

- [ ] **Step 1: Write `internal/handlers/notes.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/models"
)

func mountNotes(mux *http.ServeMux, d *db.DB, secret string) {
	auth := auth.RequireUser(secret, userCtxKey)
	mux.Handle("GET /api/notes", auth(http.HandlerFunc(noteList(d))))
	mux.Handle("POST /api/notes", auth(http.HandlerFunc(noteCreate(d))))
	mux.Handle("GET /api/notes/{id}", auth(http.HandlerFunc(noteGet(d))))
	mux.Handle("PATCH /api/notes/{id}", auth(http.HandlerFunc(noteUpdate(d))))
	mux.Handle("DELETE /api/notes/{id}", auth(http.HandlerFunc(noteDelete(d))))
	mux.Handle("GET /api/notes/{id}/versions", auth(http.HandlerFunc(noteVersions(d))))
	mux.Handle("POST /api/notes/{id}/restore/{vid}", auth(http.HandlerFunc(noteRestore(d))))
	mux.Handle("POST /api/notes/{id}/tags", auth(http.HandlerFunc(noteAddTag(d))))
	mux.Handle("DELETE /api/notes/{id}/tags/{tag}", auth(http.HandlerFunc(noteRemoveTag(d))))
	mux.Handle("POST /api/notes/{id}/tasks/{tid}", auth(http.HandlerFunc(noteLinkTask(d))))
	mux.Handle("DELETE /api/notes/{id}/tasks/{tid}", auth(http.HandlerFunc(noteUnlinkTask(d))))
}

func noteList(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		f := db.NoteFilter{Q: r.URL.Query().Get("q"), Tag: r.URL.Query().Get("tag")}
		if v := r.URL.Query().Get("pinned"); v != "" {
			b := v == "true" || v == "1"
			f.Pinned = &b
		}
		notes, err := d.ListNotesForUser(u.ID, f)
		if err != nil { Internal(w, "list"); return }
		if notes == nil { notes = []models.Note{} }
		writeJSON(w, http.StatusOK, notes)
	}
}

func noteCreate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		var n models.Note
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if strings.TrimSpace(n.Title) == "" { BadRequest(w, "INVALID_TITLE", "title required"); return }
		id, err := d.CreateNote(u.ID, &n)
		if err != nil { BadRequest(w, "INVALID_BODY", err.Error()); return }
		note, _ := d.GetNote(id, u.ID)
		writeJSON(w, http.StatusCreated, note)
	}
}

func noteGet(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		note, err := d.GetNote(id, u.ID)
		if err != nil { NotFound(w, "note not found"); return }
		writeJSON(w, http.StatusOK, note)
	}
}

func noteUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if err := d.UpdateNote(id, u.ID, body, u.ID); err != nil { NotFound(w, "note not found"); return }
		note, _ := d.GetNote(id, u.ID)
		writeJSON(w, http.StatusOK, note)
	}
}

func noteDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		if err := d.DeleteNote(id, u.ID); err != nil { NotFound(w, "note not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteVersions(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		vs, err := d.ListNoteVersions(id, u.ID)
		if err != nil { NotFound(w, "note not found"); return }
		if vs == nil { vs = []models.NoteVersion{} }
		writeJSON(w, http.StatusOK, vs)
	}
}

func noteRestore(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		vid, err := strconv.ParseInt(r.PathValue("vid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad version id"); return }
		if err := d.RestoreNoteVersion(id, vid, u.ID); err != nil { NotFound(w, "version not found"); return }
		note, _ := d.GetNote(id, u.ID)
		writeJSON(w, http.StatusOK, note)
	}
}

func noteAddTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		var body struct{ Tag string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" { BadRequest(w, "INVALID_BODY", "tag required"); return }
		if err := d.AddNoteTag(id, u.ID, body.Tag); err != nil { NotFound(w, "note not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteRemoveTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		if err := d.RemoveNoteTag(id, u.ID, r.PathValue("tag")); err != nil { NotFound(w, "note not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteLinkTask(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		nid, ok := parseID(w, r); if !ok { return }
		tid, err := strconv.ParseInt(r.PathValue("tid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad task id"); return }
		if err := d.LinkNoteToTask(nid, tid, u.ID); err != nil { NotFound(w, "note or task not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteUnlinkTask(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		nid, ok := parseID(w, r); if !ok { return }
		tid, _ := strconv.ParseInt(r.PathValue("tid"), 10, 64)
		if err := d.UnlinkNoteFromTask(nid, tid, u.ID); err != nil { NotFound(w, "note or task not found"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 2: Write `internal/handlers/notes_test.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoteFlowWithVersions(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux(); Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(`{"title":"First","body_md":"# Hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 { t.Fatalf("create: %d", rec.Code) }
	var n struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &n)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/api/notes/"+itoa(n.ID), strings.NewReader(`{"title":"Second","body_md":"# v2"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("update: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/notes/"+itoa(n.ID)+"/versions", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	var versions []struct{ ID int64; Title string }
	json.Unmarshal(rec.Body.Bytes(), &versions)
	if len(versions) != 1 || versions[0].Title != "First" {
		t.Fatalf("expected 1 version titled First, got %+v", versions)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/notes/"+itoa(n.ID)+"/restore/"+itoa(versions[0].ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("restore: %d", rec.Code) }
	var restored struct{ Title string }
	json.Unmarshal(rec.Body.Bytes(), &restored)
	if restored.Title != "First" { t.Fatalf("got %q", restored.Title) }
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./...
# Expected: PASS
git add .
git commit -m "feat: add note handlers with versions and tags"
```

---

## Task 13: Share handler (POST /api/share, DELETE /api/share, GET /api/shared)

**Files:** Create: `internal/handlers/share.go`. Modify: `internal/handlers/auth.go` (replace `mountShare` stub).

- [ ] **Step 1: Write `internal/handlers/share.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
)

func mountShare(mux *http.ServeMux, d *db.DB, secret string) {
	auth := auth.RequireUser(secret, userCtxKey)
	mux.Handle("POST /api/share", auth(http.HandlerFunc(shareCreate(d))))
	mux.Handle("DELETE /api/share", auth(http.HandlerFunc(shareDelete(d))))
	mux.Handle("GET /api/shared", auth(http.HandlerFunc(sharedList(d))))
}

func shareCreate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		var body struct {
			ResourceType string `json:"resource_type"`
			ResourceID   int64  `json:"resource_id"`
			Username     string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if body.ResourceType != "task" && body.ResourceType != "note" {
			BadRequest(w, "INVALID_TYPE", "resource_type must be task or note"); return
		}
		owner, _ := d.IsOwner(body.ResourceType, body.ResourceID, u.ID)
		if !owner { Forbidden(w, "only the owner can share"); return }
		target, err := d.GetUserByUsername(body.Username)
		if err != nil { NotFound(w, "user not found"); return }
		if err := d.Share(body.ResourceType, body.ResourceID, target.ID); err != nil { Internal(w, "share"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func shareDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		var body struct {
			ResourceType string `json:"resource_type"`
			ResourceID   int64  `json:"resource_id"`
			UserID       int64  `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		owner, _ := d.IsOwner(body.ResourceType, body.ResourceID, u.ID)
		if !owner { Forbidden(w, "only the owner can unshare"); return }
		if err := d.Unshare(body.ResourceType, body.ResourceID, body.UserID); err != nil { Internal(w, "unshare"); return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func sharedList(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		items, err := d.ListSharedWithUser(u.ID)
		if err != nil { Internal(w, "list shared"); return }
		type out struct {
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		}
		var result []out
		for _, it := range items {
			switch it.Type {
			case "task":
				if t, err := d.GetTask(it.ID, u.ID); err == nil { result = append(result, out{Type: "task", Data: t}) }
			case "note":
				if n, err := d.GetNote(it.ID, u.ID); err == nil { result = append(result, out{Type: "note", Data: n}) }
			}
		}
		if result == nil { result = []out{} }
		writeJSON(w, http.StatusOK, result)
	}
}
```

- [ ] **Step 2: Add share test to `internal/handlers/tasks_test.go`**

Append:

```go
func TestShareFlow(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux(); Mount(mux, d, secret)
	tokA := registerAs(t, mux, "alice", "hunter2hunter2")
	tokB := registerAs(t, mux, "bob", "bobpassbobpass")
	tid := createTask(t, mux, tokA, `{"title":"A"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 { t.Fatalf("expected 404, got %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/share", strings.NewReader(`{"resource_type":"task","resource_id":`+itoa(tid)+`,"username":"bob"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokA)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 { t.Fatalf("share: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("expected 200, got %d", rec.Code) }
}
```

- [ ] **Step 3: Run all tests and commit**

```bash
go test ./...
# Expected: PASS
git add .
git commit -m "feat: add share handler with co-authorship"
```

---

## Task 14: Serve static frontend

**Files:** Modify: `main.go`. Create: `web/index.html`, `web/style.css`, `web/app.js`, `web/vendor/.gitkeep`.

- [ ] **Step 1: Create placeholders**

`web/index.html`:

```html
<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Todoshka</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <div id="app">Loading…</div>
  <script type="module" src="/app.js"></script>
</body>
</html>
```

`web/style.css`:

```css
* { box-sizing: border-box; }
body { margin: 0; font-family: system-ui, -apple-system, sans-serif; color: #222; }
a { color: #2563eb; }
```

`web/app.js`:

```js
document.getElementById('app').textContent = 'Todoshka';
```

`web/vendor/.gitkeep`: empty file.

- [ ] **Step 2: Add static file server to `main.go`**

Add (before `handlers.Mount(...)`):

```go
mux.Handle("GET /", http.FileServer(http.Dir("web")))
```

Make sure this is the **last** `Handle` call (or it shadows other GETs) — place it after `handlers.Mount`.

- [ ] **Step 3: Smoke test**

```bash
go run . &
sleep 1
curl -s http://localhost:8080/ | head -3
# Expected: <!DOCTYPE html>...
curl -s http://localhost:8080/app.js | head -1
# Expected: document.getElementById('app').textContent = 'Todoshka';
kill %1
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: serve static frontend from /"
```

---

## Task 15: Vendor marked (Markdown parser)

**Files:** Create: `web/vendor/marked.min.js`

- [ ] **Step 1: Download and verify**

```bash
curl -sL https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js -o web/vendor/marked.min.js
head -c 80 web/vendor/marked.min.js
# Expected: starts with /* or similar JS comment
wc -c web/vendor/marked.min.js
# Expected: > 20000 bytes
```

- [ ] **Step 2: Commit**

```bash
git add web/vendor/marked.min.js
git commit -m "chore: vendor marked@12.0.2"
```

---

## Task 16: Frontend — api.js, store.js, router.js

**Files:** Create: `web/api.js`, `web/store.js`, `web/router.js`. Modify: `web/app.js`.

- [ ] **Step 1: Write `web/store.js`**

```js
export const store = {
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  token: localStorage.getItem('token') || '',
  setSession(user, token) {
    this.user = user; this.token = token;
    localStorage.setItem('user', JSON.stringify(user));
    localStorage.setItem('token', token);
  },
  clear() {
    this.user = null; this.token = '';
    localStorage.removeItem('user'); localStorage.removeItem('token');
  },
  isAuthed() { return !!this.token; },
};
```

- [ ] **Step 2: Write `web/api.js`**

```js
import { store } from './store.js';

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (store.token) headers['Authorization'] = 'Bearer ' + store.token;
  const res = await fetch(path, {
    method, headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    store.clear(); location.hash = '#/login';
    throw new Error('unauthorized');
  }
  if (res.status === 204) return null;
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
  return data;
}

const q = (params) => '?' + new URLSearchParams(params).toString();
const enc = encodeURIComponent;

export const api = {
  register: (username, password) => request('POST', '/api/register', { username, password }),
  login:    (username, password) => request('POST', '/api/login',    { username, password }),
  me:       ()                   => request('GET',  '/api/me'),
  searchUsers: (s)               => request('GET',  '/api/users?q=' + enc(s)),

  listTasks:   (f)               => request('GET',  '/api/tasks' + (f ? q(f) : '')),
  createTask:  (b)               => request('POST', '/api/tasks', b),
  getTask:     (id)              => request('GET',  '/api/tasks/' + id),
  updateTask:  (id, b)           => request('PATCH','/api/tasks/' + id, b),
  deleteTask:  (id)              => request('DELETE','/api/tasks/' + id),
  addSubtask:  (id, title)       => request('POST', '/api/tasks/' + id + '/subtasks', { title }),
  addTaskTag:  (id, tag)         => request('POST', '/api/tasks/' + id + '/tags', { tag }),
  removeTaskTag: (id, tag)       => request('DELETE','/api/tasks/' + id + '/tags/' + enc(tag)),
  linkNoteToTask: (tid, nid)     => request('POST', '/api/tasks/' + tid + '/notes/' + nid),

  listNotes:   (f)               => request('GET',  '/api/notes' + (f ? q(f) : '')),
  createNote:  (b)               => request('POST', '/api/notes', b),
  getNote:     (id)              => request('GET',  '/api/notes/' + id),
  updateNote:  (id, b)           => request('PATCH','/api/notes/' + id, b),
  deleteNote:  (id)              => request('DELETE','/api/notes/' + id),
  versions:    (id)              => request('GET',  '/api/notes/' + id + '/versions'),
  restore:     (id, vid)         => request('POST', '/api/notes/' + id + '/restore/' + vid),
  addNoteTag:  (id, tag)         => request('POST', '/api/notes/' + id + '/tags', { tag }),
  linkTaskToNote: (nid, tid)     => request('POST', '/api/notes/' + nid + '/tasks/' + tid),

  share:   (resource_type, resource_id, username) => request('POST',   '/api/share', { resource_type, resource_id, username }),
  unshare: (resource_type, resource_id, user_id)  => request('DELETE', '/api/share', { resource_type, resource_id, user_id }),
  shared:  ()                                      => request('GET',    '/api/shared'),
};
```

- [ ] **Step 3: Write `web/router.js`**

```js
const routes = [];

export function route(pattern, handler) { routes.push({ pattern, handler }); }

export function startRouter(rootEl) {
  async function run() {
    const hash = location.hash.slice(1) || '/';
    for (const r of routes) {
      const m = match(r.pattern, hash);
      if (m) {
        rootEl.innerHTML = '<div class="loading">Loading…</div>';
        try { await r.handler(m, rootEl); }
        catch (e) { rootEl.innerHTML = `<div class="error">${esc(e.message)}</div>`; }
        return;
      }
    }
    rootEl.innerHTML = '<div class="error">404 — page not found</div>';
  }
  window.addEventListener('hashchange', run);
  run();
}

function match(pattern, path) {
  const p = pattern.split('/').filter(Boolean);
  const a = path.split('/').filter(Boolean);
  if (p.length !== a.length) return null;
  const params = {};
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith('{')) params[p[i].slice(1, -1)] = decodeURIComponent(a[i]);
    else if (p[i] !== a[i]) return null;
  }
  return params;
}

export function escapeHtml(s) {
  return String(s).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

export function go(path) { location.hash = '#' + path; }

function esc(s) { return escapeHtml(s); }
```

- [ ] **Step 4: Replace `web/app.js` with imports**

```js
import { startRouter } from './router.js';
import { store } from './store.js';
import './views/login.js';
import './views/tasks.js';
import './views/task.js';
import './views/notes.js';
import './views/note.js';
import './views/shared.js';
import './views/search.js';

const root = document.getElementById('app');
if (!store.isAuthed() && !location.hash.startsWith('#/login') && !location.hash.startsWith('#/register')) {
  location.hash = '#/login';
} else if (store.isAuthed() && (location.hash === '' || location.hash === '#/')) {
  location.hash = '#/tasks';
}
startRouter(root);
```

- [ ] **Step 5: Create empty view stubs so imports work**

`web/views/login.js`:

```js
import { route, go, escapeHtml } from '../router.js';
import { store } from '../store.js';
import { api } from '../api.js';

route('/login',    () => render());
route('/register', () => render());

async function render() {
  const app = document.getElementById('app');
  const isRegister = location.hash === '#/register';
  app.innerHTML = `
    <div class="auth">
      <h1>${isRegister ? 'Создать аккаунт' : 'Войти'}</h1>
      <form id="authForm">
        <input name="username" placeholder="Имя пользователя" required minlength="3" maxlength="32" autocomplete="username">
        <input name="password" type="password" placeholder="Пароль (мин. 8)" required minlength="8" autocomplete="${isRegister ? 'new-password' : 'current-password'}">
        <button type="submit">${isRegister ? 'Создать' : 'Войти'}</button>
      </form>
      <p>${isRegister ? 'Уже есть аккаунт?' : 'Нет аккаунта?'}
        <a href="#/${isRegister ? 'login' : 'register'}">${isRegister ? 'Войти' : 'Создать'}</a>
      </p>
    </div>`;
  document.getElementById('authForm').onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const fn = isRegister ? api.register : api.login;
    try {
      const r = await fn(f.username.value, f.password.value);
      store.setSession(r.user, r.token);
      go('/tasks');
    } catch (err) { alert(err.message); }
  };
}
```

For the rest, create stubs (one each):

`web/views/tasks.js`:

```js
import { route } from '../router.js';
import { layout, bindLayout } from '../components/layout.js';
route('/tasks', async () => {
  document.getElementById('app').innerHTML = layout('/tasks', '<h2>Задачи</h2><p>В разработке…</p>');
  bindLayout();
});
```

`web/views/task.js`:

```js
import { route } from '../router.js';
route('/tasks/{id}', (p) => {
  document.getElementById('app').innerHTML = `<h2>Задача #${p.id}</h2>`;
});
```

`web/views/notes.js`:

```js
import { route } from '../router.js';
route('/notes', () => { document.getElementById('app').innerHTML = '<h2>Заметки</h2>'; });
```

`web/views/note.js`:

```js
import { route } from '../router.js';
route('/notes/{id}', (p) => {
  document.getElementById('app').innerHTML = `<h2>Заметка #${p.id}</h2>`;
});
```

`web/views/shared.js`:

```js
import { route } from '../router.js';
route('/shared', () => { document.getElementById('app').innerHTML = '<h2>Общие</h2>'; });
```

`web/views/search.js`:

```js
import { route } from '../router.js';
route('/search', () => { document.getElementById('app').innerHTML = '<h2>Поиск</h2>'; });
```

- [ ] **Step 6: Create layout/sidebar placeholders so the imports work**

`web/components/layout.js`:

```js
import { renderSidebar, bindSidebar } from './sidebar.js';
export function layout(currentPath, content) {
  return `<div class="layout">${renderSidebar(currentPath)}<main class="content">${content}</main></div>`;
}
export function bindLayout() { bindSidebar(); }
```

`web/components/sidebar.js`:

```js
import { store } from '../store.js';
import { go } from '../router.js';

export function renderSidebar(currentPath) {
  return `
    <aside class="sidebar">
      <div class="me"><strong>${esc(store.user?.username || '')}</strong>
        <button id="logout">Выйти</button>
      </div>
      <nav>
        <a href="#/tasks"  class="${currentPath === '/tasks'  ? 'active' : ''}">📋 Задачи</a>
        <a href="#/notes"  class="${currentPath === '/notes'  ? 'active' : ''}">📝 Заметки</a>
        <a href="#/shared" class="${currentPath === '/shared' ? 'active' : ''}">👥 Общие</a>
      </nav>
      <form id="searchForm" class="search">
        <input name="q" placeholder="Поиск…">
        <button type="submit">Найти</button>
      </form>
    </aside>`;
}

export function bindSidebar() {
  document.getElementById('logout')?.addEventListener('click', () => { store.clear(); go('/login'); });
  document.getElementById('searchForm')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const q = e.target.q.value.trim();
    if (q) go('/search?q=' + encodeURIComponent(q));
  });
}

function esc(s) { return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }
```

- [ ] **Step 7: Append CSS for layout and auth**

Append to `web/style.css`:

```css
.auth { max-width: 320px; margin: 80px auto; padding: 24px; border: 1px solid #ddd; border-radius: 8px; }
.auth h1 { margin-top: 0; }
.auth form { display: flex; flex-direction: column; gap: 8px; }
.auth input, .auth button { padding: 8px; }
.auth button { cursor: pointer; }

.layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
.sidebar { background: #f5f5f5; padding: 16px; border-right: 1px solid #ddd; }
.sidebar .me { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.sidebar nav { display: flex; flex-direction: column; gap: 4px; margin-bottom: 16px; }
.sidebar nav a { text-decoration: none; color: #333; padding: 6px 8px; border-radius: 4px; }
.sidebar nav a.active, .sidebar nav a:hover { background: #e0e0e0; }
.sidebar .search { display: flex; gap: 4px; }
.sidebar .search input { flex: 1; padding: 6px; }
.sidebar button { padding: 4px 8px; cursor: pointer; }
.content { padding: 24px; }
```

- [ ] **Step 8: Smoke test**

```bash
go run . &
sleep 1
# Open http://localhost:8080 in a browser, log in. Sidebar should appear.
kill %1
```

- [ ] **Step 9: Commit**

```bash
git add web/
git commit -m "feat: add frontend api/router/store and login view"
```

---

## Task 17: Tasks view (kanban with drag-and-drop)

**Files:** Modify: `web/views/tasks.js`. Modify: `web/style.css`.

- [ ] **Step 1: Replace `web/views/tasks.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/tasks', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/tasks', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const tasks = await api.listTasks();
  app.innerHTML = layout('/tasks', boardHtml(tasks));
  bindLayout();
  bindBoard();
}

function boardHtml(tasks) {
  const cols = ['todo', 'in_progress', 'done'];
  const labels = { todo: 'К выполнению', in_progress: 'В работе', done: 'Готово' };
  const card = (t) => `
    <article class="card" draggable="true" data-id="${t.id}">
      <h4>${escapeHtml(t.title)}</h4>
      ${t.priority ? `<span class="prio prio-${escapeHtml(t.priority)}">${escapeHtml(t.priority)}</span>` : ''}
      ${t.due_date ? `<small>${escapeHtml(t.due_date)}</small>` : ''}
    </article>`;
  const cards = (status) => {
    const filtered = tasks.filter((t) => t.status === status);
    return filtered.length ? filtered.map(card).join('') : '<p class="empty">—</p>';
  };
  return `
    <h2>Задачи</h2>
    <div class="board">
      ${cols.map((c) => `
        <section class="col">
          <header><h3>${labels[c]}</h3><button class="add" data-status="${c}">+</button></header>
          <div class="cards" data-status="${c}">${cards(c)}</div>
        </section>`).join('')}
    </div>`;
}

function bindBoard() {
  document.querySelectorAll('.add').forEach((btn) => {
    btn.onclick = () => openCreate(btn.dataset.status);
  });
  document.querySelectorAll('.card').forEach((card) => {
    card.onclick = (e) => {
      if (e.target.closest('button')) return;
      go('/tasks/' + card.dataset.id);
    };
    card.ondragstart = (e) => e.dataTransfer.setData('text/plain', card.dataset.id);
  });
  document.querySelectorAll('.cards').forEach((zone) => {
    zone.ondragover = (e) => { e.preventDefault(); zone.classList.add('over'); };
    zone.ondragleave = () => zone.classList.remove('over');
    zone.ondrop = async (e) => {
      e.preventDefault();
      zone.classList.remove('over');
      const id = e.dataTransfer.getData('text/plain');
      const status = zone.dataset.status;
      try { await api.updateTask(id, { status }); location.reload(); }
      catch (err) { alert(err.message); }
    };
  });
}

function openCreate(defaultStatus) {
  const title = prompt('Название задачи?');
  if (!title) return;
  api.createTask({ title, status: defaultStatus })
    .then(() => location.reload())
    .catch((err) => alert(err.message));
}
```

- [ ] **Step 2: Append CSS for board**

Append to `web/style.css`:

```css
.board { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }
.col { background: #fafafa; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 200px; }
.col header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.col .add { background: none; border: none; font-size: 18px; cursor: pointer; }
.cards { display: flex; flex-direction: column; gap: 6px; min-height: 100px; }
.cards.over { background: #eef; }
.card { background: white; border: 1px solid #ddd; border-radius: 4px; padding: 8px; cursor: grab; }
.card:active { cursor: grabbing; }
.prio { display: inline-block; font-size: 11px; padding: 2px 6px; border-radius: 10px; margin-left: 4px; }
.prio-low    { background: #d4f4dd; }
.prio-medium { background: #fff3cd; }
.prio-high   { background: #f8d7da; }
.empty { color: #999; font-style: italic; }
```

- [ ] **Step 3: Smoke test in browser**

Open `http://localhost:8080`, log in, click `+` to add tasks, drag between columns.

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "feat: add tasks kanban view with drag-and-drop"
```

---

## Task 18: Task detail view

**Files:** Modify: `web/views/task.js`. Modify: `web/style.css`.

- [ ] **Step 1: Replace `web/views/task.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/tasks/{id}', (p) => render(p.id));

async function render(id) {
  const app = document.getElementById('app');
  app.innerHTML = layout('/tasks', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const t = await api.getTask(id);
  app.innerHTML = layout('/tasks', detailHtml(t));
  bindLayout();
  bindDetail(t);
}

function detailHtml(t) {
  return `
    <a href="#/tasks">← К доске</a>
    <h2>${escapeHtml(t.title)}</h2>
    <p>${t.description ? escapeHtml(t.description).replaceAll('\n', '<br>') : '<em>нет описания</em>'}</p>
    <div class="row">
      <label>Статус:
        <select id="status">
          <option value="todo"        ${t.status === 'todo'        ? 'selected' : ''}>К выполнению</option>
          <option value="in_progress" ${t.status === 'in_progress' ? 'selected' : ''}>В работе</option>
          <option value="done"        ${t.status === 'done'        ? 'selected' : ''}>Готово</option>
        </select>
      </label>
      <label>Приоритет:
        <select id="priority">
          <option value="low"    ${t.priority === 'low'    ? 'selected' : ''}>Низкий</option>
          <option value="medium" ${t.priority === 'medium' ? 'selected' : ''}>Средний</option>
          <option value="high"   ${t.priority === 'high'   ? 'selected' : ''}>Высокий</option>
        </select>
      </label>
      <label>Срок: <input id="due_date" type="date" value="${t.due_date || ''}"></label>
    </div>

    <h3>Подзадачи</h3>
    <ul class="subtasks">
      ${(t.subtasks || []).map((s) => `
        <li>
          <input type="checkbox" data-sid="${s.id}" ${s.Done ? 'checked' : ''}>
          ${escapeHtml(s.title)}
          <button class="del-sub" data-sid="${s.id}">×</button>
        </li>`).join('') || '<li class="empty">—</li>'}
    </ul>
    <form id="addSubtask"><input name="title" placeholder="Новая подзадача"><button>+</button></form>

    <h3>Теги</h3>
    <ul class="tags">${(t.tags || []).map((tag) => `<li>${escapeHtml(tag)} <button class="del-tag" data-tag="${escapeHtml(tag)}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addTag"><input name="tag" placeholder="новый-тег"><button>+</button></form>

    <h3>Прикреплённые заметки</h3>
    <ul class="links">${(t.linked_notes || []).map((nid) => `<li><a href="#/notes/${nid}">Заметка #${nid}</a></li>`).join('') || '<li class="empty">—</li>'}</ul>

    <h3>Доступ</h3>
    <button id="shareBtn">Поделиться…</button>

    <p><button id="deleteBtn" class="danger">Удалить задачу</button></p>
  `;
}

async function bindDetail(t) {
  document.getElementById('status').onchange  = (e) => api.updateTask(t.id, { status: e.target.value }).then(refresh);
  document.getElementById('priority').onchange = (e) => api.updateTask(t.id, { priority: e.target.value }).then(refresh);
  document.getElementById('due_date').onchange = (e) => api.updateTask(t.id, { due_date: e.target.value || null }).then(refresh);

  document.getElementById('addSubtask').onsubmit = async (e) => {
    e.preventDefault();
    const title = e.target.title.value.trim();
    if (!title) return;
    await api.addSubtask(t.id, title);
    refresh();
  };
  document.querySelectorAll('.del-sub').forEach((btn) => {
    btn.onclick = async () => {
      const sid = btn.dataset.sid;
      await fetch('/api/tasks/' + t.id + '/subtasks/' + sid, { method: 'DELETE', headers: { 'Authorization': 'Bearer ' + JSON.parse(localStorage.getItem('user') || '{}').token || '' } });
      refresh();
    };
  });
  document.querySelectorAll('input[type=checkbox][data-sid]').forEach((cb) => {
    cb.onchange = async () => {
      const sid = cb.dataset.sid;
      await fetch('/api/tasks/' + t.id + '/subtasks/' + sid, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + (localStorage.getItem('token') || '') },
        body: JSON.stringify({ done: cb.checked }),
      });
    };
  });

  document.getElementById('addTag').onsubmit = async (e) => {
    e.preventDefault();
    const tag = e.target.tag.value.trim();
    if (!tag) return;
    await api.addTaskTag(t.id, tag);
    refresh();
  };
  document.querySelectorAll('.del-tag').forEach((btn) => {
    btn.onclick = async () => { await api.removeTaskTag(t.id, btn.dataset.tag); refresh(); };
  });

  document.getElementById('shareBtn').onclick = () => openShare('task', t.id);
  document.getElementById('deleteBtn').onclick = async () => {
    if (!confirm('Удалить задачу?')) return;
    await api.deleteTask(t.id);
    go('/tasks');
  };
}

function refresh() { location.reload(); }

async function openShare(type, id) {
  const username = prompt('Имя пользователя для шаринга?');
  if (!username) return;
  try { await api.share(type, id, username); alert('Доступ открыт'); }
  catch (e) { alert(e.message); }
}
```

- [ ] **Step 2: Append CSS for task detail**

Append to `web/style.css`:

```css
.row { display: flex; gap: 16px; flex-wrap: wrap; align-items: center; margin: 8px 0 16px; }
.subtasks, .tags, .links { list-style: none; padding: 0; margin: 8px 0; }
.subtasks li, .tags li, .links li { padding: 4px 0; }
button.danger { background: #f8d7da; border: 1px solid #f5c2c7; padding: 6px 12px; cursor: pointer; border-radius: 4px; }
input[type=text], input[type=date], select { padding: 4px 8px; }
form.inline { display: inline-block; }
```

- [ ] **Step 3: Smoke test and commit**

```bash
go run . &
sleep 1
# Open http://localhost:8080, log in, create a task, click it, edit fields, add subtasks/tags.
kill %1
git add web/
git commit -m "feat: add task detail view with subtasks, tags, share"
```

---

## Task 19: Notes list view

**Files:** Modify: `web/views/notes.js`. Modify: `web/style.css`.

- [ ] **Step 1: Replace `web/views/notes.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/notes', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/notes', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const notes = await api.listNotes();
  app.innerHTML = layout('/notes', listHtml(notes));
  bindLayout();
  bindList();
}

function listHtml(notes) {
  return `
    <h2>Заметки</h2>
    <p><button id="newNote" class="primary">+ Новая заметка</button></p>
    ${notes.length === 0 ? '<p class="empty">Пока нет заметок</p>' : `
    <ul class="note-list">
      ${notes.map((n) => `
        <li class="note-item ${n.pinned ? 'pinned' : ''}">
          <a href="#/notes/${n.id}">
            <strong>${n.pinned ? '📌 ' : ''}${escapeHtml(n.title)}</strong>
            <small>${new Date(n.updated_at).toLocaleString('ru')}</small>
            ${n.tags && n.tags.length ? `<div class="tags-mini">${n.tags.map(escapeHtml).join(', ')}</div>` : ''}
          </a>
        </li>`).join('')}
    </ul>`}`;
}

function bindList() {
  document.getElementById('newNote').onclick = async () => {
    const title = prompt('Название заметки?');
    if (!title) return;
    const n = await api.createNote({ title, body_md: '' });
    go('/notes/' + n.id);
  };
}
```

- [ ] **Step 2: Append CSS for note list**

Append to `web/style.css`:

```css
button.primary { background: #2563eb; color: white; border: none; padding: 6px 12px; border-radius: 4px; cursor: pointer; }
.note-list { list-style: none; padding: 0; }
.note-item { border: 1px solid #ddd; border-radius: 4px; margin-bottom: 6px; }
.note-item.pinned { border-color: #f0c000; background: #fffbe6; }
.note-item a { display: block; padding: 8px 12px; text-decoration: none; color: #222; }
.note-item small { color: #777; margin-left: 8px; }
.tags-mini { font-size: 12px; color: #888; margin-top: 4px; }
```

- [ ] **Step 3: Smoke test and commit**

```bash
go run . &
sleep 1
# Open the app, create a note, return to list.
kill %1
git add web/
git commit -m "feat: add notes list view"
```

---

## Task 20: Note editor (markdown)

**Files:** Modify: `web/views/note.js`. Modify: `web/style.css`.

- [ ] **Step 1: Replace `web/views/note.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/notes/{id}', (p) => render(p.id));

let editor = null;

async function render(id) {
  const app = document.getElementById('app');
  app.innerHTML = layout('/notes', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const n = await api.getNote(id);
  app.innerHTML = layout('/notes', editorHtml(n));
  bindLayout();
  initEditor(n);
}

function editorHtml(n) {
  return `
    <a href="#/notes">← К заметкам</a>
    <div class="note-toolbar">
      <input id="title" value="${escapeHtml(n.title)}" placeholder="Заголовок">
      <label><input type="checkbox" id="pinned" ${n.pinned ? 'checked' : ''}> Закрепить</label>
      <button id="saveBtn">Сохранить</button>
      <button id="versionsBtn">История</button>
      <button id="shareBtn">Поделиться</button>
      <button id="deleteBtn" class="danger">Удалить</button>
    </div>
    <div class="editor-split">
      <textarea id="body" placeholder="Markdown...">${escapeHtml(n.body_md)}</textarea>
      <div id="preview" class="preview"></div>
    </div>
    <h3>Теги</h3>
    <ul class="tags">${(n.tags || []).map((tag) => `<li>${escapeHtml(tag)} <button class="del-tag" data-tag="${escapeHtml(tag)}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addTag"><input name="tag" placeholder="новый-тег"><button>+</button></form>

    <h3>Прикреплённые задачи</h3>
    <ul class="links">${(n.linked_tasks || []).map((tid) => `<li><a href="#/tasks/${tid}">Задача #${tid}</a></li>`).join('') || '<li class="empty">—</li>'}</ul>
  `;
}

function initEditor(n) {
  editor = { id: n.id };
  const marked = window.marked;
  const bodyEl = document.getElementById('body');
  const previewEl = document.getElementById('preview');
  const render = () => { previewEl.innerHTML = marked.parse(bodyEl.value); };
  bodyEl.addEventListener('input', render);
  render();

  document.getElementById('title').addEventListener('change', save);
  bodyEl.addEventListener('change', save);
  document.getElementById('pinned').addEventListener('change', save);
  document.getElementById('saveBtn').onclick = () => save().then(() => alert('Сохранено'));

  document.getElementById('addTag').onsubmit = async (e) => {
    e.preventDefault();
    const tag = e.target.tag.value.trim();
    if (!tag) return;
    await api.addNoteTag(editor.id, tag);
    location.reload();
  };
  document.querySelectorAll('.del-tag').forEach((btn) => {
    btn.onclick = async () => { await fetch('/api/notes/' + editor.id + '/tags/' + encodeURIComponent(btn.dataset.tag), { method: 'DELETE', headers: { 'Authorization': 'Bearer ' + localStorage.getItem('token') } }); location.reload(); };
  });

  document.getElementById('versionsBtn').onclick = () => openVersions(editor.id);
  document.getElementById('shareBtn').onclick = () => {
    const username = prompt('Имя пользователя для шаринга?');
    if (!username) return;
    api.share('note', editor.id, username).then(() => alert('Доступ открыт')).catch((e) => alert(e.message));
  };
  document.getElementById('deleteBtn').onclick = async () => {
    if (!confirm('Удалить заметку?')) return;
    await api.deleteNote(editor.id);
    go('/notes');
  };
}

async function save() {
  await api.updateNote(editor.id, {
    title:  document.getElementById('title').value,
    body_md: document.getElementById('body').value,
    pinned: document.getElementById('pinned').checked,
  });
}

async function openVersions(id) {
  const vs = await api.versions(id);
  if (!vs.length) { alert('История пуста'); return; }
  const lines = vs.map((v, i) => `${i + 1}. [${new Date(v.SavedAt).toLocaleString('ru')}] ${v.EditorName || '—'} — ${v.title}`);
  const choice = prompt('Введите номер версии для восстановления:\n\n' + lines.join('\n'));
  if (!choice) return;
  const idx = parseInt(choice, 10) - 1;
  if (isNaN(idx) || idx < 0 || idx >= vs.length) { alert('Неверный номер'); return; }
  await api.restore(id, vs[idx].id);
  location.reload();
}
```

- [ ] **Step 2: Append CSS for editor**

Append to `web/style.css`:

```css
.note-toolbar { display: flex; gap: 8px; align-items: center; margin: 8px 0 16px; flex-wrap: wrap; }
.note-toolbar input[type=text] { flex: 1; padding: 6px; font-size: 16px; }
.editor-split { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; min-height: 400px; }
.editor-split textarea { width: 100%; min-height: 400px; font-family: ui-monospace, monospace; padding: 8px; }
.preview { border: 1px solid #ddd; padding: 12px; background: #fafafa; overflow: auto; max-height: 70vh; }
.preview h1, .preview h2, .preview h3 { margin-top: 8px; }
.preview pre { background: #eee; padding: 8px; border-radius: 4px; overflow: auto; }
.preview code { background: #eee; padding: 1px 4px; border-radius: 2px; }
.preview blockquote { border-left: 3px solid #ccc; margin: 0; padding-left: 12px; color: #555; }
```

- [ ] **Step 3: Smoke test and commit**

```bash
go run . &
sleep 1
# Open app, create note, type Markdown, save, verify preview updates.
kill %1
git add web/
git commit -m "feat: add note editor with markdown preview"
```

---

## Task 21: Shared view (resources shared with me)

**Files:** Modify: `web/views/shared.js`.

- [ ] **Step 1: Replace `web/views/shared.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/shared', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/shared', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const items = await api.shared();
  app.innerHTML = layout('/shared', listHtml(items));
  bindLayout();
}

function listHtml(items) {
  if (items.length === 0) return '<h2>Общие со мной</h2><p class="empty">Никто пока не поделился.</p>';
  return `
    <h2>Общие со мной</h2>
    <ul class="note-list">
      ${items.map((it) => {
        const d = it.data || {};
        const title = escapeHtml(d.title || '(без названия)');
        const link  = it.type === 'task' ? `#/tasks/${d.id}` : `#/notes/${d.id}`;
        return `<li class="note-item"><a href="${link}">
          <strong>${it.type === 'task' ? '📋 ' : '📝 '}${title}</strong>
        </a></li>`;
      }).join('')}
    </ul>`;
}
```

- [ ] **Step 2: Smoke test and commit**

```bash
go run . &
sleep 1
# Log in as two users in different browsers, share, verify the other sees it.
kill %1
git add web/
git commit -m "feat: add shared-with-me view"
```

---

## Task 22: Search view

**Files:** Modify: `web/views/search.js`.

- [ ] **Step 1: Replace `web/views/search.js`**

```js
import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/search', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/search', '<div class="loading">Поиск…</div>');
  bindLayout();
  const q = new URLSearchParams(location.hash.split('?')[1] || '').get('q') || '';
  if (!q) { app.innerHTML = layout('/search', '<h2>Поиск</h2><p>Введите запрос в боковой панели.</p>'); bindLayout(); return; }

  const [tasks, notes] = await Promise.all([api.listTasks({ q }), api.listNotes({ q })]);
  const taskItems = tasks.map((t) => `<li class="note-item"><a href="#/tasks/${t.id}">📋 ${escapeHtml(t.title)} <small>${escapeHtml(t.status)}</small></a></li>`).join('') || '<li class="empty">—</li>';
  const noteItems = notes.map((n) => `<li class="note-item"><a href="#/notes/${n.id}">📝 ${escapeHtml(n.title)}</a></li>`).join('') || '<li class="empty">—</li>';

  app.innerHTML = layout('/search', `
    <h2>Поиск: «${escapeHtml(q)}»</h2>
    <h3>Задачи</h3><ul class="note-list">${taskItems}</ul>
    <h3>Заметки</h3><ul class="note-list">${noteItems}</ul>
  `);
  bindLayout();
}
```

- [ ] **Step 2: Smoke test and commit**

```bash
go run . &
sleep 1
# Use sidebar search.
kill %1
git add web/
git commit -m "feat: add search view"
```

---

## Task 23: README and end-to-end smoke test

**Files:** Create: `README.md`

- [ ] **Step 1: Write `README.md`**

```markdown
# Todoshka

Web app combining a kanban task board with markdown notes, multi-user with co-authorship sharing.

## Run

```bash
cd ~/IdeaProjects/todoshka
go mod tidy
go run .
# open http://localhost:8080
```

Set `TODOSHKA_DB` to override the SQLite path, `TODOSHKA_JWT_SECRET` to override the dev JWT secret.

## Smoke test (manual)

1. Register user A (e.g. `alice`).
2. Register user B (e.g. `bob`) in a different browser/incognito.
3. As A: create a task, add subtasks, add a tag.
4. As A: create a note, type Markdown, save.
5. As A: open the task, click "Поделиться", share with `bob`.
6. As B: open `/shared` — should see the task. Open it, change status.
7. As A: refresh the task — Bob's status change should be there.
8. As A: edit the note, click "История", restore an earlier version.
9. Search from sidebar — both task and note should appear.

## Tests

```bash
go test ./...
```

## Architecture

See `docs/superpowers/specs/2026-06-12-todoshka-design.md` and `docs/superpowers/plans/2026-06-12-todoshka-implementation.md`.
```

- [ ] **Step 2: Run the full test suite and the manual smoke checklist above**

```bash
go test ./...
# Expected: all PASS
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add README with run instructions and smoke checklist"
```

---
