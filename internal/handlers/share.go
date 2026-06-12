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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			BadRequest(w, "INVALID_JSON", "bad json")
			return
		}
		if body.ResourceType != "task" && body.ResourceType != "note" {
			BadRequest(w, "INVALID_TYPE", "resource_type must be task or note")
			return
		}
		owner, _ := d.IsOwner(body.ResourceType, body.ResourceID, u.ID)
		if !owner {
			Forbidden(w, "only the owner can share")
			return
		}
		target, err := d.GetUserByUsername(body.Username)
		if err != nil {
			NotFound(w, "user not found")
			return
		}
		if err := d.Share(body.ResourceType, body.ResourceID, target.ID); err != nil {
			Internal(w, "share")
			return
		}
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			BadRequest(w, "INVALID_JSON", "bad json")
			return
		}
		owner, _ := d.IsOwner(body.ResourceType, body.ResourceID, u.ID)
		if !owner {
			Forbidden(w, "only the owner can unshare")
			return
		}
		if err := d.Unshare(body.ResourceType, body.ResourceID, body.UserID); err != nil {
			Internal(w, "unshare")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func sharedList(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		items, err := d.ListSharedWithUser(u.ID)
		if err != nil {
			Internal(w, "list shared")
			return
		}
		type out struct {
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		}
		var result []out
		for _, it := range items {
			switch it.Type {
			case "task":
				if t, err := d.GetTask(it.ID, u.ID); err == nil {
					result = append(result, out{Type: "task", Data: t})
				}
			case "note":
				if n, err := d.GetNote(it.ID, u.ID); err == nil {
					result = append(result, out{Type: "note", Data: n})
				}
			}
		}
		if result == nil {
			result = []out{}
		}
		writeJSON(w, http.StatusOK, result)
	}
}
