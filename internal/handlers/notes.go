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
		note, err := d.GetNote(id, u.ID)
		if MapDBError(w, err, "note not found", "refetch") { return }
		writeJSON(w, http.StatusCreated, note)
	}
}

func noteGet(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		note, err := d.GetNote(id, u.ID)
		if MapDBError(w, err, "note not found", "refetch") { return }
		writeJSON(w, http.StatusOK, note)
	}
}

func noteUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { BadRequest(w, "INVALID_JSON", "bad json"); return }
		if MapDBError(w, d.UpdateNote(id, u.ID, body, u.ID), "note not found", "update note") { return }
		note, err := d.GetNote(id, u.ID)
		if MapDBError(w, err, "note not found", "refetch") { return }
		writeJSON(w, http.StatusOK, note)
	}
}

func noteDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		if MapDBError(w, d.DeleteNote(id, u.ID), "note not found", "delete note") { return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteVersions(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		vs, err := d.ListNoteVersions(id, u.ID)
		if MapDBError(w, err, "note not found", "list versions") { return }
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
		if MapDBError(w, d.RestoreNoteVersion(id, vid, u.ID), "version not found", "restore") { return }
		note, err := d.GetNote(id, u.ID)
		if MapDBError(w, err, "note not found", "refetch") { return }
		writeJSON(w, http.StatusOK, note)
	}
}

func noteAddTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		var body struct{ Tag string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" { BadRequest(w, "INVALID_BODY", "tag required"); return }
		if MapDBError(w, d.AddNoteTag(id, u.ID, body.Tag), "note not found", "tag") { return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteRemoveTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r); if !ok { return }
		if MapDBError(w, d.RemoveNoteTag(id, u.ID, r.PathValue("tag")), "note not found", "tag") { return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteLinkTask(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		nid, ok := parseID(w, r); if !ok { return }
		tid, err := strconv.ParseInt(r.PathValue("tid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad task id"); return }
		if MapDBError(w, d.LinkNoteToTask(nid, tid, u.ID), "note or task not found", "link") { return }
		w.WriteHeader(http.StatusNoContent)
	}
}

func noteUnlinkTask(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		nid, ok := parseID(w, r); if !ok { return }
		tid, err := strconv.ParseInt(r.PathValue("tid"), 10, 64)
		if err != nil { BadRequest(w, "INVALID_ID", "bad task id"); return }
		if MapDBError(w, d.UnlinkNoteFromTask(nid, tid, u.ID), "note or task not found", "unlink") { return }
		w.WriteHeader(http.StatusNoContent)
	}
}
