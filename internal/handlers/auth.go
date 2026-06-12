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

func mountUsers(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountTasks(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountNotes(mux *http.ServeMux, d *db.DB, secret string)  {}
func mountShare(mux *http.ServeMux, d *db.DB, secret string)  {}
