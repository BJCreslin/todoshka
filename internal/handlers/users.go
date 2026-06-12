package handlers

import (
	"net/http"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
)

func mountUsers(mux *http.ServeMux, d *db.DB, secret string) {
	mux.Handle("GET /api/users", auth.RequireUser(secret, userCtxKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		users, err := d.SearchUsers(q)
		if err != nil {
			Internal(w, "search users")
			return
		}
		out := make([]publicUser, 0, len(users))
		for _, u := range users {
			out = append(out, publicUser{ID: u.ID, Username: u.Username})
		}
		writeJSON(w, http.StatusOK, out)
	})))
}
